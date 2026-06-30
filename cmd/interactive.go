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
	"codeberg.org/ubunatic/cati/internal/input"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sextant"
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
	modeSparkBest
	modeSextant
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
	case modeSpark, modeSparkBest:
		return viewgeom.NewCell(4, 8, 1)
	case modeSextant:
		return viewgeom.NewCell(2, 3, 1)
	default:
		return viewgeom.NewCell(1, 2, 1)
	}
}

func (m renderMode) useQuad() bool {
	return m == modeQuad
}

func (m renderMode) useSpark() bool {
	return m == modeSpark || m == modeSparkBest
}

func (m renderMode) useSextant() bool {
	return m == modeSextant
}

func (m renderMode) v2FitSpec() (viewgeom.V2Spec, bool) {
	if !m.useSextant() {
		return viewgeom.V2Spec{}, false
	}
	return viewgeom.NewV2CellRatio(2, 3, 4, 3), true
}

// ── renderCfg ─────────────────────────────────────────────────────────────────

// renderCfg carries the active render mode (halfblock, quad variant, or sparkline).
// The zero value uses halfblock, matching the historic default.
//
// id is set by renderModes entries and used for equality/cycling; it must be
// unique per renderModes element. The zero id (0) belongs to "halfblock".
type renderCfg struct {
	id          int
	mode        renderMode
	sparkMode   sparkline.Mode
	sextantMode sextant.Mode
	quadOpts    quadblock.Options
	preScale    func(image.Image) image.Image // optional pre-scaler applied before ScaleToFit
	prescaler   prescaleMode
	jobs        int
	gray        bool                     // when true, convert image to grayscale before rendering
	grayColors  quadblock.ColorReduction // active grayscale palette level (ColorGray4/8/64/256)
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
	return fitRenderedImage(img, cols, rows, rc)
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
	case modeSextant:
		if rc.jobs > 1 {
			return sextant.RenderJ(w, img, rc.sextantMode, rc.jobs)
		}
		return sextant.Render(w, img, rc.sextantMode)
	case modeSpark, modeSparkBest:
		b := img.Bounds()
		outCols := max(1, b.Dx()/4)
		outRows := max(1, b.Dy()/8)
		if rc.jobs > 1 {
			return sparkline.RenderJ(w, img, outCols, outRows, rc.sparkMode, rc.jobs)
		}
		return sparkline.RenderOpts(w, img, outCols, outRows, rc.sparkMode)
	case modeQuad:
		if rc.jobs > 1 {
			return quadblock.RenderJ(w, img, rc.quadOpts, rc.jobs)
		}
		return quadblock.RenderOpts(w, img, rc.quadOpts)
	default:
		if rc.jobs > 1 {
			return halfblock.RenderJ(w, img, rc.jobs)
		}
		return halfblock.Render(w, img)
	}
}

type renderCells struct {
	Cols int
	Rows int
}

type zoomInfo struct {
	RawK        float64
	LadderK     float64
	SrcW        int
	SrcH        int
	ScaledW     int
	ScaledH     int
	ViewW       int
	ViewH       int
	AlignedW    int
	AlignedH    int
	TrimW       int
	TrimH       int
	Rendered    renderCells
	SourceCells renderCells
	Crop        image.Rectangle
}

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

func renderedCellSize(vp image.Image, rc renderCfg) renderCells {
	b := vp.Bounds()
	return renderedCellSizeForPixels(b.Dx(), b.Dy(), rc)
}

func renderedCellSizeForPixels(w, h int, rc renderCfg) renderCells {
	switch rc.mode {
	case modeSextant:
		return renderCells{Cols: ceilDiv(w, 2), Rows: ceilDiv(h, 3)}
	case modeSpark, modeSparkBest:
		return renderCells{Cols: max(1, w/4), Rows: max(1, h/8)}
	case modeQuad:
		return renderCells{Cols: ceilDiv(w, 2), Rows: ceilDiv(h, 2)}
	default:
		return renderCells{Cols: w, Rows: ceilDiv(h, 2)}
	}
}

