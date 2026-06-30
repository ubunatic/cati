package cmd

import (
	"fmt"
	"image"
	"math"
	"strings"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
)

type prescaleMode int

const (
	prescaleNearestNeighbor prescaleMode = iota
	prescalePyramid
)

func parsePrescaleMode(mode string) (prescaleMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "nn", "nearest", "nearest-neighbor", "nearest-neighbour":
		return prescaleNearestNeighbor, nil
	case "pyramid", "pyr":
		return prescalePyramid, nil
	default:
		return 0, fmt.Errorf("unknown --prescaler %q; valid: nn, pyramid", mode)
	}
}

func (m prescaleMode) String() string {
	switch m {
	case prescalePyramid:
		return "pyramid"
	default:
		return "nn"
	}
}

// prepareRenderedImage applies the shared image pipeline used by static,
// interactive, and playback rendering.
//
// When state is nil, the image is fit to the terminal box using the supplied
// initial zoom flag. When state is non-nil, the image is treated as a viewport
// and cropped after zoom/pan normalization.
func prepareRenderedImage(orig image.Image, state *viewState, termCols, termRows int, rc renderCfg, initialZoom string) image.Image {
	img, err := prepareRenderedImageChecked(orig, state, termCols, termRows, rc, initialZoom)
	if err != nil {
		panic(err)
	}
	return img
}

func prepareRenderedImageChecked(orig image.Image, state *viewState, termCols, termRows int, rc renderCfg, initialZoom string) (image.Image, error) {
	if rc.gray {
		orig = quadblock.ReduceColors(orig, rc.grayColors)
	}
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig, nil
	}

	if state == nil {
		if initialZoom == "" {
			if termCols == 0 && termRows == 0 {
				termCols = halfblock.TermWidth()
				termRows = halfblock.TermHeight()
			}
			return fitRenderedImageChecked(orig, termCols, termRows, rc)
		}
		spec := rc.mode.viewSpec()
		zoom := spec.InitialZoomRatio(initialZoom, srcW, srcH, termCols, termRows)
		dims := spec.Dims(srcW, srcH, termCols, termRows, zoom)
		scaledW, scaledH := imgutil.AlignCellSize(dims.ScaledW, dims.ScaledH, spec.CellW, spec.CellH)
		if err := validateSourceAspect(rc, image.Rect(0, 0, srcW, srcH), scaledW, scaledH); err != nil {
			return nil, err
		}
		return resizeRenderedImage(orig, scaledW, scaledH, rc), nil
	}

	spec := rc.mode.viewSpec()
	dims := spec.Dims(srcW, srcH, termCols, termRows, state.zoom)
	scaled := resizeRenderedImage(orig, dims.ScaledW, dims.ScaledH, rc)

	dims.ClampPan(&state.panX, &state.panY)
	viewW, viewH := alignViewportSize(dims.ViewW, dims.ViewH, rc)
	vp := imgutil.CropImage(scaled, state.panX, state.panY, viewW, viewH)
	targetCells := expectedCellSize(orig, *state, termCols, termRows, rc)
	targetW, targetH := viewportPixelSizeForCells(targetCells, rc)
	if targetW > 0 && targetH > 0 {
		b := vp.Bounds()
		if b.Dx() != targetW || b.Dy() != targetH {
			vp = resizeRenderedImage(vp, targetW, targetH, rc)
		}
	}
	return vp, nil
}

func fitRenderedImage(img image.Image, cols, rows int, rc renderCfg) image.Image {
	result, err := fitRenderedImageChecked(img, cols, rows, rc)
	if err != nil {
		panic(err)
	}
	return result
}

