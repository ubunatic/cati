package cmd

import (
	"context"
	"fmt"
	"image"
	"io"
	"io/fs"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codeberg.org/ubunatic/cati/internal/audio"
	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/input"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	spec "codeberg.org/ubunatic/cati/spec"
	"golang.org/x/term"
)

// ── renderCfg ─────────────────────────────────────────────────────────────────

// renderCfg carries the active render mode (halfblock or quad variant).
// The zero value uses halfblock, matching the historic default.
//
// id is set by renderModes entries and used for equality/cycling; it must be
// unique per renderModes element. The zero id (0) belongs to "halfblock".
type renderCfg struct {
	id       int
	useQuad  bool
	quadOpts quadblock.Options
	preScale func(image.Image) image.Image // optional pre-scaler applied before ScaleToFit
}

func (rc renderCfg) scaleToFit(img image.Image, cols, rows int) image.Image {
	if rc.preScale != nil {
		img = rc.preScale(img)
	}
	if rc.useQuad {
		return quadblock.ScaleToFit(img, cols, rows)
	}
	return halfblock.ScaleToFit(img, cols, rows)
}

// preScaleName returns a short suffix for the pre-scaler name, or "".
func (rc renderCfg) preScaleName() string {
	switch {
	case rc.preScale == nil:
		return ""
	default:
		return "+pre"
	}
}

func (rc renderCfg) render(w io.Writer, img image.Image) error {
	if rc.useQuad {
		return quadblock.RenderOpts(w, img, rc.quadOpts)
	}
	return halfblock.Render(w, img)
}

// renderModes is the cycle order for the R key. Each entry's cfg.id must equal
// its slice index so that cycleRenderCfg and rcModeName can find entries by id.
var renderModes = []struct {
	name string
	cfg  renderCfg
}{
	{"halfblock", renderCfg{id: 4}},
	// {"halfblock+sharp", renderCfg{id: 10, preScale: pixelart.Sharpen05}},
	// {"halfblock+epx2x", renderCfg{id: 8, preScale: pixelart.Scale2x}},
	{"quad/pca2", renderCfg{id: 6, useQuad: true, quadOpts: quadblock.Options{PCA2: true}}},
	// {"quad/default", renderCfg{id: 7, useQuad: true}},
	// {"quad/lum+ambig", renderCfg{id: 1, useQuad: true, quadOpts: quadblock.Options{LumSplit: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/pca2+ambig", renderCfg{id: 2, useQuad: true, quadOpts: quadblock.Options{PCA2: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/splithalf+ambig", renderCfg{id: 3, useQuad: true, quadOpts: quadblock.Options{SplitHalf: true, Blend: quadblock.BlendAmbiguous}}},
	{"quad/lum-split", renderCfg{id: 5, useQuad: true, quadOpts: quadblock.Options{LumSplit: true}}},
	{"quad/splithalf", renderCfg{id: 0, useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}}},
	//{"quad/splithalf+sharp", renderCfg{id: 11, useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Sharpen05}},
	//{"quad/splithalf+epx2x", renderCfg{id: 9, useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Scale2x}},
	{"quad/edge-snap", renderCfg{id: 12, useQuad: true, quadOpts: quadblock.Options{EdgeSnap: true}}},
	// {"quad/edge-snap+ambig", renderCfg{id: 13, useQuad: true, quadOpts: quadblock.Options{EdgeSnap: true, Blend: quadblock.BlendAmbiguous}}},
}

// cycleRenderCfg returns the next renderCfg in the cycle and its display name.
// Comparison is by cfg.id, not struct equality (cfg contains a func field).
func cycleRenderCfg(rc renderCfg) (renderCfg, string) {
	for i, m := range renderModes {
		if m.cfg.id == rc.id {
			next := renderModes[(i+1)%len(renderModes)]
			return next.cfg, next.name
		}
	}
	return renderModes[0].cfg, renderModes[0].name
}

