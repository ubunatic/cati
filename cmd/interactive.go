package cmd

import (
	"context"
	"fmt"
	"image"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/term"
	"ubunatic.com/cati/internal/audio"
	browserui "ubunatic.com/cati/internal/browser"
	"ubunatic.com/cati/internal/input"
	"ubunatic.com/cati/internal/metrics"
	"ubunatic.com/cati/internal/viewgeom"
	spec "ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	catiterm "ubunatic.com/cati/v1/term"
)

func interactive(path string, initWidth, initHeight int, rc renderCfg, fullComp bool, initialZoom string) error {
	return interactiveWithChan(path, initWidth, initHeight, rc, nil, nil, nil, nil, nil, nil, fullComp, initialZoom)
}

func interactiveWithChan(path string, initWidth, initHeight int, rc renderCfg, sharedInputs chan string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string, inputSpec *input.Spec, fullComp bool, initialZoom string) error {
	if inputSpec == nil {
		loadedInputSpec, loadErr := input.Load(fs.FS(spec.FS))
		if loadErr != nil {
			return fmt.Errorf("load input spec: %w", loadErr)
		}
		inputSpec = loadedInputSpec
	}

	orig, err := halfblock.LoadImage(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if style == nil {
		style, err = loadStyle()
		if err != nil {
			return err
		}
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

	sigs := make(chan os.Signal, 1)
	notifySignals(sigs, inputSpec.SignalsFor(input.EventQuit))
	defer signal.Stop(sigs)

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
						default:
						}
					} else {
						inputs <- tok
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

	vc := newViewerCore("image_viewer", initWidth, initHeight, rc, fullComp, inputSpec, style, labels, viewBtnRows, viewKeyMaps, btnActions)
	b := orig.Bounds()
	vc.lastSrcW, vc.lastSrcH = b.Dx(), b.Dy()
	vc.src = orig
	if initWidth > 0 && initHeight == 0 && initialZoom == "" {
		if cells := fittedCellSize(orig, vc.termCols, 0, vc.rc); cells.Rows > 0 {
			vc.termRows = cells.Rows + 2
		}
	}
	fitInitialFrame := initWidth > 0 && initHeight == 0 && initialZoom == ""
	vc.state.zoom = initialZoomRatio(initialZoom, vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows(), vc.rc.mode)
	vc.rerender = func() {
		if fitInitialFrame {
			vp := prepareRenderedImage(orig, nil, vc.termCols, 0, vc.rc, "")
			b := vp.Bounds()
			ref := metrics.PyramidDownscale(orig, b.Dx(), b.Dy())
			vc.curQ = computeQuality(ref, vp, vc.rc)
			vc.lastVP = vp
			return
		}
		vp := buildViewport(orig, &vc.state, vc.termCols, vc.viewRows(), vc.rc)
		ref := buildRef(orig, vc.state, vc.termCols, vc.viewRows(), vc.rc, metrics.GridK, vc.fullComp)
		vc.curQ = computeQuality(ref, vp, vc.rc)
		vc.lastVP = vp
	}
	vc.rerender()

	var spacePan bool
	var spacePanAnchor dragState
	fileMeta := loadMediaMeta(path, false)
	srcW, srcH := b.Dx(), b.Dy()

	redraw := func() error {
		halfblock.CursorHome(os.Stdout)
		if err := renderValidated(os.Stdout, orig, vc.lastVP, vc.state, vc.termCols, vc.viewRows(), vc.rc); err != nil {
			return err
		}
		halfblock.EraseDown(os.Stdout)
		vc.drawMenu(os.Stdout, nil)
		hint := vc.labels["hint_viewer"]
		if vc.infoVisible {
			hint = formatZoomInfo(vc.state, orig, vc.termCols, vc.viewRows(), vc.rc)
		} else if vc.status != "" {
			hint = vc.status
		}
		fileMeta.DispW = fmt.Sprintf("%d", vc.termCols)
		fileMeta.DispH = fmt.Sprintf("%d", vc.viewRows())
		fileMeta.DispMode = rcDispMode(vc.rc)
		extra := map[string]string{}
		cw, ch := vc.rc.viewSpec().VisibleCrop(srcW, srcH, vc.state.zoom, vc.state.panX, vc.state.panY, vc.termCols, vc.viewRows())
		if cw > 0 && ch > 0 && (cw != srcW || ch != srcH) {
			extra["meta.src_res"] = fmt.Sprintf("%d×%d", cw, ch)
		}
		vc.drawHint(os.Stdout, hint, vc.hintVars(fileMeta, hint, extra))
		return nil
	}
	if err := redraw(); err != nil {
		return err
	}

	for {
		select {
		case <-sigs:
			return nil

		case in := <-inputs:
			vc.termCols, vc.termRows = resolveViewerTermSize(initWidth, initHeight)

			changed := false
			shouldQuit := false

			processInput := func(tok string) {
				if tok != "q" {
					fitInitialFrame = false
				}
				if m, ok := inputSpec.ParseMouse(tok); ok {
					c, r := m.Col-1, m.Row-1
					// Space-pan: intercept canvas mouse motion when active.
					if spacePan && (m.IsMove() || m.IsDrag()) && !m.IsScroll() && m.Row != vc.termRows-1 {
						if !spacePanAnchor.Active {
							spacePanAnchor = viewgeom.NewPanAnchor(c, r, vc.state.panX, vc.state.panY)
						}
						vc.state.panX, vc.state.panY = vc.rc.viewSpec().PanFromAnchor(spacePanAnchor, c, r)
						vc.rerender()
						changed = true
						return
					}
					quit, ch, _ := vc.handleMouse(m)
					if quit {
						shouldQuit = true
					}
					if ch {
						changed = true
					}
				} else {
					quit, ch, unhandled := vc.handleKey(tok)
					if quit {
						shouldQuit = true
						return
					}
					if ch {
						changed = true
						return
					}
					// Image-viewer-specific: toggle_pan.
					if unhandled == "toggle_pan" {
						spacePan = !spacePan
						spacePanAnchor = dragState{}
						if spacePan {
							fmt.Fprint(os.Stdout, "\x1b[?1003h\x1b[?1006h")
							vc.status = "pan  —  move mouse to pan · Space to exit"
						} else {
							fmt.Fprint(os.Stdout, "\x1b[?1002h\x1b[?1006h")
							vc.status = ""
						}
						changed = true
					}
				}
			}

			processInput(in)
			if shouldQuit {
				return nil
			}

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
				if err := redraw(); err != nil {
					return err
				}
			}
		}
	}
}

