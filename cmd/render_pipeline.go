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

const maxTargetDim = int(^uint(0) >> 1)

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
		zoom := rc.mode.viewSpec().InitialZoomRatio(initialZoom, srcW, srcH, termCols, termRows)
		dims := rc.mode.viewSpec().Dims(srcW, srcH, termCols, termRows, zoom)
		scaledW, scaledH := alignScaledSize(dims.ScaledW, dims.ScaledH, rc)
		return resizeRenderedImage(orig, scaledW, scaledH, rc)
	}

	dims := rc.mode.viewSpec().Dims(srcW, srcH, termCols, termRows, state.zoom)
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
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return img
	}
	spec := rc.mode.viewSpec()
	maxW, maxH := cols*spec.CellW, rows*spec.CellH
	if cols <= 0 {
		maxW = maxTargetDim
	}
	if rows <= 0 {
		maxH = maxTargetDim
	}
	if maxW < 1 || maxH < 1 {
		return img
	}
	// Fit the aspect-corrected source into the viewport, allowing upscale.
	// imgutil.FitPixelDims never upscales; here we always fill the viewport.
	acSrcW := srcW * spec.AspectX
	var rawW, rawH int
	if acSrcW*maxH >= srcH*maxW { // width-bound
		rawW = maxW
		rawH = max(1, srcH*maxW/acSrcW)
	} else { // height-bound
		rawH = maxH
		rawW = max(1, acSrcW*maxH/srcH)
	}
	targetW, targetH := alignScaledSize(rawW, rawH, rc)
	// When rawH extends into the next cell by at least half a cell, keep the
	// natural height (not the truncated aligned height) and pad with only the
	// remaining transparent pixels to complete the cell boundary.
	// This limits BG transparency to ≤ CellH/2 pixels (half a char) rather
	// than a full CellH that would result from resizing to alignedH + full extension.
	var extH int
	if spec.CellH > 1 && rawH >= spec.CellH {
		if rem := rawH % spec.CellH; rem > 0 && rem*2 >= spec.CellH {
			targetH = rawH
			extH = spec.CellH - rem
		}
	}
	result := resizeRenderedImage(img, targetW, targetH, rc)
	if extH > 0 {
		result = appendTransparentRows(result, extH)
	}
	return result
}

// appendTransparentRows returns a new image with addH transparent (alpha=0) rows at the
// bottom. The transparent rows signal that the content does not fill that cell row.
func appendTransparentRows(img image.Image, addH int) image.Image {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()+addH))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x, y-b.Min.Y, img.At(x, y))
		}
	}
	// Rows [b.Dy()..b.Dy()+addH-1] remain zero-initialized (transparent).
	return out
}


// fittedCellSize returns the cell dimensions for img fitted to the given viewport.
func fittedCellSize(img image.Image, cols, rows int, rc renderCfg) renderCells {
	fitted := fitRenderedImage(img, cols, rows, rc)
	b := fitted.Bounds()
	return renderedCellSizeForPixels(b.Dx(), b.Dy(), rc)
}

// resizeRenderedImage resizes img through the shared render pipeline.
//
// The default prescaler is nearest-neighbour centered on pixel centres so it
// does not duplicate the leading column/row when downscaling. Pyramid filtering
// remains available for cases that want a smoother downsample.
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
	return scaleNNCentered(img, w, h)
}

// scaleNNCentered resizes img with nearest-neighbour sampling anchored at
// pixel centres instead of the top-left edge. This avoids over-representing the
// first column/row when downscaling.
func scaleNNCentered(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 || w < 1 || h < 1 {
		return img
	}
	if srcW == w && srcH == h {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := b.Min.Y + centeredSourceIndex(y, srcH, h)
		for x := 0; x < w; x++ {
			srcX := b.Min.X + centeredSourceIndex(x, srcW, w)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func centeredSourceIndex(dst, srcN, dstN int) int {
	return ((2*dst+1)*srcN - 1) / (2 * dstN)
}
