package cmd

import (
	"image"

	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/v1/quadblock"
	"codeberg.org/ubunatic/cati/v1/sextant"
	"codeberg.org/ubunatic/cati/v1/sparkline"
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
	dims := rc.mode.viewSpec().Dims(srcW, srcH, termCols, termRows, state.zoom)
	dims.ClampPan(&state.panX, &state.panY)

	viewW, viewH := dims.VisibleSize(state.panX, state.panY)
	viewW, viewH = alignViewportSize(viewW, viewH, rc)
	if viewW <= 0 || viewH <= 0 {
		return orig
	}
	crop := dims.SrcCrop(srcW, srcH, state.panX, state.panY)
	region := imgutil.CropImage(orig, crop.Min.X, crop.Min.Y, crop.Dx(), crop.Dy())
	if fullComp {
		return metrics.PyramidDownscale(region, viewW, viewH)
	}
	if qualityK > 0 {
		gw, gh := metrics.QualityGridDims(viewW, viewH, rc.mode.pixCols(1), rc.mode.pixRows(1), qualityK)
		if gh >= viewH {
			return metrics.PyramidDownscale(region, gw, gh)
		}
		// Quality grid is coarser than native render resolution (e.g. spark CellH=8
		// yields gh=viewH/2). Compare at full viewport dims to avoid 2× downscale
		// mismatch that penalises correct spark reconstructions.
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
	if rc.mode.useSextant() {
		if rc.jobs > 1 {
			return metrics.SSIMLuminance(ref, sextant.RenderToImageJ(vp, rc.sextantMode, rc.jobs))
		}
		return metrics.SSIMLuminance(ref, sextant.RenderToImage(vp, rc.sextantMode))
	}
	if rc.mode.useQuad() {
		if rc.jobs > 1 {
			return metrics.SSIMLuminance(ref, quadblock.RenderToImageJ(vp, rc.quadOpts, rc.jobs))
		}
		return metrics.SSIMLuminance(ref, quadblock.RenderToImage(vp, rc.quadOpts))
	}
	if rc.mode.useSpark() {
		b := vp.Bounds()
		outCols := max(1, b.Dx()/rc.mode.pixCols(1))
		outRows := max(1, b.Dy()/rc.mode.pixRows(1))
		if rc.jobs > 1 {
			rendered := sparkline.RenderToImageJ(vp, outCols, outRows, rc.sparkMode, rc.jobs)
			rb := ref.Bounds()
			vb := rendered.Bounds()
			if rb.Dx() != vb.Dx() || rb.Dy() != vb.Dy() {
				rendered = resizeRenderedImage(rendered, rb.Dx(), rb.Dy(), rc)
			}
			return metrics.SSIMLuminance(ref, rendered)
		}
		rendered := sparkline.RenderToImage(vp, outCols, outRows, rc.sparkMode)
		rb := ref.Bounds()
		vb := rendered.Bounds()
		if rb.Dx() != vb.Dx() || rb.Dy() != vb.Dy() {
			rendered = resizeRenderedImage(rendered, rb.Dx(), rb.Dy(), rc)
		}
		return metrics.SSIMLuminance(ref, rendered)
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

// rcDispMode returns the coarse display-geometry label used in metadata.
func rcDispMode(rc renderCfg) string {
	switch {
	case rc.mode.useSextant():
		return "sextant"
	case rc.mode.useQuad():
		return "quad"
	case rc.mode.useSpark():
		return "spark"
	default:
		return "half"
	}
}
