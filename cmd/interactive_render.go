package cmd

import (
	"fmt"
	"image"
	"io"
	"math"

	"ubunatic.com/cati/internal/viewgeom"
	spec "ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
)

// ── renderMode ────────────────────────────────────────────────────────────────

// renderMode identifies the image-rendering strategy.
type renderMode int

const (
	modeHalfblock renderMode = iota
	modeHalfSplit
	modeQuad
	modeSpark
	modeSparkQuad
	modeSextant
	modeSixHalf
	modeSparkSix

	// Backwards-compatible test/benchmark alias for the old spark/best name.
	modeSparkBest = modeSparkSix
)

func (m renderMode) pixCols(termCols int) int {
	return m.viewSpec().PixCols(termCols)
}

func (m renderMode) pixRows(termRows int) int {
	return m.viewSpec().PixRows(termRows)
}

func (m renderMode) viewSpec() viewgeom.Spec {
	switch m {
	case modeHalfSplit:
		return viewgeom.NewCell(2, 2, 2)
	case modeQuad:
		return viewgeom.NewCell(2, 2, 2)
	case modeSpark, modeSparkQuad:
		return viewgeom.NewCell(4, 8, 1)
	case modeSixHalf:
		return viewgeom.NewCellRatio(2, 6, 2, 3)
	case modeSparkSix:
		return viewgeom.NewCellRatio(4, 24, 1, 3)
	case modeSextant:
		return viewgeom.NewCellRatio(2, 3, 4, 3)
	default:
		return viewgeom.NewCell(1, 2, 1)
	}
}

func (m renderMode) useQuad() bool {
	return m == modeQuad
}

func (m renderMode) useSpark() bool {
	return m == modeHalfSplit || m == modeSpark || m == modeSparkQuad || m == modeSixHalf || m == modeSparkSix
}

func (m renderMode) useSextant() bool {
	return m == modeSextant
}

func (m renderMode) v2FitSpec() (viewgeom.V2Spec, bool) {
	switch m {
	case modeSextant:
		return viewgeom.NewV2CellRatio(2, 3, 4, 3), true
	case modeSixHalf:
		return viewgeom.NewV2CellRatio(2, 6, 2, 3), true
	case modeSparkSix:
		return viewgeom.NewV2CellRatio(4, 24, 1, 3), true
	default:
		return viewgeom.V2Spec{}, false
	}
}

// ── renderCfg ─────────────────────────────────────────────────────────────────

// renderCfg carries the active render mode (halfblock, quad variant, or sparkline).
// The zero value uses halfblock, matching the historic default.
//
// id is set by renderModes entries and used for equality/cycling; it must be
// unique per renderModes element. The zero id (0) belongs to "halfblock".
type renderCfg struct {
	id          int
	name        string
	mode        renderMode
	glyph       *spec.GlyphSetResolution
	sparkMode   sparkline.Mode
	sextantMode sextant.Mode
	quadOpts    quadblock.Options
	preScale    func(image.Image) image.Image // optional pre-scaler applied before ScaleToFit
	prescaler   prescaleMode
	jobs        int
	gray        bool                     // when true, convert image to grayscale before rendering
	grayColors  quadblock.ColorReduction // active grayscale palette level (ColorGray4/8/64/256)
	smart       bool                     // opt-in nearby-width PSNR selection for static renders
}

func (rc renderCfg) viewSpec() viewgeom.Spec {
	if rc.glyph != nil {
		return viewgeom.NewCellRatio(rc.glyph.Geometry.W, rc.glyph.Geometry.H, 2*rc.glyph.Geometry.W, rc.glyph.Geometry.H)
	}
	return rc.mode.viewSpec()
}

func (rc renderCfg) useGlyphs() bool  { return rc.glyph != nil }
func (rc renderCfg) useQuad() bool    { return !rc.useGlyphs() && rc.mode.useQuad() }
func (rc renderCfg) useSpark() bool   { return !rc.useGlyphs() && rc.mode.useSpark() }
func (rc renderCfg) useSextant() bool { return !rc.useGlyphs() && rc.mode.useSextant() }
func (rc renderCfg) renderCellSize() (int, int) {
	geom := rc.viewSpec()
	return geom.CellW, geom.CellH
}

