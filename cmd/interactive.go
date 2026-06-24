package cmd

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"golang.org/x/term"
)

// ── constants ─────────────────────────────────────────────────────────────────

const (
	zoomStep = 1.25  // multiply/divide per zoom action
	minZoom  = 0.125 // 1/8×
	maxZoom  = 8.0   // 8×
)

// SGR button-code bit layout:
//
//	bits 0–1  button number (0=left, 1=middle, 2=right)
//	bit  2    Shift modifier
//	bit  3    Meta/Alt modifier
//	bit  4    Ctrl modifier
//	bit  5    motion flag  – set when the event is a drag (button held + move)
//	bit  6    scroll flag  – set for scroll-wheel events
const (
	sgrFlagMotion = 1 << 5 // 0x20
	sgrFlagScroll = 1 << 6 // 0x40
)

// sgrIsScroll reports whether the event is a scroll-wheel event.
func sgrIsScroll(btn int) bool { return btn&sgrFlagScroll != 0 }

// sgrIsDrag reports whether the event is a button-motion (drag) event.
func sgrIsDrag(btn int) bool { return btn&sgrFlagScroll == 0 && btn&sgrFlagMotion != 0 }

// sgrButton returns the button number (0 = left, 1 = middle, 2 = right).
func sgrButton(btn int) int { return btn & 3 }

// sgrScrollDir returns -1 for scroll-up (zoom in) and +1 for scroll-down (zoom out).
// Valid only when sgrIsScroll(btn) is true.
func sgrScrollDir(btn int) int {
	if btn&1 == 0 {
		return -1 // scroll up
	}
	return 1 // scroll down
}

// ── viewState ────────────────────────────────────────────────────────────────

// viewState holds the current zoom level and pan offset (in scaled pixels).
type viewState struct {
	zoom float64
	panX int // pixel offset into the zoomed image (x)
	panY int // pixel offset into the zoomed image (y)
}

// ── dragState ────────────────────────────────────────────────────────────────

// dragState tracks an in-progress left-button drag used to pan the image.
type dragState struct {
	active    bool
	startCol  int // 0-indexed terminal column where drag began
	startRow  int // 0-indexed terminal row    where drag began
	startPanX int // state.panX at drag start
	startPanY int // state.panY at drag start
}

// ── interactive ──────────────────────────────────────────────────────────────

// interactive opens a single image in a full-screen interactive viewer.
//
// Mouse:
//
//	scroll up/down     zoom in / out  at cursor position
//	left-button drag   pan (grab-and-pull the image)
//
// Keys:
//
//	+/=          zoom in  (centred on screen)
//	-            zoom out (centred on screen)
//	↑↓←→         pan
//	c / C        copy current viewport to clipboard (PNG)
//	q/Q/Esc/^C   quit
func tokenizeInput(s string) []string {
	var tokens []string
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			if i+3 < len(s) && s[i:i+3] == "\x1b[<" {
				idx := -1
				for j := i + 3; j < len(s); j++ {
					if s[j] == 'M' || s[j] == 'm' {
						idx = j
						break
					}
				}
				if idx != -1 {
					tokens = append(tokens, s[i:idx+1])
					i = idx + 1
					continue
				}
			}
			if i+2 < len(s) && s[i+1] == '[' {
				idx := -1
				for j := i + 2; j < len(s); j++ {
					c := s[j]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '~' {
						idx = j
						break
					}
				}
				if idx != -1 {
					tokens = append(tokens, s[i:idx+1])
					i = idx + 1
					continue
				}
			}
			tokens = append(tokens, "\x1b")
			i++
		} else {
			tokens = append(tokens, string(s[i]))
			i++
		}
	}
	return tokens
}

func interactive(path string, initWidth, initHeight int) error {
	return interactiveWithChan(path, initWidth, initHeight, nil)
}