func fitRenderedImageChecked(img image.Image, cols, rows int, rc renderCfg) (image.Image, error) {
	if rc.preScale != nil {
		img = rc.preScale(img)
	}
	b := img.Bounds()
	if spec, ok := rc.mode.v2FitSpec(); ok {
		plan := spec.Fit(b.Dx(), b.Dy(), cols, rows, false)
		if err := validateSourceAspectWith(rc, b, plan.RenderW, plan.RenderH, spec.AspectNum, spec.AspectDen, spec.CellW, spec.CellH); err != nil {
			return nil, err
		}
		result := resizeRenderedImage(img, plan.RenderW, plan.RenderH, rc)
		if plan.ExtH > 0 {
			result = imgutil.AppendTransparentRows(result, plan.ExtH)
		}
		return result, nil
	}
	spec := rc.mode.viewSpec()
	targetW, targetH, extH := imgutil.FitDims(b.Dx(), b.Dy(), spec.CellW, spec.CellH, spec.AspectX, cols, rows)
	if err := validateSourceAspect(rc, b, targetW, targetH); err != nil {
		return nil, err
	}
	result := resizeRenderedImage(img, targetW, targetH, rc)
	if extH > 0 {
		result = imgutil.AppendTransparentRows(result, extH)
	}
	return result, nil
}

func validateSourceAspect(rc renderCfg, src image.Rectangle, renderW, renderH int) error {
	aspectNum, aspectDen := rc.mode.renderAspectCorrection()
	cellW, cellH := rc.mode.renderCellSize()
	return validateSourceAspectWith(rc, src, renderW, renderH, aspectNum, aspectDen, cellW, cellH)
}

func validateSourceAspectWith(rc renderCfg, src image.Rectangle, renderW, renderH, aspectNum, aspectDen, cellW, cellH int) error {
	srcW, srcH := src.Dx(), src.Dy()
	if srcW <= 0 || srcH <= 0 || renderW <= 0 || renderH <= 0 {
		return nil
	}
	got := float64(renderW*aspectDen) / float64(renderH*aspectNum)
	want := float64(srcW) / float64(srcH)
	if got <= 0 || want <= 0 {
		return nil
	}
	relErr := math.Abs(got/want - 1)
	tolerance := float64(cellW)/float64(renderW) + float64(cellH)/float64(renderH)
	if relErr <= tolerance {
		return nil
	}
	return fmt.Errorf("render aspect mismatch for %s: source %dx%d aspect %.4g, viewport %dx%d with correction %d:%d gives %.4g (relative error %.3g > %.3g)",
		rcModeName(rc), srcW, srcH, want, renderW, renderH, aspectNum, aspectDen, got, relErr, tolerance)
}

func (m renderMode) renderAspectCorrection() (num, den int) {
	spec := m.viewSpec()
	return spec.AspectX, 1
}

func (m renderMode) renderCellSize() (cellW, cellH int) {
	spec := m.viewSpec()
	return spec.CellW, spec.CellH
}

func alignRenderedCellSize(w, h int, rc renderCfg) (int, int) {
	spec := rc.mode.viewSpec()
	if rc.mode.useSextant() {
		return imgutil.AlignCellSize(w, h, spec.CellW, 1)
	}
	return imgutil.AlignCellSize(w, h, spec.CellW, spec.CellH)
}

// fittedCellSize returns the cell dimensions for img fitted to the given viewport.
func fittedCellSize(img image.Image, cols, rows int, rc renderCfg) renderCells {
	fitted := fitRenderedImage(img, cols, rows, rc)
	b := fitted.Bounds()
	return renderedCellSizeForPixels(b.Dx(), b.Dy(), rc)
}

// resizeRenderedImage resizes img to w×h. Uses pyramid downscaling when
// prescalePyramid is set and the image is being reduced; NN otherwise.
func resizeRenderedImage(img image.Image, w, h int, rc renderCfg) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 || w < 1 || h < 1 {
		return img
	}
	if srcW == w && srcH == h {
		return img
	}
	if rc.prescaler == prescalePyramid && (w < srcW || h < srcH) {
		return metrics.PyramidDownscale(img, w, h)
	}
	return imgutil.ScaleNN(img, w, h)
}
