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
	"sync"
	"syscall"
	"time"

	"codeberg.org/ubunatic/cati/internal/audio"
	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/input"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sparkline"
	"codeberg.org/ubunatic/cati/internal/viewgeom"
	spec "codeberg.org/ubunatic/cati/spec"
	"golang.org/x/term"
)

// ── renderMode ────────────────────────────────────────────────────────────────

// renderMode identifies the image-rendering strategy.
type renderMode int

const (
	modeHalfblock renderMode = iota
	modeQuad
	modeSpark
)

func (m renderMode) pixCols(termCols int) int {
	return m.viewSpec().PixCols(termCols)
}

func (m renderMode) pixRows(termRows int) int {
	return m.viewSpec().PixRows(termRows)
}

func (m renderMode) viewSpec() viewgeom.Spec {
	switch m {
	case modeQuad:
		return viewgeom.NewCell(2, 2, 2)
	case modeSpark:
		return viewgeom.NewCell(4, 8, 1)
	default:
		return viewgeom.NewCell(1, 2, 1)
	}
}

func (m renderMode) useQuad() bool {
	return m == modeQuad
}

func (m renderMode) useSpark() bool {
	return m == modeSpark
}

// ── renderCfg ─────────────────────────────────────────────────────────────────

// renderCfg carries the active render mode (halfblock, quad variant, or sparkline).
// The zero value uses halfblock, matching the historic default.
//
// id is set by renderModes entries and used for equality/cycling; it must be
// unique per renderModes element. The zero id (0) belongs to "halfblock".
type renderCfg struct {
	id         int
	mode       renderMode
	sparkMode  sparkline.Mode
	quadOpts   quadblock.Options
	preScale   func(image.Image) image.Image // optional pre-scaler applied before ScaleToFit
	gray       bool                          // when true, convert image to grayscale before rendering
	grayColors quadblock.ColorReduction      // active grayscale palette level (ColorGray4/8/64/256)
}

// grayLevels is the cycle order for the G key: off → 256 → 64 → 8 → 4 → off.
var grayLevels = []quadblock.ColorReduction{
	quadblock.ColorGray256,
	quadblock.ColorGray64,
	quadblock.ColorGray8,
	quadblock.ColorGray4,
}

// cycleGray advances rc one step through the gray level cycle.
func cycleGray(rc *renderCfg) {
	if !rc.gray {
		rc.gray = true
		rc.grayColors = grayLevels[0]
		return
	}
	for i, l := range grayLevels {
		if l == rc.grayColors {
			if i+1 < len(grayLevels) {
				rc.grayColors = grayLevels[i+1]
			} else {
				rc.gray = false
			}
			return
		}
	}
	// unknown level — reset
	rc.gray = true
	rc.grayColors = grayLevels[0]
}

// grayColorsCount returns the number of gray shades for a given ColorReduction.
func grayColorsCount(cr quadblock.ColorReduction) int {
	switch cr {
	case quadblock.ColorGray4:
		return 4
	case quadblock.ColorGray8:
		return 8
	case quadblock.ColorGray16:
		return 16
	case quadblock.ColorGray64:
		return 64
	case quadblock.ColorGray256:
		return 256
	default:
		return 0
	}
}