func interactiveWithChan(path string, initWidth, initHeight int, sharedInputs chan string) error {
	// Load the original image once; it is never mutated.
	orig, err := halfblock.LoadImage(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	// ── Raw terminal mode ─────────────────────────────────────────────────────
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		oldState = nil // stdin not a tty (e.g. in tests)
	}
	defer func() {
		if oldState != nil {
			_ = term.Restore(fd, oldState)
		}
	}()

	// ── Signal handling ───────────────────────────────────────────────────────
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	// ── Input reader (goroutine or shared) ────────────────────────────────────
	inputs := sharedInputs
	if inputs == nil {
		inputs = make(chan string, 32)
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				tokens := tokenizeInput(string(buf[:n]))
				for _, tok := range tokens {
					inputs <- tok
				}
			}
		}()
	}

	// ── Enter visual mode ─────────────────────────────────────────────────────
	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	halfblock.EnableMouse(os.Stdout)
	defer func() {
		halfblock.DisableMouse(os.Stdout)
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n") // CR+LF: ensure col 0 so the shell won't show a stray '%'
	}()

	state := viewState{zoom: 1.0}
	var drag dragState
	termCols, termRows := resolveTermSize(initWidth, initHeight)

	var status string
	redraw := func() {
		halfblock.CursorHome(os.Stdout)
		renderView(orig, &state, termCols, termRows)
		halfblock.EraseDown(os.Stdout)
		if status != "" {
			// Pin status on the last terminal row with reverse-video styling.
			fmt.Fprintf(os.Stdout, "\x1b[%d;1H\x1b[K\x1b[7m %s \x1b[m", termRows, status)
		}
	}
	redraw()

	for {
		select {
		case <-sigs:
			return nil

		case k := <-inputs:
			termCols, termRows = resolveTermSize(initWidth, initHeight)
			hStep := max(1, termCols/8)
			vStep := max(1, termRows*2/8) // in pixel rows

			changed := false
			newStatus := ""
			shouldQuit := false

			processInput := func(tok string) {
				// ── SGR mouse event ───────────────────────────────────────────────
				if btn, col, row, release, ok := parseSGRMouse(tok); ok {
					// col/row are 1-indexed terminal coordinates → convert to 0-indexed.
					c, r := col-1, row-1

					switch {
					// ── Scroll wheel: zoom at cursor ──────────────────────────────
					case sgrIsScroll(btn) && !release:
						var newZoom float64
						if sgrScrollDir(btn) < 0 {
							newZoom = math.Min(state.zoom*zoomStep, maxZoom)
						} else {
							newZoom = math.Max(state.zoom/zoomStep, minZoom)
						}
						if newZoom != state.zoom {
							zoomAtCursor(&state, newZoom, c, r)
							changed = true
						}

					// ── Left press: start drag ────────────────────────────────────
					case !sgrIsScroll(btn) && !sgrIsDrag(btn) && sgrButton(btn) == 0 && !release:
						drag = dragState{
							active:    true,
							startCol:  c,
							startRow:  r,
							startPanX: state.panX,
							startPanY: state.panY,
						}

					// ── Left drag: update pan ─────────────────────────────────────
					case sgrIsDrag(btn) && sgrButton(btn) == 0 && drag.active:
						// Grab-and-pull: dragging right shows more of the left side.
						state.panX = drag.startPanX - (c - drag.startCol)
						state.panY = drag.startPanY - (r-drag.startRow)*2
						changed = true

					// ── Left release: end drag ────────────────────────────────────
					case !sgrIsScroll(btn) && !sgrIsDrag(btn) && sgrButton(btn) == 0 && release:
						drag.active = false
					}

				} else {
					// ── Keyboard event ────────────────────────────────────────────
					switch tok {
					case "q", "Q", "\x1b", "\x03":
						shouldQuit = true

					case "+", "=":
						newZoom := math.Min(state.zoom*zoomStep, maxZoom)
						if newZoom != state.zoom {
							state.zoom = newZoom
							changed = true
						}

					case "-":
						newZoom := math.Max(state.zoom/zoomStep, minZoom)
						if newZoom != state.zoom {
							state.zoom = newZoom
							changed = true
						}

					case "\x1b[A": // ↑
						state.panY -= vStep
						changed = true

					case "\x1b[B": // ↓
						state.panY += vStep
						changed = true

					case "\x1b[C": // →
						state.panX += hStep
						changed = true

					case "\x1b[D": // ←
						state.panX -= hStep
						changed = true

					case "c", "C":
						vp := buildViewport(orig, &state, termCols, termRows)
						if copyErr := copyImageToClipboard(vp); copyErr != nil {
							newStatus = "⚠ copy failed: " + copyErr.Error()
						} else {
							newStatus = "✓ copied to clipboard"
						}
						changed = true
					}
				}
			}

			// Process first event
			processInput(k)
			if shouldQuit {
				return nil
			}

			// Coalesce / drain consecutive events
			draining := true
			for draining {
				select {
				case tok := <-inputs:
					processInput(tok)
					if shouldQuit {
						return nil
					}
				default:
					draining = false
				}
			}

			if changed {
				status = newStatus
				redraw()
			}
		}
	}
}