func (rc renderCfg) v2FitSpec() (viewgeom.V2Spec, bool) {
	if rc.useGlyphs() {
		return viewgeom.NewV2CellRatio(rc.glyph.Geometry.W, rc.glyph.Geometry.H, 2*rc.glyph.Geometry.W, rc.glyph.Geometry.H), true
	}
	return rc.mode.v2FitSpec()
}

func (rc renderCfg) renderAspectCorrection() (int, int) {
	if spec, ok := rc.v2FitSpec(); ok {
		return spec.AspectNum, spec.AspectDen
	}
	geom := rc.viewSpec()
	return geom.AspectX, 1
}

func (rc renderCfg) glyphOptions() sparkline.Options {
	if rc.glyph == nil {
		return sparkline.Options{}
	}
	shapes := make([]sparkline.Shape, len(rc.glyph.Shapes))
	for i, shape := range rc.glyph.Shapes {
		shapes[i] = sparkline.Shape{Ch: shape.Glyph, Width: rc.glyph.Geometry.W, Height: rc.glyph.Geometry.H, Mask: shape.Mask}
	}
	return sparkline.Options{CellW: rc.glyph.Geometry.W, CellH: rc.glyph.Geometry.H, AspectX: 1, Shapes: shapes, Jobs: rc.jobs}
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
	if rc.useGlyphs() {
		b := img.Bounds()
		cellW, cellH := rc.renderCellSize()
		opts := rc.glyphOptions()
		opts.Rows = max(1, ceilDiv(b.Dy(), cellH))
		return sparkline.Render(w, img, max(1, ceilDiv(b.Dx(), cellW)), opts)
	}
	switch rc.mode {
	case modeSextant:
		return sextant.Render(w, img, 0, sextant.Options{Mode: rc.sextantMode, Jobs: rc.jobs})
	case modeHalfSplit, modeSpark, modeSparkQuad, modeSixHalf, modeSparkSix:
		b := img.Bounds()
		spec := rc.viewSpec()
		outCols := max(1, b.Dx()/spec.CellW)
		outRows := max(1, b.Dy()/spec.CellH)
		return sparkline.Render(w, img, outCols, sparkline.Options{Mode: rc.sparkMode, Rows: outRows, Jobs: rc.jobs, CellW: spec.CellW, CellH: spec.CellH, AspectX: spec.AspectX})
	case modeQuad:
		opts := rc.quadOpts
		opts.Jobs = rc.jobs
		return quadblock.Render(w, img, 0, opts)
	default:
		return halfblock.Render(w, img, 0, halfblock.Options{Jobs: rc.jobs})
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
	if rc.useGlyphs() {
		cellW, cellH := rc.renderCellSize()
		return renderCells{Cols: ceilDiv(w, cellW), Rows: ceilDiv(h, cellH)}
	}
	switch rc.mode {
	case modeSextant:
		return renderCells{Cols: ceilDiv(w, 2), Rows: ceilDiv(h, 3)}
	case modeHalfSplit, modeSpark, modeSparkQuad, modeSixHalf, modeSparkSix:
		spec := rc.viewSpec()
		return renderCells{Cols: max(1, w/spec.CellW), Rows: max(1, h/spec.CellH)}
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
	dims := rc.viewSpec().Dims(srcW, srcH, termCols, termRows, state.zoom)
	dims.ClampPan(&state.panX, &state.panY)
	viewW, viewH := dims.VisibleSize(state.panX, state.panY)
	if viewW <= 0 || viewH <= 0 {
		return renderCells{}
	}
	crop := image.Rect(viewgeom.SrcCrop(srcW, srcH, state.panX, state.panY, dims.ScaledW, dims.ScaledH, viewW, viewH))
	k := rc.viewSpec().MaxZoom(srcW, srcH, termCols, termRows) / state.zoom
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
	spec := rc.viewSpec()
	return cells.Cols * spec.CellW, cells.Rows * spec.CellH
}

func alignViewportSize(viewW, viewH int, rc renderCfg) (int, int) {
	if rc.useQuad() || rc.useSextant() || rc.useGlyphs() {
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
