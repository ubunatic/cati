package cmd

import (
	"fmt"
	"image"
	"math"
	"sync"

	"ubunatic.com/cati/internal/viewgeom"
	spec "ubunatic.com/cati/spec"
)

func maxZoom(srcW, srcH, termCols, termRows int, mode renderMode) float64 {
	return mode.viewSpec().MaxZoom(srcW, srcH, termCols, termRows)
}

func maxZoomForCfg(srcW, srcH, termCols, termRows int, rc renderCfg) float64 {
	return rc.viewSpec().MaxZoom(srcW, srcH, termCols, termRows)
}

// ── Zoom-levels spec (from spec/zoom_levels.yaml) ──────────────────────────────

type zoomLevelsSpec struct {
	Levels             []float64
	Extend             string
	UpscaleSmallImages bool
}

var (
	zoomLevelsOnce   sync.Once
	zoomLevelsCached zoomLevelsSpec
)

func loadZoomLevels() zoomLevelsSpec {
	zoomLevelsOnce.Do(func() {
		zoomLevelsCached = zoomLevelsSpec{
			Levels:             []float64{0.125, 0.25, 0.5, 0.75, 1.25},
			Extend:             "adaptive",
			UpscaleSmallImages: true,
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
		zoomLevelsCached.UpscaleSmallImages = specDef.UpscaleSmallImages
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
// zoom ratio (mz/k).
func initialZoomRatio(s string, srcW, srcH, termCols, termRows int, mode renderMode) float64 {
	spec := loadZoomLevels()
	return mode.viewSpec().InitialZoomRatio(s, srcW, srcH, termCols, termRows, spec.UpscaleSmallImages)
}

func parseZoomK(s string) float64 {
	return viewgeom.ParseZoomK(s)
}

// zoomLevel returns the current source-pixels-per-cell value formatted for the hint bar.
func zoomLevel(state viewState, orig image.Image, termCols, termRows int, rc renderCfg) string {
	info, ok := currentZoomInfo(state, orig, termCols, termRows, rc)
	if !ok {
		b := orig.Bounds()
		return rc.viewSpec().ZoomLevel(state.zoom, b.Dx(), b.Dy(), termCols, termRows)
	}
	return fmt.Sprintf("src px/cell=%.3g", info.LadderK)
}

func currentZoomInfo(state viewState, orig image.Image, termCols, termRows int, rc renderCfg) (zoomInfo, bool) {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 || state.zoom <= 0 {
		return zoomInfo{}, false
	}
	dims := rc.viewSpec().Dims(srcW, srcH, termCols, termRows, state.zoom)
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