func (rc renderCfg) scaleToFit(img image.Image, cols, rows int) image.Image {
	if rc.preScale != nil {
		img = rc.preScale(img)
	}
	switch rc.mode {
	case modeSpark:
		return sparkline.ScaleToFit(img, rc.mode.pixCols(cols), rc.mode.pixRows(rows))
	case modeQuad:
		return quadblock.ScaleToFit(img, cols, rows)
	default:
		return halfblock.ScaleToFit(img, cols, rows)
	}
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

func ellipsizeRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func viewerHintVars(meta MediaMeta, termCols int, hintTpl string, extra map[string]string) map[string]string {
	vars := meta.Vars()
	for k, v := range extra {
		vars[k] = v
	}
	vars["meta.name_short"] = ""
	available := max(0, termCols-2-tplWidth(hintTpl, vars))
	meta.NameShort = ellipsizeRunes(meta.Name, available)
	vars = meta.Vars()
	for k, v := range extra {
		vars[k] = v
	}
	return vars
}

func (rc renderCfg) render(w io.Writer, img image.Image) error {
	switch rc.mode {
	case modeSpark:
		b := img.Bounds()
		outCols := max(1, b.Dx()/4)
		outRows := max(1, b.Dy()/8)
		return sparkline.RenderOpts(w, img, outCols, outRows, rc.sparkMode)
	case modeQuad:
		return quadblock.RenderOpts(w, img, rc.quadOpts)
	default:
		return halfblock.Render(w, img)
	}
}

// renderModes is the cycle order for the R key. Each entry's cfg.id must equal
// its slice index so that cycleRenderCfg and rcModeName can find entries by id.
var renderModes = []struct {
	name string
	cfg  renderCfg
}{
	{"halfblock", renderCfg{id: 4}},
	{"spark/quad", renderCfg{id: 14, mode: modeSpark, sparkMode: sparkline.Quad}},
	// {"halfblock+sharp", renderCfg{id: 10, preScale: pixelart.Sharpen05}},
	// {"halfblock+epx2x", renderCfg{id: 8, preScale: pixelart.Scale2x}},
	// {"quad/pca2", renderCfg{id: 6, mode: modeQuad, quadOpts: quadblock.Options{PCA2: true}}},
	// {"quad/default", renderCfg{id: 7, mode: modeQuad}},
	// {"quad/lum+ambig", renderCfg{id: 1, mode: modeQuad, quadOpts: quadblock.Options{LumSplit: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/pca2+ambig", renderCfg{id: 2, mode: modeQuad, quadOpts: quadblock.Options{PCA2: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/splithalf+ambig", renderCfg{id: 3, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/lum-split", renderCfg{id: 5, mode: modeQuad, quadOpts: quadblock.Options{LumSplit: true}}},
	{"quad/splithalf", renderCfg{id: 0, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}},
	//{"quad/splithalf+sharp", renderCfg{id: 11, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Sharpen05}},
	//{"quad/splithalf+epx2x", renderCfg{id: 9, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Scale2x}},
	{"quad/edge-snap", renderCfg{id: 12, mode: modeQuad, quadOpts: quadblock.Options{EdgeSnap: true}}},
	// {"quad/edge-snap+ambig", renderCfg{id: 13, mode: modeQuad, quadOpts: quadblock.Options{EdgeSnap: true, Blend: quadblock.BlendAmbiguous}}},
}

// cycleRenderCfg returns the next renderCfg in the cycle and its display name.
// The gray state is carried over from the current cfg.
func cycleRenderCfg(rc renderCfg) (renderCfg, string) {
	for i, m := range renderModes {
		if m.cfg.id == rc.id {
			next := renderModes[(i+1)%len(renderModes)]
			next.cfg.gray = rc.gray
			next.cfg.grayColors = rc.grayColors
			return next.cfg, next.name
		}
	}
	next := renderModes[0]
	next.cfg.gray = rc.gray
	next.cfg.grayColors = rc.grayColors
	return next.cfg, next.name
}

// cycleRenderCfgPrev returns the previous renderCfg in the cycle and its display name.
// The gray state is carried over from the current cfg.
func cycleRenderCfgPrev(rc renderCfg) (renderCfg, string) {
	n := len(renderModes)
	for i, m := range renderModes {
		if m.cfg.id == rc.id {
			prev := renderModes[(i+n-1)%n]
			prev.cfg.gray = rc.gray
			prev.cfg.grayColors = rc.grayColors
			return prev.cfg, prev.name
		}
	}
	prev := renderModes[n-1]
	prev.cfg.gray = rc.gray
	prev.cfg.grayColors = rc.grayColors
	return prev.cfg, prev.name
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

// ── zoom levels ──────────────────────────────────────────────────────────────

// maxZoom returns the zoom level at which each terminal cell covers exactly
// 1 source pixel column × 2 source pixel rows (1:1 pixel-perfect).
func maxZoom(srcW, srcH, termCols, termRows int, mode renderMode) float64 {
	return mode.viewSpec().MaxZoom(srcW, srcH, termCols, termRows)
}

// ── Zoom-levels spec (from spec/zoom_levels.yaml) ──────────────────────────────

type zoomLevelsSpec struct {
	Levels []float64
	Extend string
}

var (
	zoomLevelsOnce   sync.Once
	zoomLevelsCached zoomLevelsSpec
)

func loadZoomLevels() zoomLevelsSpec {
	zoomLevelsOnce.Do(func() {
		zoomLevelsCached = zoomLevelsSpec{
			Levels: []float64{0.125, 0.25, 0.5, 0.75, 1.25},
			Extend: "adaptive",
		}
		specDef, err := spec.LoadZoomLevels()
		if err != nil {
			return
		}
		if len(specDef.Levels) > 0 {
			zoomLevelsCached.Levels = specDef.Levels
		}
		if specDef.Extend != "" {
			zoomLevelsCached.Extend = specDef.Extend
		}
	})
	return zoomLevelsCached
}

// zoomSteps returns a descending sequence of zoom values with ~1.25× ratio
// between consecutive steps. k = mz / zoom is the number of source columns
// per terminal cell. The sequence includes zoom-in (k < 1), 1:1 (k = 1),
// and zoom-out (k > 1) steps, all hitting exact integer k values.
func zoomSteps(mz float64, srcW int) []float64 {
	spec := loadZoomLevels()
	return viewgeom.ZoomSteps(mz, srcW, viewgeom.ZoomStepSpec{Levels: spec.Levels, Extend: spec.Extend})
}

// stepIdx returns the index of the nearest zoom step ≤ zoom (clamped to
// [0, len(steps)-1]). steps must be descending (zoomSteps order).
func stepIdx(zoom float64, steps []float64) int {
	return viewgeom.StepIdx(zoom, steps)
}

// initialZoomRatio parses the --zoom flag value and returns the corresponding
// zoom ratio (mz/k).  Empty string returns 1.0 (fit-to-viewport).
func initialZoomRatio(s string, srcW, srcH, termCols, termRows int, mode renderMode) float64 {
	return mode.viewSpec().InitialZoomRatio(s, srcW, srcH, termCols, termRows)
}

func parseZoomK(s string) float64 {
	return viewgeom.ParseZoomK(s)
}

// zoomLevel returns the current source-pixels-per-cell value formatted for the hint bar.
func zoomLevel(state viewState, orig image.Image, termCols, termRows int, rc renderCfg) string {
	b := orig.Bounds()
	cropW, _ := visibleCrop(b.Dx(), b.Dy(), state, termCols, termRows, rc)
	if cropW <= 0 || termCols <= 0 {
		return rc.mode.viewSpec().ZoomLevel(state.zoom, b.Dx(), b.Dy(), termCols, termRows)
	}
	return fmt.Sprintf("src px/cell=%.3g", float64(cropW)/float64(termCols))
}

// ── viewState ────────────────────────────────────────────────────────────────

// viewState holds the current zoom level and pan offset (in scaled pixels).
type viewState struct {
	zoom float64
	panX int // pixel offset into the zoomed image (x)
	panY int // pixel offset into the zoomed image (y)
}

// dragState tracks an in-progress pan gesture in terminal-cell coordinates.
type dragState = viewgeom.PanAnchor

// ── interactive ──────────────────────────────────────────────────────────────

func interactive(path string, initWidth, initHeight int, rc renderCfg, fullComp bool, initialZoom string) error {
	return interactiveWithChan(path, initWidth, initHeight, rc, nil, nil, nil, nil, nil, nil, fullComp, initialZoom)
}

func interactiveWithChan(path string, initWidth, initHeight int, rc renderCfg, sharedInputs chan string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string, inputSpec *input.Spec, fullComp bool, initialZoom string) error {
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
	if viewKeyMaps == nil {
		viewKeyMaps = buildViewKeyMaps(viewBtnRows, loadButtonKeyDefs(inputSpec))
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

	termCols, termRows := resolveTermSize(initWidth, initHeight)

	viewRows := max(1, termRows-2)
	state := viewState{zoom: initialZoomRatio(initialZoom, orig.Bounds().Dx(), orig.Bounds().Dy(), termCols, viewRows, rc.mode)}
	var drag dragState
	var spacePan bool
	var spacePanAnchor dragState

	var buttons []menuButton
	activeAction := ""
	var status string
	var lastKey string
	modeName := rcModeName(rc)
	lastNonHBID := rc.id // last non-halfblock mode id; used by toggle_halfblock
	if rc.mode != modeQuad {
		lastNonHBID = -1
	}
	var curQ metrics.RenderQuality
	fileMeta := loadMediaMeta(path, false)
	redraw := func() {
		halfblock.CursorHome(os.Stdout)
		vp := renderView(orig, &state, termCols, max(1, termRows-2), rc)
		ref := buildRef(orig, state, termCols, max(1, termRows-2), rc, metrics.GridK, fullComp)
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
		graySuffix := ""
		if rc.gray {
			graySuffix = fmt.Sprintf(" (gray%d)", grayColorsCount(rc.grayColors))
		}
		hintVars := viewerHintVars(fileMeta, termCols, hint, map[string]string{
			"last_key":    lastKey,
			"ssim":        fmt.Sprintf("%.3f", curQ.SSIM),
			"blockiness":  fmt.Sprintf("%.3f", curQ.Blockiness),
			"edge_cont":   fmt.Sprintf("%.3f", curQ.EdgeCont),
			"render_mode": modeName + graySuffix,
			"zoom_level":  zoomLevel(state, orig, termCols, max(1, termRows-2), rc),
		})
		// Override src_res with visible crop region when zoomed/panning.
		b := orig.Bounds()
		if cw, ch := visibleCrop(b.Dx(), b.Dy(), state, termCols, max(1, termRows-2), rc); cw > 0 && ch > 0 {
			if cw != b.Dx() || ch != b.Dy() {
				hintVars["meta.src_res"] = fmt.Sprintf("%d×%d", cw, ch)
			}
		}
		drawHintBar(os.Stdout, termRows, hint, hintVars, style)
	}
	redraw()

	for {
		select {
		case <-sigs:
			return nil

		case in := <-inputs:
			termCols, termRows = resolveTermSize(initWidth, initHeight)
			viewRows = max(1, termRows-2)
			mz := maxZoom(orig.Bounds().Dx(), orig.Bounds().Dy(), termCols, viewRows, rc.mode)
			k := max(1, int(math.Round(mz/state.zoom))) // source columns per cell
			geom := rc.mode.viewSpec()
			hStep := max(1, min(termCols/8, k))
			vStep := max(1, min(viewRows/8, k))

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
								steps := zoomSteps(maxZoom(orig.Bounds().Dx(), orig.Bounds().Dy(), termCols, viewRows, rc.mode), orig.Bounds().Dx())
								i := stepIdx(state.zoom, steps)
								if i > 0 {
									state.zoom = steps[i-1]
									changed = true
								}
							case "dec_zoom":
								steps := zoomSteps(maxZoom(orig.Bounds().Dx(), orig.Bounds().Dy(), termCols, viewRows, rc.mode), orig.Bounds().Dx())
								i := stepIdx(state.zoom, steps)
								if i < len(steps)-1 {
									state.zoom = steps[i+1]
									changed = true
								}
							case "cycle_render":
								oldRC := rc
								rc, modeName = cycleRenderCfg(rc)
								recenterForMode(&state, orig, termCols, max(1, termRows-2), oldRC, rc)
								changed = true
							case "cycle_render_prev":
								oldRC := rc
								rc, modeName = cycleRenderCfgPrev(rc)
								recenterForMode(&state, orig, termCols, max(1, termRows-2), oldRC, rc)
								changed = true
							case "toggle_gray":
								cycleGray(&rc)
								changed = true
							case "toggle_halfblock":
								oldRC := rc
								if rc.mode.useQuad() {
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
								rc.gray = oldRC.gray
								rc.grayColors = oldRC.grayColors
								recenterForMode(&state, orig, termCols, max(1, termRows-2), oldRC, rc)
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
						steps := zoomSteps(maxZoom(orig.Bounds().Dx(), orig.Bounds().Dy(), termCols, viewRows, rc.mode), orig.Bounds().Dx())
						i := stepIdx(state.zoom, steps)
						if m.ScrollDir() < 0 && i > 0 {
							zoomAtCursor(&state, steps[i-1], c, r, rc.mode)
							changed = true
						} else if m.ScrollDir() >= 0 && i < len(steps)-1 {
							zoomAtCursor(&state, steps[i+1], c, r, rc.mode)
							changed = true
						}

					// ── Space-pan: any mouse motion pans when Space is held ───────
					case spacePan && (m.IsMove() || m.IsDrag()) && !m.IsScroll():
						if !spacePanAnchor.Active {
							spacePanAnchor = viewgeom.NewPanAnchor(c, r, state.panX, state.panY)
						}
						state.panX, state.panY = geom.PanFromAnchor(spacePanAnchor, c, r)
						changed = true

					// ── Left press: start drag ────────────────────────────────────
					case !spacePan && !m.IsScroll() && !m.IsDrag() && m.Button == 0 && !m.Release:
						drag = viewgeom.NewPanAnchor(c, r, state.panX, state.panY)

					// ── Left drag: update pan ─────────────────────────────────────
					case !spacePan && m.IsDrag() && m.Button == 0 && drag.Active:
						// Grab-and-pull: dragging right shows more of the left side.
						state.panX, state.panY = geom.PanFromAnchor(drag, c, r)
						changed = true

					// ── Left release: end drag ────────────────────────────────────
					case !m.IsScroll() && !m.IsDrag() && m.Button == 0 && m.Release:
						drag.Active = false
					}

				} else {
					// ── Keyboard event ────────────────────────────────────────────
					switch tok {
					case "\x03": // ctrl-c always quits regardless of spec
						shouldQuit = true

					case "\x1b[A": // ↑ — structural pan (no button equiv)
						geom.PanByCells(&state.panX, &state.panY, 0, -vStep)
						changed = true

					case "\x1b[B": // ↓ — structural pan
						geom.PanByCells(&state.panX, &state.panY, 0, vStep)
						changed = true

					case "\x1b[C": // → — structural pan
						geom.PanByCells(&state.panX, &state.panY, hStep, 0)
						changed = true

					case "\x1b[D": // ← — structural pan
						geom.PanByCells(&state.panX, &state.panY, -hStep, 0)
						changed = true

					default:
						if action, ok := viewKeyMaps["image_viewer"][tok]; ok {
							switch action {
							case "go_back", "quit":
								shouldQuit = true
							case "zoom_k":
								k := 1.0
								if len(tok) == 1 && tok[0] >= '0' && tok[0] <= '9' {
									val := int(tok[0] - '0')
									if val == 0 {
										state.zoom = 1.0
										changed = true
										break
									}
									if val == 1 {
										k = 1.0
									} else {
										k = float64(val)
									}
								}
								srcW := orig.Bounds().Dx()
								if srcW > 0 {
									k = math.Max(k, 1.0/float64(srcW))
									k = math.Min(k, float64(srcW))
								}
								mz := maxZoom(srcW, orig.Bounds().Dy(), termCols, viewRows, rc.mode)
								state.zoom = rc.mode.viewSpec().ZoomRatioForK(mz, k)
								changed = true
							case "inc_zoom":
								steps := zoomSteps(maxZoom(orig.Bounds().Dx(), orig.Bounds().Dy(), termCols, viewRows, rc.mode), orig.Bounds().Dx())
								i := stepIdx(state.zoom, steps)
								if i > 0 {
									state.zoom = steps[i-1]
									changed = true
								}
							case "dec_zoom":
								steps := zoomSteps(maxZoom(orig.Bounds().Dx(), orig.Bounds().Dy(), termCols, viewRows, rc.mode), orig.Bounds().Dx())
								i := stepIdx(state.zoom, steps)
								if i < len(steps)-1 {
									state.zoom = steps[i+1]
									changed = true
								}
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
								oldRC := rc
								rc, modeName = cycleRenderCfg(rc)
								recenterForMode(&state, orig, termCols, max(1, termRows-2), oldRC, rc)
								changed = true
							case "cycle_render_prev":
								oldRC := rc
								rc, modeName = cycleRenderCfgPrev(rc)
								recenterForMode(&state, orig, termCols, max(1, termRows-2), oldRC, rc)
								changed = true
							case "toggle_gray":
								cycleGray(&rc)
								changed = true
							case "toggle_halfblock":
								oldRC := rc
								if rc.mode.useQuad() {
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
								rc.gray = oldRC.gray
								rc.grayColors = oldRC.grayColors
								recenterForMode(&state, orig, termCols, max(1, termRows-2), oldRC, rc)
								changed = true
							}
						}
					}
				}
			}

			// Process first event
			processInput(in)
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

// viewportDims computes the derived pixel dimensions for a given view state.
//
//	pixCols  — pixel budget per row (termCols × 2 for quad, else termCols)
//	pixRows  — pixel budget per column (termRows × 2, termRows for sparkline)
//	scaledW  — full NN-scaled width  after zoom (≥ pixCols on zoom-in)
//	scaledH  — full NN-scaled height after zoom (≥ pixRows on zoom-in)
//	viewW    — viewport width  clamped to min(pixCols, scaledW)
//	viewH    — viewport height clamped to min(pixRows, scaledH)
func viewportDims(srcW, srcH, termCols, termRows int, zoom float64, mode renderMode) (pixCols, pixRows, scaledW, scaledH, viewW, viewH int) {
	return mode.viewSpec().ViewportDims(srcW, srcH, termCols, termRows, zoom)
}

// srcCrop maps viewport pixel coords back to source image coords and returns
// the visible source rectangle.
func srcCrop(srcW, srcH, panX, panY, scaledW, scaledH, viewW, viewH int) (x0, y0, x1, y1 int) {
	return viewgeom.SrcCrop(srcW, srcH, panX, panY, scaledW, scaledH, viewW, viewH)
}

// visibleCrop returns the size of the visible source region in source pixels
// for the current view state. Returns (0, 0) when the image is empty.
func visibleCrop(srcW, srcH int, state viewState, termCols, termRows int, rc renderCfg) (int, int) {
	return rc.mode.viewSpec().VisibleCrop(srcW, srcH, state.zoom, state.panX, state.panY, termCols, termRows)
}

// applyGrayIf returns a grayscale copy of img when rc.gray is true, otherwise img.
func applyGrayIf(img image.Image, rc renderCfg) image.Image {
	if rc.gray {
		return quadblock.ReduceColors(img, rc.grayColors)
	}
	return img
}

// recenterForMode adjusts panX/panY after a render-mode switch so the same
// region of the source image stays visible, despite the pixel budget change.
func recenterForMode(state *viewState, orig image.Image, termCols, termRows int, oldRC, newRC renderCfg) {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return
	}
	oldQ := oldRC.mode.viewSpec()
	newQ := newRC.mode.viewSpec()
	state.panX, state.panY = oldQ.Recenter(srcW, srcH, termCols, termRows, state.zoom, oldQ, newQ, state.panX, state.panY)
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
	if rc.gray {
		orig = quadblock.ReduceColors(orig, rc.grayColors)
	}
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig
	}
	dims := rc.mode.viewSpec().Dims(srcW, srcH, termCols, termRows, state.zoom)

	scaled := halfblock.ScaleNN(orig, dims.ScaledW, dims.ScaledH)

	dims.ClampPan(&state.panX, &state.panY)
	return imgutil.CropImage(scaled, state.panX, state.panY, dims.ViewW, dims.ViewH)
}

// renderView renders the current viewport to stdout and returns it for callers
// that need the pixel data (e.g. to compute SSIM).
func renderView(orig image.Image, state *viewState, termCols, termRows int, rc renderCfg) image.Image {
	vp := buildViewport(orig, state, termCols, termRows, rc)
	_ = rc.render(os.Stdout, vp)
	return vp
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

func interactiveVideo(path string, initWidth, initHeight int, rc renderCfg, sharedInputs chan string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string, inputSpec *input.Spec, fullComp bool, initialZoom string) error {
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
	if !rc.mode.useQuad() {
		lastNonHBID = -1
	}
	var curQ metrics.RenderQuality
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
							src := applyGrayIf(lastRawFrame, rc)
							lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
							qW, qH := metrics.QualityGridDims(lastFrame.Bounds().Dx(), lastFrame.Bounds().Dy(), rc.mode.pixCols(1), rc.mode.pixRows(1), metrics.GridK)
							ref := src
							if !fullComp {
								ref = metrics.PyramidDownscale(src, qW, qH)
							}
							curQ = computeQuality(ref, lastFrame, rc)
						}
					case "cycle_render_prev":
						rc, modeName = cycleRenderCfgPrev(rc)
						if lastRawFrame != nil {
							src := applyGrayIf(lastRawFrame, rc)
							lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
							qW, qH := metrics.QualityGridDims(lastFrame.Bounds().Dx(), lastFrame.Bounds().Dy(), rc.mode.pixCols(1), rc.mode.pixRows(1), metrics.GridK)
							ref := src
							if !fullComp {
								ref = metrics.PyramidDownscale(src, qW, qH)
							}
							curQ = computeQuality(ref, lastFrame, rc)
						}
					case "toggle_gray":
						cycleGray(&rc)
						if lastRawFrame != nil {
							src := applyGrayIf(lastRawFrame, rc)
							lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
							qW, qH := metrics.QualityGridDims(lastFrame.Bounds().Dx(), lastFrame.Bounds().Dy(), rc.mode.pixCols(1), rc.mode.pixRows(1), metrics.GridK)
							ref := src
							if !fullComp {
								ref = metrics.PyramidDownscale(src, qW, qH)
							}
							curQ = computeQuality(ref, lastFrame, rc)
						}
					case "toggle_halfblock":
						graySaved := rc.gray
						grayColorsSaved := rc.grayColors
						if rc.mode.useQuad() {
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
						rc.gray = graySaved
						rc.grayColors = grayColorsSaved
						if lastRawFrame != nil {
							src := applyGrayIf(lastRawFrame, rc)
							lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
							qW, qH := metrics.QualityGridDims(lastFrame.Bounds().Dx(), lastFrame.Bounds().Dy(), rc.mode.pixCols(1), rc.mode.pixRows(1), metrics.GridK)
							ref := src
							if !fullComp {
								ref = metrics.PyramidDownscale(src, qW, qH)
							}
							curQ = computeQuality(ref, lastFrame, rc)
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
						src := applyGrayIf(lastRawFrame, rc)
						lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
						b := lastFrame.Bounds()
						curQ = computeQuality(metrics.PyramidDownscale(src, b.Dx(), b.Dy()), lastFrame, rc)
					}

				case "cycle_render_prev":
					rc, modeName = cycleRenderCfgPrev(rc)
					if lastRawFrame != nil {
						src := applyGrayIf(lastRawFrame, rc)
						lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
						b := lastFrame.Bounds()
						curQ = computeQuality(metrics.PyramidDownscale(src, b.Dx(), b.Dy()), lastFrame, rc)
					}

				case "toggle_gray":
					cycleGray(&rc)
					if lastRawFrame != nil {
						src := applyGrayIf(lastRawFrame, rc)
						lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
						b := lastFrame.Bounds()
						curQ = computeQuality(metrics.PyramidDownscale(src, b.Dx(), b.Dy()), lastFrame, rc)
					}

				case "toggle_halfblock":
					graySaved := rc.gray
					grayColorsSaved := rc.grayColors
					if rc.mode.useQuad() {
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
					rc.gray = graySaved
					rc.grayColors = grayColorsSaved
					if lastRawFrame != nil {
						src := applyGrayIf(lastRawFrame, rc)
						lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
						b := lastFrame.Bounds()
						curQ = computeQuality(metrics.PyramidDownscale(src, b.Dx(), b.Dy()), lastFrame, rc)
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
						src := applyGrayIf(img, rc)
						lastFrame = rc.scaleToFit(src, termCols, max(1, termRows-2))
						{
							qW, qH := metrics.QualityGridDims(lastFrame.Bounds().Dx(), lastFrame.Bounds().Dy(), rc.mode.pixCols(1), rc.mode.pixRows(1), metrics.GridK)
							ref := src
							if !fullComp {
								ref = metrics.PyramidDownscale(src, qW, qH)
							}
							curQ = computeQuality(ref, lastFrame, rc)
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
			graySuffix := ""
			if rc.gray {
				graySuffix = fmt.Sprintf(" (gray%d)", grayColorsCount(rc.grayColors))
			}
			hintVars := viewerHintVars(fileMeta, termCols, hint, map[string]string{
				"last_key":    lastKey,
				"ssim":        fmt.Sprintf("%.3f", curQ.SSIM),
				"blockiness":  fmt.Sprintf("%.3f", curQ.Blockiness),
				"edge_cont":   fmt.Sprintf("%.3f", curQ.EdgeCont),
				"render_mode": modeName + graySuffix,
			})
			drawHintBar(os.Stdout, termRows, hint, hintVars, style)
		}
	}
}