// ── resolveTermSize ───────────────────────────────────────────────────────────

// resolveTermSize returns the effective terminal dimensions, auto-detecting any
// dimension that is ≤ 0, with a safe fallback when the terminal is not available.
func resolveTermSize(width, height int) (cols, rows int) {
	autoCols, autoRows := catiterm.TermWidth(), catiterm.TermHeight()
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

func resolveViewerTermSize(width, height int) (cols, rows int) {
	autoCols, autoRows := resolveTermSize(0, 0)
	cols = autoCols
	if width > 0 {
		cols = min(width, autoCols)
	}

	viewRows := max(1, autoRows-viewerChromeRows)
	if height > 0 {
		viewRows = min(height, viewRows)
	}
	rows = min(autoRows, viewRows+viewerChromeRows)
	return cols, rows
}

// recenterForMode adjusts panX/panY after a render-mode switch so the same
// region of the source image stays visible, despite the pixel budget change.
func recenterForMode(state *viewState, orig image.Image, termCols, termRows int, oldRC, newRC renderCfg) {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return
	}
	oldQ := oldRC.viewSpec()
	newQ := newRC.viewSpec()
	state.panX, state.panY = oldQ.Recenter(srcW, srcH, termCols, termRows, state.zoom, oldQ, newQ, state.panX, state.panY)
}

func preserveZoomForMode(state *viewState, orig image.Image, termCols, termRows int, oldRC, newRC renderCfg) {
	info, ok := currentZoomInfo(*state, orig, termCols, termRows, oldRC)
	if !ok || info.RawK <= 0 {
		return
	}
	b := orig.Bounds()
	mz := maxZoomForCfg(b.Dx(), b.Dy(), termCols, termRows, newRC)
	state.zoom = newRC.viewSpec().ZoomRatioForK(mz, info.RawK)
}