// cycleRenderCfgPrev returns the previous renderCfg in the cycle and its display name.
func cycleRenderCfgPrev(rc renderCfg) (renderCfg, string) {
	n := len(renderModes)
	for i, m := range renderModes {
		if m.cfg.id == rc.id {
			prev := renderModes[(i+n-1)%n]
			return prev.cfg, prev.name
		}
	}
	return renderModes[n-1].cfg, renderModes[n-1].name
}

// findRenderModeByID looks up a render mode by id in renderModes.
func findRenderModeByID(id int) (renderCfg, string, bool) {
	for _, m := range renderModes {
		if m.cfg.id == id {
			return m.cfg, m.name, true
		}
	}
	return renderCfg{}, "", false
}

// ── constants ─────────────────────────────────────────────────────────────────

const (
	zoomStep = 1.25  // multiply/divide per zoom action
	minZoom  = 0.125 // 1/8×
	maxZoom  = 8.0   // 8×
)

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

func interactive(path string, initWidth, initHeight int, rc renderCfg) error {
	return interactiveWithChan(path, initWidth, initHeight, rc, nil, nil, nil, nil, nil, nil)
}

func interactiveWithChan(path string, initWidth, initHeight int, rc renderCfg, sharedInputs chan string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string, inputSpec *input.Spec) error {
	if inputSpec == nil {
		inputSpec, _ = input.Load(fs.FS(spec.FS))
	}

	// Load the original image once; it is never mutated.
	orig, err := halfblock.LoadImage(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if style == nil {
		style = loadStyle()
	}
	if labels == nil {
		labels = loadLabels()
		for k, v := range loadButtons(style.BtnLeftCap, style.BtnRightCap) {
			labels[k] = v
		}
	}
	if viewBtnRows == nil {
		viewBtnRows = loadViewButtonRows()
	}
	btnActions := loadButtonActions()
	// altBtnActions := loadAltButtonActions()

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
				for _, tok := range inputSpec.Tokenize(string(buf[:n])) {
					if strings.HasPrefix(tok, "\x1b[<") {
						select {
						case inputs <- tok:
						default: // drop mouse events when buffer is full
						}
					} else {
						inputs <- tok
					}
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
	var spacePan bool
	var spacePanAnchor dragState
	termCols, termRows := resolveTermSize(initWidth, initHeight)

	var buttons []menuButton
	activeAction := ""
	var status string
	var lastKey string
	modeName := rcModeName(rc)
	lastNonHBID := rc.id // last non-halfblock mode id; used by toggle_halfblock
	if !rc.useQuad {
		lastNonHBID = -1
	}
	var curQ RenderQuality
	fileMeta := loadMediaMeta(path, false)
	redraw := func() {
		halfblock.CursorHome(os.Stdout)
		vp := renderView(orig, &state, termCols, max(1, termRows-2), rc)
		ref := buildRef(orig, state, termCols, max(1, termRows-2), rc)
		curQ = computeQuality(ref, vp, rc)
		halfblock.EraseDown(os.Stdout)
		buttons = drawBottomMenu(os.Stdout, termRows, "image_viewer", activeAction, style, labels, viewBtnRows, nil, btnActions, nil)
		hint := labels["hint_viewer"]
		if status != "" {
			hint = status
		}
		fileMeta.DispW = fmt.Sprintf("%d", termCols)
		fileMeta.DispH = fmt.Sprintf("%d", max(1, termRows-2))
		fileMeta.DispMode = "half"
		hintVars := map[string]string{
			"last_key":    lastKey,
			"ssim":        fmt.Sprintf("%.3f", curQ.SSIM),
			"blockiness":  fmt.Sprintf("%.3f", curQ.Blockiness),
			"edge_cont":   fmt.Sprintf("%.3f", curQ.EdgeCont),
			"render_mode": modeName,
		}
		for k, v := range fileMeta.Vars() {
			hintVars[k] = v
		}
		drawHintBar(os.Stdout, termRows, hint, hintVars, style)
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
				lastKey = inputSpec.EventName(inputSpec.Classify(tok))
				// ── SGR mouse event ───────────────────────────────────────────────
				if m, ok := inputSpec.ParseMouse(tok); ok {
					// Col/Row are 1-indexed terminal coordinates → convert to 0-indexed.
					c, r := m.Col-1, m.Row-1

					// ── Button bar (row termRows-1) ───────────────────────────────
					if m.Row == termRows-1 {
						newAction := ""
						for _, b := range buttons {
							if m.Col >= b.col && m.Col < b.col+b.width {
								newAction = b.action
								break
							}
						}
						if newAction != activeAction {
							activeAction = newAction
							changed = true
						}
						if m.Release && newAction != "" {
							switch newAction {
							case "inc_zoom":
								state.zoom = math.Min(state.zoom*zoomStep, maxZoom)
								changed = true
							case "dec_zoom":
								state.zoom = math.Max(state.zoom/zoomStep, minZoom)
								changed = true
							case "cycle_render":
								rc, modeName = cycleRenderCfg(rc)
								changed = true
							case "cycle_render_prev":
								rc, modeName = cycleRenderCfgPrev(rc)
								changed = true
							case "toggle_halfblock":
								if rc.useQuad {
									lastNonHBID = rc.id
									if m, n, ok := findRenderModeByID(4); ok {
										rc, modeName = m, n
									}
								} else if lastNonHBID >= 0 {
									if m, n, ok := findRenderModeByID(lastNonHBID); ok {
										rc, modeName = m, n
									}
								} else {
									// no halfblock history — just cycle forward
									rc, modeName = cycleRenderCfg(rc)
								}
								changed = true
							case "go_back", "quit":
								shouldQuit = true
							}
						}
						return
					}
					if activeAction != "" {
						activeAction = ""
						changed = true
					}

					switch {
					// ── Scroll wheel: zoom at cursor ──────────────────────────────
					case m.IsScroll() && !m.Release:
						var newZoom float64
						if m.ScrollDir() < 0 {
							newZoom = math.Min(state.zoom*zoomStep, maxZoom)
						} else {
							newZoom = math.Max(state.zoom/zoomStep, minZoom)
						}
						if newZoom != state.zoom {
							zoomAtCursor(&state, newZoom, c, r)
							changed = true
						}

					// ── Space-pan: any mouse motion pans when Space is held ───────
					case spacePan && m.IsDrag() && !m.IsScroll():
						if !spacePanAnchor.active {
							spacePanAnchor = dragState{
								active:    true,
								startCol:  c,
								startRow:  r,
								startPanX: state.panX,
								startPanY: state.panY,
							}
						}
						state.panX = spacePanAnchor.startPanX - (c - spacePanAnchor.startCol)
						state.panY = spacePanAnchor.startPanY - (r-spacePanAnchor.startRow)*2
						changed = true

					// ── Left press: start drag ────────────────────────────────────
					case !spacePan && !m.IsScroll() && !m.IsDrag() && m.Button == 0 && !m.Release:
						drag = dragState{
							active:    true,
							startCol:  c,
							startRow:  r,
							startPanX: state.panX,
							startPanY: state.panY,
						}

					// ── Left drag: update pan ─────────────────────────────────────
					case !spacePan && m.IsDrag() && m.Button == 0 && drag.active:
						// Grab-and-pull: dragging right shows more of the left side.
						state.panX = drag.startPanX - (c - drag.startCol)
						state.panY = drag.startPanY - (r-drag.startRow)*2
						changed = true

					// ── Left release: end drag ────────────────────────────────────
					case !m.IsScroll() && !m.IsDrag() && m.Button == 0 && m.Release:
						drag.active = false
					}

				} else {
					// ── Keyboard event ────────────────────────────────────────────
					switch tok {
					case "\x03": // ctrl-c always quits regardless of spec
						shouldQuit = true

					case "\x1b[A": // ↑ — structural pan (no button equiv)
						state.panY -= vStep
						changed = true

					case "\x1b[B": // ↓ — structural pan
						state.panY += vStep
						changed = true

					case "\x1b[C": // → — structural pan
						state.panX += hStep
						changed = true

					case "\x1b[D": // ← — structural pan
						state.panX -= hStep
						changed = true

					default:
						if action, ok := viewKeyMaps["image_viewer"][tok]; ok {
							switch action {
							case "go_back", "quit":
								shouldQuit = true
							case "inc_zoom":
								state.zoom = math.Min(state.zoom*zoomStep, maxZoom)
								changed = true
							case "dec_zoom":
								state.zoom = math.Max(state.zoom/zoomStep, minZoom)
								changed = true
							case "toggle_pan":
								spacePan = !spacePan
								spacePanAnchor = dragState{}
								if spacePan {
									fmt.Fprint(os.Stdout, "\x1b[?1003h\x1b[?1006h")
									newStatus = "pan  —  move mouse to pan · Space to exit"
								} else {
									fmt.Fprint(os.Stdout, "\x1b[?1002h\x1b[?1006h")
									newStatus = ""
								}
								changed = true
							case "copy_viewport":
								vp := buildViewport(orig, &state, termCols, termRows, rc)
								if copyErr := copyImageToClipboard(vp); copyErr != nil {
									newStatus = "⚠ copy failed: " + copyErr.Error()
								} else {
									newStatus = "✓ copied to clipboard"
								}
								changed = true
							case "cycle_render":
								rc, modeName = cycleRenderCfg(rc)
								changed = true
							case "cycle_render_prev":
								rc, modeName = cycleRenderCfgPrev(rc)
								changed = true
							case "toggle_halfblock":
								if rc.useQuad {
									lastNonHBID = rc.id
									if m, n, ok := findRenderModeByID(4); ok {
										rc, modeName = m, n
									}
								} else if lastNonHBID >= 0 {
									if m, n, ok := findRenderModeByID(lastNonHBID); ok {
										rc, modeName = m, n
									}
								} else {
									rc, modeName = cycleRenderCfg(rc)
								}
								changed = true
							}
						}
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
// rc is needed to compute the correct pixel budget: quad uses 2 pixels per
// terminal column horizontally (half-block uses 1).
func buildViewport(orig image.Image, state *viewState, termCols, termRows int, rc renderCfg) image.Image {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig
	}

	// Pixel budget per terminal column: 2 for quad, 1 for halfblock.
	pixCols := termCols
	if rc.useQuad {
		pixCols = termCols * 2
	}

	// For quad, treat source as 2× wider to compensate for the 1:2 pixel
	// aspect ratio (each quad pixel is narrow-and-tall, like halfblock pixels).
	fitSrcW := srcW
	if rc.useQuad {
		fitSrcW = srcW * 2
	}

	// Compute the "fit" pixel dims (what the image looks like at zoom 1.0).
	fitW, fitH := fitPixelDims(fitSrcW, srcH, pixCols, termRows*2)

	// Apply zoom to get the full scaled image size.
	scaledW := max(1, int(math.Round(float64(fitW)*state.zoom)))
	scaledH := max(1, int(math.Round(float64(fitH)*state.zoom)))

	// Scale original to zoomed dimensions (supports upscale for zoom > 1).
	scaled := halfblock.ScaleNN(orig, scaledW, scaledH)

	// Clamp pan.
	state.panX = max(0, min(state.panX, max(0, scaledW-pixCols)))
	state.panY = max(0, min(state.panY, max(0, scaledH-termRows*2)))

	// Crop to viewport.
	viewW := min(pixCols, scaledW)
	viewH := min(termRows*2, scaledH)
	return cropImage(scaled, state.panX, state.panY, viewW, viewH)
}

// renderView renders the current viewport to stdout and returns it for callers
// that need the pixel data (e.g. to compute SSIM).
func renderView(orig image.Image, state *viewState, termCols, termRows int, rc renderCfg) image.Image {
	vp := buildViewport(orig, state, termCols, termRows, rc)
	_ = rc.render(os.Stdout, vp)
	return vp
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
// openAudio starts audio playback for path if the file has an audio stream.
// Returns nil silently on any failure so video continues without audio.
func openAudio(ctx context.Context, path string) *audio.Player {
	ok, err := audio.HasAudio(path)
	if err != nil || !ok {
		return nil
	}
	p, err := audio.Open(ctx, path)
	if err != nil {
		return nil
	}
	return p
}

// stopAudio stops audio playback if a player is running.
func stopAudio(p *audio.Player) {
	if p != nil {
		p.Stop()
	}
}

func interactiveVideo(path string, initWidth, initHeight int, rc renderCfg, sharedInputs chan string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string, inputSpec *input.Spec) error {
	// The browser restores cooked-mode before calling us.  We must enter raw
	// mode ourselves so single keypresses are readable without Enter.
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		oldState = nil
	}
	defer func() {
		if oldState != nil {
			_ = term.Restore(fd, oldState)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if inputSpec == nil {
		inputSpec, _ = input.Load(fs.FS(spec.FS))
	}

	if style == nil {
		style = loadStyle()
	}
	if labels == nil {
		labels = loadLabels()
		for k, v := range loadButtons(style.BtnLeftCap, style.BtnRightCap) {
			labels[k] = v
		}
	}
	if viewBtnRows == nil {
		viewBtnRows = loadViewButtonRows()
	}
	if viewKeyMaps == nil {
		viewKeyMaps = buildViewKeyMaps(viewBtnRows, loadButtonKeyDefs(inputSpec))
	}
	btnActions := loadButtonActions()
	//altBtnActions := loadAltButtonActions()

	// keyInputs carries keyboard tokens and sits in the blocking select so keys
	// are processed immediately.  mouseInputs carries SGR mouse tokens and is
	// drained (with drop-on-full) before the select so mouse events never block
	// keyboard delivery.
	//
	// When a shared channel is provided (browser mode) we split it into the same
	// two logical streams by type-sniffing on receive.
	keyInputs := make(chan string, 4)
	mouseInputs := make(chan string, 32)

	if sharedInputs == nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				for _, tok := range inputSpec.Tokenize(string(buf[:n])) {
					if strings.HasPrefix(tok, "\x1b[<") {
						select {
						case mouseInputs <- tok:
						default: // drop mouse events when buffer is full
						}
					} else {
						select {
						case keyInputs <- tok:
						default: // drop keys only when key buffer is also full (rare)
						}
					}
				}
			}
		}()
	} else {
		// Re-route tokens from the shared channel into the two typed channels.
		// The goroutine exits when ctx is cancelled (video viewer returns).
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case tok, ok := <-sharedInputs:
					if !ok {
						return
					}
					if strings.HasPrefix(tok, "\x1b[<") {
						select {
						case mouseInputs <- tok:
						default:
						}
					} else {
						select {
						case keyInputs <- tok:
						default:
						}
					}
				}
			}
		}()
	}

	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	halfblock.EnableMouse(os.Stdout)
	defer func() {
		halfblock.DisableMouse(os.Stdout)
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	paused := false
	videoEnded := false
	var buttons []menuButton
	activeAction := ""
	var status string
	var lastKey string
	var statusClearAt time.Time
	termCols, termRows := resolveTermSize(initWidth, initHeight)
	modeName := rcModeName(rc)
	lastNonHBID := rc.id
	if !rc.useQuad {
		lastNonHBID = -1
	}
	var curQ RenderQuality
	fileMeta := loadMediaMeta(path, true)

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

	frames, cleanup, err := halfblock.OpenVideoStream(ctx, path, displayFPS)
	if err != nil {
		return fmt.Errorf("open video: %w", err)
	}
	defer cleanup()

	var audioPlayer *audio.Player

	// restartStream reopens the video and audio streams from the beginning.
	restartStream := func() {
		cleanup()
		frames, cleanup, err = halfblock.OpenVideoStream(ctx, path, displayFPS)
		if err != nil {
			frames = nil
		}
		stopAudio(audioPlayer)
		audioPlayer = openAudio(ctx, path)
	}

	var lastFrame image.Image
	var lastRawFrame image.Image

	// setPaused updates the paused flag and suspends/resumes audio accordingly.
	setPaused := func(p bool) {
		paused = p
		if p {
			audioPlayer.Pause()
		} else {
			audioPlayer.Resume()
		}
	}

	processToken := func(tok string) (quit bool) {
		lastKey = inputSpec.EventName(inputSpec.Classify(tok))
		if m, ok := inputSpec.ParseMouse(tok); ok {
			if m.Row == termRows-1 {
				newAction := ""
				for _, b := range buttons {
					if m.Col >= b.col && m.Col < b.col+b.width {
						newAction = b.action
						break
					}
				}
				activeAction = newAction
				if m.Release && newAction != "" {
					switch newAction {
					case "toggle_play_pause":
						if videoEnded {
							restartStream()
							videoEnded = false
							setPaused(false)
						} else {
							setPaused(!paused)
						}
					case "copy_viewport":
						if lastFrame != nil {
							if copyErr := copyImageToClipboard(lastFrame); copyErr != nil {
								status = "⚠ copy failed: " + copyErr.Error()
							} else {
								status = "✓ copied to clipboard"
							}
							statusClearAt = time.Now().Add(3 * time.Second)
						}
					case "cycle_render":
						rc, modeName = cycleRenderCfg(rc)
						if lastRawFrame != nil {
							lastFrame = rc.scaleToFit(lastRawFrame, termCols, max(1, termRows-2))
							b := lastFrame.Bounds()
							curQ = computeQuality(pyramidDownscale(lastRawFrame, b.Dx(), b.Dy()), lastFrame, rc)
						}
					case "cycle_render_prev":
						rc, modeName = cycleRenderCfgPrev(rc)
						if lastRawFrame != nil {
							lastFrame = rc.scaleToFit(lastRawFrame, termCols, max(1, termRows-2))
							b := lastFrame.Bounds()
							curQ = computeQuality(pyramidDownscale(lastRawFrame, b.Dx(), b.Dy()), lastFrame, rc)
						}
					case "toggle_halfblock":
						if rc.useQuad {
							lastNonHBID = rc.id
							if m, n, ok := findRenderModeByID(4); ok {
								rc, modeName = m, n
							}
						} else if lastNonHBID >= 0 {
							if m, n, ok := findRenderModeByID(lastNonHBID); ok {
								rc, modeName = m, n
							}
						} else {
							rc, modeName = cycleRenderCfg(rc)
						}
						if lastRawFrame != nil {
							lastFrame = rc.scaleToFit(lastRawFrame, termCols, max(1, termRows-2))
							b := lastFrame.Bounds()
							curQ = computeQuality(pyramidDownscale(lastRawFrame, b.Dx(), b.Dy()), lastFrame, rc)
						}
					case "go_back", "quit":
						return true
					}
				}
			}
			return false
		}
		switch tok {
		case "\x03": // ctrl-c always quits regardless of spec
			return true
		default:
			if action, ok := viewKeyMaps["video_player"][tok]; ok {
				switch action {
				case "go_back", "quit":
					return true
				case "toggle_play_pause":
					if videoEnded {
						restartStream()
						videoEnded = false
						setPaused(false)
					} else {
						setPaused(!paused)
					}
				case "copy_viewport":
					if lastFrame != nil {
						if copyErr := copyImageToClipboard(lastFrame); copyErr != nil {
							status = "⚠ copy failed: " + copyErr.Error()
						} else {
							status = "✓ copied to clipboard"
						}
						statusClearAt = time.Now().Add(3 * time.Second)
					}
					
			case "cycle_render":
					rc, modeName = cycleRenderCfg(rc)
					if lastRawFrame != nil {
						lastFrame = rc.scaleToFit(lastRawFrame, termCols, max(1, termRows-2))
						b := lastFrame.Bounds()
						curQ = computeQuality(pyramidDownscale(lastRawFrame, b.Dx(), b.Dy()), lastFrame, rc)
					}
					
				case "cycle_render_prev":
					rc, modeName = cycleRenderCfgPrev(rc)
					if lastRawFrame != nil {
						lastFrame = rc.scaleToFit(lastRawFrame, termCols, max(1, termRows-2))
						b := lastFrame.Bounds()
						curQ = computeQuality(pyramidDownscale(lastRawFrame, b.Dx(), b.Dy()), lastFrame, rc)
					}

				case "toggle_halfblock":
					if rc.useQuad {
						lastNonHBID = rc.id
						if m, n, ok := findRenderModeByID(4); ok {
							rc, modeName = m, n
						}
					} else if lastNonHBID >= 0 {
						if m, n, ok := findRenderModeByID(lastNonHBID); ok {
							rc, modeName = m, n
						}
					} else {
						rc, modeName = cycleRenderCfg(rc)
					}
					if lastRawFrame != nil {
						lastFrame = rc.scaleToFit(lastRawFrame, termCols, max(1, termRows-2))
						b := lastFrame.Bounds()
						curQ = computeQuality(pyramidDownscale(lastRawFrame, b.Dx(), b.Dy()), lastFrame, rc)
					}
						
					}
				}
			}
			return false
		}
			// Audio: start playback alongside video.
	audioPlayer = openAudio(ctx, path)
	defer stopAudio(audioPlayer)

	for {
		// Drain pending mouse events (drop-on-full already handled at source).
	drainMouse:
		for {
			select {
			case tok := <-mouseInputs:
				if processToken(tok) {
					return nil
				}
				if paused && lastFrame != nil {
					halfblock.CursorHome(os.Stdout)
					if err := rc.render(os.Stdout, lastFrame); err != nil {
						return err
					}
					halfblock.EraseDown(os.Stdout)
				}
			default:
				break drainMouse
			}
		}

		select {
		case <-sigs:
			return nil

		case tok := <-keyInputs:
			// Keys are in the blocking select so they are processed immediately,
			// not deferred to the next ticker tick.
			if processToken(tok) {
				return nil
			}
			if paused && lastFrame != nil {
				halfblock.CursorHome(os.Stdout)
				if err := rc.render(os.Stdout, lastFrame); err != nil {
					return err
				}
				halfblock.EraseDown(os.Stdout)
			}

		case <-ticker.C:
			// Pull exactly one frame per tick (non-blocking) so playback advances
			// at displayFPS without fast-forwarding between ticks.
			if !paused {
				select {
				case img, ok := <-frames:
					if !ok {
						frames = nil
						paused = true
						videoEnded = true
						stopAudio(audioPlayer)
						audioPlayer = nil
					} else {
						lastRawFrame = img
						lastFrame = rc.scaleToFit(img, termCols, max(1, termRows-2))
						{
							b := lastFrame.Bounds()
							curQ = computeQuality(pyramidDownscale(lastRawFrame, b.Dx(), b.Dy()), lastFrame, rc)
						}
					}
				default:
					// no frame ready — keep showing lastFrame
				}
			}

			if lastFrame == nil {
				continue
			}
			if !paused {
				halfblock.CursorHome(os.Stdout)
				if err := rc.render(os.Stdout, lastFrame); err != nil {
					return err
				}
				halfblock.EraseDown(os.Stdout)
			}
			conditions := map[string]bool{"playing": !paused}
			buttons = drawBottomMenu(os.Stdout, termRows, "video_player", activeAction, style, labels, viewBtnRows, conditions, btnActions, nil)
			if status != "" && time.Now().After(statusClearAt) {
				status = ""
			}
			hint := labels["hint_viewer"]
			if status != "" {
				hint = status
			}
			fileMeta.DispW = fmt.Sprintf("%d", termCols)
			fileMeta.DispH = fmt.Sprintf("%d", max(1, termRows-2))
			fileMeta.DispMode = "half"
			hintVars := map[string]string{
				"last_key":    lastKey,
				"ssim":        fmt.Sprintf("%.3f", curQ.SSIM),
				"blockiness":  fmt.Sprintf("%.3f", curQ.Blockiness),
				"edge_cont":   fmt.Sprintf("%.3f", curQ.EdgeCont),
				"render_mode": modeName,
			}
			for k, v := range fileMeta.Vars() {
				hintVars[k] = v
			}
			drawHintBar(os.Stdout, termRows, hint, hintVars, style)
		}
	}
}