// ── resolveTermSize ───────────────────────────────────────────────────────────

// resolveTermSize returns the effective terminal dimensions, auto-detecting any
// dimension that is ≤ 0, with a safe fallback when the terminal is not available.
func resolveTermSize(width, height int) (cols, rows int) {
	autoCols, autoRows := halfblock.TermWidth(), halfblock.TermHeight()
	cols, rows = width, height
	if cols <= 0 {
		cols = autoCols
	}
	if rows <= 0 {
		rows = autoRows
	}
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	return
}

// ── zoom helpers ─────────────────────────────────────────────────────────────

// zoomAtCursor adjusts state so the pixel under the cursor (0-indexed terminal
// col/row) remains visually fixed after the zoom changes to newZoom.
//
// Derivation: let p = panX + col be the pixel in the scaled image under the
// cursor. After scaling by factor f = newZoom/oldZoom, that pixel moves to
// p*f in the new scaled image. To keep it under the cursor: newPanX = p*f - col.
func zoomAtCursor(state *viewState, newZoom float64, col, row int) {
	f := newZoom / state.zoom
	state.panX = int(math.Round(float64(state.panX+col)*f)) - col
	state.panY = int(math.Round(float64(state.panY+row*2)*f)) - row*2
	state.zoom = newZoom
}

// ── buildViewport / renderView ────────────────────────────────────────────────

// buildViewport returns the cropped+scaled image for the current view state.
// It also clamps state.panX/panY in-place so they never exceed image bounds.
func buildViewport(orig image.Image, state *viewState, termCols, termRows int) image.Image {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig
	}

	// Compute the "fit" pixel dims (what the image looks like at zoom 1.0).
	fitW, fitH := fitPixelDims(srcW, srcH, termCols, termRows*2)

	// Apply zoom to get the full scaled image size.
	scaledW := max(1, int(math.Round(float64(fitW)*state.zoom)))
	scaledH := max(1, int(math.Round(float64(fitH)*state.zoom)))

	// Scale original to zoomed dimensions (supports upscale for zoom > 1).
	scaled := halfblock.ScaleNN(orig, scaledW, scaledH)

	// Clamp pan.
	state.panX = max(0, min(state.panX, max(0, scaledW-termCols)))
	state.panY = max(0, min(state.panY, max(0, scaledH-termRows*2)))

	// Crop to viewport.
	viewW := min(termCols, scaledW)
	viewH := min(termRows*2, scaledH)
	return cropImage(scaled, state.panX, state.panY, viewW, viewH)
}

// renderView renders the current viewport to stdout.
func renderView(orig image.Image, state *viewState, termCols, termRows int) {
	vp := buildViewport(orig, state, termCols, termRows)
	_ = halfblock.Render(os.Stdout, vp)
}

// ── SGR mouse parsing ─────────────────────────────────────────────────────────

