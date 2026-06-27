package cmd

import (
	"image"

	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
)

// buildRef returns a pyramid-downscale reference at the quality-grid resolution
// (when qualityK > 0) or at viewport pixel dimensions (when qualityK ≤ 0).
// The reference is computed from the original image (not the already-NN-scaled
// viewport), so all rendering modes are measured against the same ideal.
//
// qualityK > 0 produces a K × K sub-pixel grid per terminal cell, enabling
// fair SSIM comparison across render modes with different native sub-pixel
// layouts.
//
// state must already be clamped (call buildViewport first).
func buildRef(orig image.Image, state viewState, termCols, termRows int, rc renderCfg, qualityK int, fullComp bool) image.Image {
	if rc.gray {
		orig = quadblock.ReduceColors(orig, rc.grayColors)
	}
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig
	}
	_, _, scaledW, scaledH, viewW, viewH := viewportDims(srcW, srcH, termCols, termRows, state.zoom, rc.mode)

	viewW = min(viewW, scaledW-state.panX)
	viewH = min(viewH, scaledH-state.panY)
	if viewW <= 0 || viewH <= 0 {
		return orig
	}
	x0, y0, x1, y1 := srcCrop(srcW, srcH, state.panX, state.panY, scaledW, scaledH, viewW, viewH)
	region := imgutil.CropImage(orig, x0, y0, x1-x0, y1-y0)
	if fullComp {
		return metrics.PyramidDownscale(region, viewW, viewH)
	}
	if qualityK > 0 {
		gw, gh := metrics.QualityGridDims(viewW, viewH, rc.mode.pixCols(1), rc.mode.pixRows(1), qualityK)
		return metrics.PyramidDownscale(region, gw, gh)
	}
	return metrics.PyramidDownscale(region, viewW, viewH)
}

// renderSSIM returns SSIM(ref, rendered) where rendered is the faithful
// pixel reconstruction of the block-char output. For halfblock this is vp
// itself (lossless in colour). For quad modes it is RenderToImage — the exact
// fg/bg assignment each algorithm makes per 2×2 block — so different quad
// algorithms produce different SSIM scores. ref should be a box-filter
// downsample of the original source region.
func renderSSIM(ref, vp image.Image, rc renderCfg) float64 {
	if rc.mode.useQuad() {
		return metrics.SSIMLuminance(ref, quadblock.RenderToImage(vp, rc.quadOpts))
	}
	return metrics.SSIMLuminance(ref, vp)
}

// rcModeName returns the display name of rc in renderModes, or "?" if unknown.
// Matches by cfg.id because renderCfg contains a func field and is not comparable.
func rcModeName(rc renderCfg) string {
	for _, m := range renderModes {
		if m.cfg.id == rc.id {
			return m.name
		}
	}
	return "?"
}