func expectedCellSize(orig image.Image, state viewState, termCols, termRows int, rc renderCfg) renderCells {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 || state.zoom <= 0 {
		return renderCells{}
	}
	dims := rc.mode.viewSpec().Dims(srcW, srcH, termCols, termRows, state.zoom)
	dims.ClampPan(&state.panX, &state.panY)
	viewW, viewH := dims.VisibleSize(state.panX, state.panY)
	if viewW <= 0 || viewH <= 0 {
		return renderCells{}
	}
	crop := image.Rect(viewgeom.SrcCrop(srcW, srcH, state.panX, state.panY, dims.ScaledW, dims.ScaledH, viewW, viewH))
	k := rc.mode.viewSpec().MaxZoom(srcW, srcH, termCols, termRows) / state.zoom
	if k <= 0 {
		return renderCells{}
	}
	return renderCells{
		Cols: min(termCols, max(1, int(math.Ceil(float64(crop.Dx())/k)))),
		Rows: min(termRows, max(1, int(math.Ceil(float64(crop.Dy())/(2*k))))),
	}
}

func viewportPixelSizeForCells(cells renderCells, rc renderCfg) (int, int) {
	if cells.Cols <= 0 || cells.Rows <= 0 {
		return 0, 0
	}
	spec := rc.mode.viewSpec()
	return cells.Cols * spec.CellW, cells.Rows * spec.CellH
}

func alignViewportSize(viewW, viewH int, rc renderCfg) (int, int) {
	if rc.mode.useQuad() || rc.mode.useSextant() {
		if viewW > 1 && viewW%2 != 0 {
			viewW--
		}
	}
	return viewW, viewH
}

func validateRenderSize(orig, vp image.Image, state viewState, termCols, termRows int, rc renderCfg) error {
	got := renderedCellSize(vp, rc)
	want := expectedCellSize(orig, state, termCols, termRows, rc)
	if want.Cols <= 0 || want.Rows <= 0 {
		return nil
	}
	if got != want {
		return fmt.Errorf("render size mismatch for %s: got %dx%d cells from viewport %dx%d, want %dx%d cells",
			rcModeName(rc), got.Cols, got.Rows, vp.Bounds().Dx(), vp.Bounds().Dy(), want.Cols, want.Rows)
	}
	return nil
}

func renderValidated(w io.Writer, orig, vp image.Image, state viewState, termCols, termRows int, rc renderCfg) error {
	if orig != nil {
		if err := validateRenderSize(orig, vp, state, termCols, termRows, rc); err != nil {
			return err
		}
	}
	return renderChecked(w, vp, rc)
}

// renderValidatedGated is like renderValidated but uses a time-gated ANSI
// check. The size/aspect validation on orig is also skipped when the gate is
// not yet due, so the hot video path only pays for the actual render work.
func renderValidatedGated(w io.Writer, orig, vp image.Image, state viewState, termCols, termRows int, rc renderCfg, gate *renderCheckGate) error {
	size := renderedCellSize(vp, rc)
	if orig != nil && gate.due(size) {
		if err := validateRenderSize(orig, vp, state, termCols, termRows, rc); err != nil {
			return err
		}
	}
	return renderCheckedGated(w, vp, rc, gate)
}