// parseSGRMouse parses an SGR extended mouse event of the form:
//
//	\x1b[<btn;col;rowM   (press / drag)
//	\x1b[<btn;col;rowm   (release)
//
// col and row are 1-indexed terminal coordinates.
// Returns ok=false for any other input sequence.
func parseSGRMouse(s string) (btn, col, row int, release bool, ok bool) {
	if !strings.HasPrefix(s, "\x1b[<") {
		return
	}
	body := s[3:]
	switch {
	case strings.HasSuffix(body, "M"):
		body = body[:len(body)-1]
	case strings.HasSuffix(body, "m"):
		release = true
		body = body[:len(body)-1]
	default:
		return
	}
	parts := strings.SplitN(body, ";", 3)
	if len(parts) != 3 {
		return
	}
	btn, _ = strconv.Atoi(parts[0])
	col, _ = strconv.Atoi(parts[1])
	row, _ = strconv.Atoi(parts[2])
	ok = true
	return
}

// ── pixel helpers ─────────────────────────────────────────────────────────────

// fitPixelDims returns the largest w×h that fits srcW×srcH inside maxW×maxH
// while preserving the aspect ratio (never upscales).
func fitPixelDims(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW == 0 || srcH == 0 {
		return max(1, maxW), max(1, maxH)
	}
	w, h := srcW, srcH
	if w > maxW {
		h = h * maxW / w
		w = maxW
	}
	if h > maxH {
		w = w * maxH / h
		h = maxH
	}
	return max(1, w), max(1, h)
}

// cropImage returns the w×h sub-image of img starting at pixel (x, y).
// Uses SubImage when available (zero-copy for *image.RGBA); otherwise copies.
func cropImage(img image.Image, x, y, w, h int) image.Image {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	b := img.Bounds()
	r := image.Rect(b.Min.X+x, b.Min.Y+y, b.Min.X+x+w, b.Min.Y+y+h)
	if si, ok := img.(subImager); ok {
		return si.SubImage(r)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			dst.Set(dx, dy, img.At(r.Min.X+dx, r.Min.Y+dy))
		}
	}
	return dst
}

// interactiveVideo plays a video file in the terminal using the caller's shared
// input channel so that keyboard events are routed through the browser's tokenizer.
func interactiveVideo(path string, initWidth, initHeight int, sharedInputs chan string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputs := sharedInputs
	if inputs == nil {
		inputs = make(chan string, 32)
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				for _, tok := range tokenizeInput(string(buf[:n])) {
					inputs <- tok
				}
			}
		}()
	}

	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	defer func() {
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	termCols, termRows := resolveTermSize(initWidth, initHeight)

	// Probe native fps for smooth playback.
	displayFPS := 24.0
	if native, err := halfblock.ProbeVideoFPS(path); err == nil && native > 0 {
		displayFPS = native
	}
	interval := time.Duration(float64(time.Second) / displayFPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	frames, cleanup, err := halfblock.OpenVideoStream(ctx, path)
	if err != nil {
		return fmt.Errorf("open video: %w", err)
	}
	defer cleanup()

	var lastFrame image.Image

	isQuitToken := func(tok string) bool {
		switch tok {
		case "q", "Q", "\x1b", "\x03":
			return true
		}
		return false
	}

	for {
		// Priority: drain any pending quit tokens before yielding to the high-
		// frequency ticker/frames cases (which would otherwise starve input).
		select {
		case tok := <-inputs:
			if isQuitToken(tok) {
				return nil
			}
		default:
		}

		select {
		case <-sigs:
			return nil

		case tok := <-inputs:
			if isQuitToken(tok) {
				return nil
			}

		case img, ok := <-frames:
			if !ok {
				return nil // video ended
			}
			lastFrame = halfblock.ScaleToFit(img, termCols, termRows)

		case <-ticker.C:
			if lastFrame == nil {
				continue
			}
			halfblock.CursorHome(os.Stdout)
			if err := halfblock.Render(os.Stdout, lastFrame); err != nil {
				return err
			}
			halfblock.EraseDown(os.Stdout)
			// Drop stale buffered frames to stay in sync with the ticker.
			n := max(0, len(frames)-1)
			for range n {
				if img, ok := <-frames; ok {
					lastFrame = halfblock.ScaleToFit(img, termCols, termRows)
				}
			}
		}
	}
}
