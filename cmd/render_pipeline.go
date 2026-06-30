package cmd

import (
	"fmt"
	"image"
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
	if rc.gray {
		orig = quadblock.ReduceColors(orig, rc.grayColors)
	}
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig
	}

	if state == nil {
		if initialZoom == "" {
			if termCols == 0 && termRows == 0 {
				termCols = halfblock.TermWidth()
				termRows = halfblock.TermHeight()
			}
			return fitRenderedImage(orig, termCols, termRows, rc)
		}
		spec := rc.mode.viewSpec()
		zoom := spec.InitialZoomRatio(initialZoom, srcW, srcH, termCols, termRows)
		dims := spec.Dims(srcW, srcH, termCols, termRows, zoom)
		scaledW, scaledH := imgutil.AlignCellSize(dims.ScaledW, dims.ScaledH, spec.CellW, spec.CellH)
		return resizeRenderedImage(orig, scaledW, scaledH, rc)
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
	return vp
}

func fitRenderedImage(img image.Image, cols, rows int, rc renderCfg) image.Image {
	if rc.preScale != nil {
		img = rc.preScale(img)
	}
	b := img.Bounds()
	if spec, ok := rc.mode.v2FitSpec(); ok {
		plan := spec.Fit(b.Dx(), b.Dy(), cols, rows, false)
		result := resizeRenderedImage(img, plan.RenderW, plan.RenderH, rc)
		if plan.ExtH > 0 {
			result = imgutil.AppendTransparentRows(result, plan.ExtH)
		}
		return result
	}
	spec := rc.mode.viewSpec()
	targetW, targetH, extH := imgutil.FitDims(b.Dx(), b.Dy(), spec.CellW, spec.CellH, spec.AspectX, cols, rows)
	result := resizeRenderedImage(img, targetW, targetH, rc)
	if extH > 0 {
		result = imgutil.AppendTransparentRows(result, extH)
	}
	return result
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