// renderModes is the cycle order for the R key. Each entry's cfg.id must be
// stable and unique so that cycleRenderCfg and rcModeName can find entries by id.
var renderModes = []struct {
	name string
	cfg  renderCfg
}{
	{"halfblock", renderCfg{id: 0}},
	// {"halfblock+sharp", renderCfg{id: 10, preScale: pixelart.Sharpen05}},
	// {"halfblock+epx2x", renderCfg{id: 8, preScale: pixelart.Scale2x}},
	// {"quad/pca2", renderCfg{id: 6, mode: modeQuad, quadOpts: quadblock.Options{PCA2: true}}},
	// {"quad/default", renderCfg{id: 7, mode: modeQuad}},
	// {"quad/lum+ambig", renderCfg{id: 1, mode: modeQuad, quadOpts: quadblock.Options{LumSplit: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/pca2+ambig", renderCfg{id: 2, mode: modeQuad, quadOpts: quadblock.Options{PCA2: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/splithalf+ambig", renderCfg{id: 3, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true, Blend: quadblock.BlendAmbiguous}}},
	// {"quad/lum-split", renderCfg{id: 5, mode: modeQuad, quadOpts: quadblock.Options{LumSplit: true}}},
	{"quad/splithalf", renderCfg{id: 1, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}},
	//{"quad/splithalf+sharp", renderCfg{id: 11, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Sharpen05}},
	//{"quad/splithalf+epx2x", renderCfg{id: 9, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Scale2x}},
	{"quad/edge-snap", renderCfg{id: 2, mode: modeQuad, quadOpts: quadblock.Options{EdgeSnap: true}}},
	// {"quad/edge-snap+ambig", renderCfg{id: 13, mode: modeQuad, quadOpts: quadblock.Options{EdgeSnap: true, Blend: quadblock.BlendAmbiguous}}},
	{"spark/quad", renderCfg{id: 3, mode: modeSpark, sparkMode: sparkline.Quad}},
	{"spark/best", renderCfg{id: 5, mode: modeSparkBest, sparkMode: sparkline.Best}},
	{"sextant/2x3", renderCfg{id: 6, mode: modeSextant, sextantMode: sextant.ModeSextant}},
}

func canonicalRenderCfg(rc renderCfg) renderCfg {
	for _, m := range renderModes {
		if sameRenderMode(rc, m.cfg) {
			canon := m.cfg
			canon.prescaler = rc.prescaler
			canon.jobs = rc.jobs
			canon.gray = rc.gray
			canon.grayColors = rc.grayColors
			return canon
		}
	}
	return rc
}

func sameRenderMode(a, b renderCfg) bool {
	if a.mode != b.mode || a.sparkMode != b.sparkMode {
		return false
	}
	if a.sextantMode != b.sextantMode {
		return false
	}
	if (a.preScale == nil) != (b.preScale == nil) {
		return false
	}
	return a.quadOpts == b.quadOpts
}

// cycleRenderCfg returns the next renderCfg in the cycle and its display name.
// The gray state is carried over from the current cfg.
func cycleRenderCfg(rc renderCfg) (renderCfg, string) {
	for i, m := range renderModes {
		if m.cfg.id == rc.id {
			next := renderModes[(i+1)%len(renderModes)]
			next.cfg.prescaler = rc.prescaler
			next.cfg.jobs = rc.jobs
			next.cfg.gray = rc.gray
			next.cfg.grayColors = rc.grayColors
			return next.cfg, next.name
		}
	}
	next := renderModes[0]
	next.cfg.prescaler = rc.prescaler
	next.cfg.jobs = rc.jobs
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
			prev.cfg.prescaler = rc.prescaler
			prev.cfg.jobs = rc.jobs
			prev.cfg.gray = rc.gray
			prev.cfg.grayColors = rc.grayColors
			return prev.cfg, prev.name
		}
	}
	prev := renderModes[n-1]
	prev.cfg.prescaler = rc.prescaler
	prev.cfg.jobs = rc.jobs
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
	info, ok := currentZoomInfo(state, orig, termCols, termRows, rc)
	if !ok {
		b := orig.Bounds()
		return rc.mode.viewSpec().ZoomLevel(state.zoom, b.Dx(), b.Dy(), termCols, termRows)
	}
	return fmt.Sprintf("src px/cell=%.3g", info.LadderK)
}