// ── zoom helpers ─────────────────────────────────────────────────────────────

// zoomAtCursor adjusts state so the pixel under the cursor (0-indexed terminal
// col/row) remains visually fixed after the zoom changes to newZoom.
//
// Derivation: let p = panX + col be the pixel in the scaled image under the
// cursor. After scaling by factor f = newZoom/oldZoom, that pixel moves to
// p*f in the new scaled image. To keep it under the cursor: newPanX = p*f - col.
func zoomAtCursor(state *viewState, newZoom float64, col, row int, mode renderMode) {
	mode.viewSpec().ZoomAtCursor(&state.zoom, &state.panX, &state.panY, newZoom, col, row)
}

// ── buildViewport / renderView ────────────────────────────────────────────────

// buildViewport returns the cropped+scaled image for the current view state.
// It also clamps state.panX/panY in-place so they never exceed image bounds.
func buildViewport(orig image.Image, state *viewState, termCols, termRows int, rc renderCfg) image.Image {
	return prepareRenderedImage(orig, state, termCols, termRows, rc, "")
}

// interactiveVideo plays a video file in the terminal using the caller's shared
// input channel so that keyboard events are routed through the browser's tokenizer.
// openAudio starts audio playback for path if the file has an audio stream.
// Returns nil silently on any failure so video continues without audio.
func openAudio(ctx context.Context, path string, startSec, endSec float64) *audio.Player {
	ok, err := audio.HasAudio(path)
	if err != nil || !ok {
		return nil
	}
	p, err := audio.Open(ctx, path, startSec, endSec)
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

func interactiveVideo(path string, initWidth, initHeight int, rc renderCfg, tr TimeRange, sharedInputs chan string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string, inputSpec *input.Spec, fullComp bool, initialZoom string) error {
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
		loadedInputSpec, loadErr := input.Load(fs.FS(spec.FS))
		if loadErr != nil {
			return fmt.Errorf("load input spec: %w", loadErr)
		}
		inputSpec = loadedInputSpec
	}
	if style == nil {
		style, err = loadStyle()
		if err != nil {
			return err
		}
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
	} else {
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

	vc := newViewerCore("video_player", initWidth, initHeight, rc, fullComp, inputSpec, style, labels, viewBtnRows, viewKeyMaps, btnActions)
	fileMeta := loadMediaMeta(path, true)

	displayFPS := 24.0
	if native, err := halfblock.ProbeVideoFPS(path); err == nil && native > 0 {
		displayFPS = native
	}
	interval := time.Duration(float64(time.Second) / displayFPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigs := make(chan os.Signal, 1)
	notifySignals(sigs, inputSpec.SignalsFor(input.EventQuit))
	defer signal.Stop(sigs)

	frames, cleanup, err := halfblock.OpenVideoStream(ctx, path, displayFPS, tr.Start, tr.End)
	if err != nil {
		return fmt.Errorf("open video: %w", err)
	}
	defer cleanup()

	var audioPlayer *audio.Player

	restartStream := func() {
		cleanup()
		frames, cleanup, err = halfblock.OpenVideoStream(ctx, path, displayFPS, tr.Start, tr.End)
		if err != nil {
			frames = nil
		}
		stopAudio(audioPlayer)
		audioPlayer = openAudio(ctx, path, tr.Start, tr.End)
	}

	paused := false
	videoEnded := false
	var statusClearAt time.Time
	var lastRawFrame image.Image
	vc.skipQuality = true // video starts playing; quality is computed on first pause

	vc.rerender = func() {
		if lastRawFrame == nil {
			return
		}
		vp := buildViewport(lastRawFrame, &vc.state, vc.termCols, vc.viewRows(), vc.rc)
		vc.lastVP = vp
		if vc.skipQuality {
			// Skip expensive quality computation while playing; curQ keeps its
			// last value. It is refreshed when playback pauses (see setPaused).
			return
		}
		ref := buildRef(lastRawFrame, vc.state, vc.termCols, vc.viewRows(), vc.rc, metrics.GridK, vc.fullComp)
		vc.curQ = computeQuality(ref, vp, vc.rc)
	}

	// checkGate throttles the expensive per-frame ANSI invariant check to at
	// most once per second while dimensions are stable. The first frame and any
	// dimension change always trigger a full check.
	checkGate := &renderCheckGate{interval: time.Second}

	setPaused := func(p bool) {
		paused = p
		vc.skipQuality = !p // compute quality when paused, skip while playing
		if p {
			audioPlayer.Pause()
			// Refresh quality now that we have time for it.
			if lastRawFrame != nil {
				ref := buildRef(lastRawFrame, vc.state, vc.termCols, vc.viewRows(), vc.rc, metrics.GridK, vc.fullComp)
				vc.curQ = computeQuality(ref, vc.lastVP, vc.rc)
			}
		} else {
			audioPlayer.Resume()
		}
	}

	renderCurrent := func() error {
		if vc.lastVP == nil {
			return nil
		}
		halfblock.CursorHome(os.Stdout)
		if err := renderValidatedGated(os.Stdout, lastRawFrame, vc.lastVP, vc.state, vc.termCols, vc.viewRows(), vc.rc, checkGate); err != nil {
			return err
		}
		halfblock.EraseDown(os.Stdout)
		return nil
	}

	handleVideoAction := func(action string) bool {
		switch action {
		case "toggle_play_pause":
			if videoEnded {
				restartStream()
				videoEnded = false
				setPaused(false)
			} else {
				setPaused(!paused)
			}
			return true
		}
		return false
	}

	processToken := func(tok string) (quit bool) {
		if m, ok := inputSpec.ParseMouse(tok); ok {
			prevStatus := vc.status
			q, changed, unhandled := vc.handleMouse(m)
			if q {
				return true
			}
			if unhandled != "" {
				if handleVideoAction(unhandled) {
					changed = true
				}
			}
			if vc.status != prevStatus && vc.status != "" {
				statusClearAt = time.Now().Add(3 * time.Second)
			}
			if changed && paused {
				_ = renderCurrent()
			}
			return false
		}
		prevStatus := vc.status
		quit, changed, unhandled := vc.handleKey(tok)
		if quit {
			return true
		}
		if unhandled != "" {
			if handleVideoAction(unhandled) {
				changed = true
			}
		}
		if vc.status != prevStatus && vc.status != "" {
			statusClearAt = time.Now().Add(3 * time.Second)
		}
		if changed && paused {
			_ = renderCurrent()
		}
		return false
	}

	audioPlayer = openAudio(ctx, path, tr.Start, tr.End)
	defer stopAudio(audioPlayer)

	for {
	drainMouse:
		for {
			select {
			case tok := <-mouseInputs:
				if processToken(tok) {
					return nil
				}
			default:
				break drainMouse
			}
		}

		select {
		case <-sigs:
			return nil

		case tok := <-keyInputs:
			if processToken(tok) {
				return nil
			}

		case <-ticker.C:
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
						vc.src = img
						b := img.Bounds()
						vc.lastSrcW, vc.lastSrcH = b.Dx(), b.Dy()
						if vc.state.zoom == 1.0 {
							vc.state.zoom = initialZoomRatio(initialZoom, vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows(), vc.rc.mode)
						}
						vc.rerender()
					}
				default:
				}
			}

			if vc.lastVP == nil {
				continue
			}
			if err := renderCurrent(); err != nil {
				return err
			}
			conditions := map[string]bool{"playing": !paused}
			vc.drawMenu(os.Stdout, conditions)
			if !vc.infoVisible && vc.status != "" && time.Now().After(statusClearAt) {
				vc.status = ""
			}
			hint := vc.labels["hint_viewer"]
			if vc.infoVisible && vc.src != nil {
				hint = formatZoomInfo(vc.state, vc.src, vc.termCols, vc.viewRows(), vc.rc)
			} else if vc.status != "" {
				hint = vc.status
			}
			fileMeta.DispW = fmt.Sprintf("%d", vc.termCols)
			fileMeta.DispH = fmt.Sprintf("%d", vc.viewRows())
			fileMeta.DispMode = rcDispMode(vc.rc)
			vc.drawHint(os.Stdout, hint, vc.hintVars(fileMeta, hint, nil))
		}
	}
}

func ellipsizeRunes(s string, max int) string { return browserui.EllipsizeRunes(s, max) }