func currentZoomInfo(state viewState, orig image.Image, termCols, termRows int, rc renderCfg) (zoomInfo, bool) {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 || state.zoom <= 0 {
		return zoomInfo{}, false
	}
	dims := rc.mode.viewSpec().Dims(srcW, srcH, termCols, termRows, state.zoom)
	dims.ClampPan(&state.panX, &state.panY)
	viewW, viewH := dims.VisibleSize(state.panX, state.panY)
	alignedW, alignedH := alignViewportSize(viewW, viewH, rc)
	if alignedW <= 0 || alignedH <= 0 {
		return zoomInfo{}, false
	}
	cells := expectedCellSize(orig, state, termCols, termRows, rc)
	if cells.Cols <= 0 {
		return zoomInfo{}, false
	}
	rawCrop := image.Rect(viewgeom.SrcCrop(srcW, srcH, state.panX, state.panY, dims.ScaledW, dims.ScaledH, viewW, viewH))
	rawK := float64(rawCrop.Dx()) / float64(cells.Cols)
	return zoomInfo{
		RawK:        rawK,
		LadderK:     nearestZoomLevelK(rawK, srcW),
		SrcW:        srcW,
		SrcH:        srcH,
		ScaledW:     dims.ScaledW,
		ScaledH:     dims.ScaledH,
		ViewW:       viewW,
		ViewH:       viewH,
		AlignedW:    alignedW,
		AlignedH:    alignedH,
		TrimW:       viewW - alignedW,
		TrimH:       viewH - alignedH,
		Rendered:    cells,
		SourceCells: expectedCellSize(orig, state, termCols, termRows, rc),
		Crop:        rawCrop,
	}, true
}

func nearestZoomLevelK(k float64, srcW int) float64 {
	if k <= 0 {
		return k
	}
	steps := zoomSteps(1, srcW)
	best := k
	bestDelta := math.Inf(1)
	for _, z := range steps {
		if z <= 0 {
			continue
		}
		candidate := 1 / z
		delta := math.Abs(candidate - k)
		if delta < bestDelta {
			best = candidate
			bestDelta = delta
		}
	}
	return best
}

func formatZoomInfo(state viewState, orig image.Image, termCols, termRows int, rc renderCfg) string {
	info, ok := currentZoomInfo(state, orig, termCols, termRows, rc)
	if !ok {
		return "info unavailable"
	}
	trim := "none"
	if info.TrimW != 0 || info.TrimH != 0 {
		trim = fmt.Sprintf("%dx%d", info.TrimW, info.TrimH)
	}
	return fmt.Sprintf(
		"info raw=%.3g ladder=%.3g trim=%s crop=%dx%d cells=%dx%d src=%dx%d",
		info.RawK,
		info.LadderK,
		trim,
		info.Crop.Dx(), info.Crop.Dy(),
		info.Rendered.Cols, info.Rendered.Rows,
		info.SrcW, info.SrcH,
	)
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
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
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
		cw, ch := vc.rc.mode.viewSpec().VisibleCrop(srcW, srcH, vc.state.zoom, vc.state.panX, vc.state.panY, vc.termCols, vc.viewRows())
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
						vc.state.panX, vc.state.panY = vc.rc.mode.viewSpec().PanFromAnchor(spacePanAnchor, c, r)
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
	oldQ := oldRC.mode.viewSpec()
	newQ := newRC.mode.viewSpec()
	state.panX, state.panY = oldQ.Recenter(srcW, srcH, termCols, termRows, state.zoom, oldQ, newQ, state.panX, state.panY)
}

func preserveZoomForMode(state *viewState, orig image.Image, termCols, termRows int, oldRC, newRC renderCfg) {
	info, ok := currentZoomInfo(*state, orig, termCols, termRows, oldRC)
	if !ok || info.RawK <= 0 {
		return
	}
	b := orig.Bounds()
	mz := maxZoom(b.Dx(), b.Dy(), termCols, termRows, newRC.mode)
	state.zoom = newRC.mode.viewSpec().ZoomRatioForK(mz, info.RawK)
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
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	frames, cleanup, err := halfblock.OpenVideoStream(ctx, path, displayFPS, 0, 0)
	if err != nil {
		return fmt.Errorf("open video: %w", err)
	}
	defer cleanup()

	var audioPlayer *audio.Player

	restartStream := func() {
		cleanup()
		frames, cleanup, err = halfblock.OpenVideoStream(ctx, path, displayFPS, 0, 0)
		if err != nil {
			frames = nil
		}
		stopAudio(audioPlayer)
		audioPlayer = openAudio(ctx, path)
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

	audioPlayer = openAudio(ctx, path)
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
