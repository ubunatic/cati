package cmd

import (
	"image"

	"codeberg.org/ubunatic/cati/internal/geomshape"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sextant"
	"codeberg.org/ubunatic/cati/internal/sparkline"
)

// computeQuality returns all perceptual quality metrics for a rendered frame.
//
//   - ref  — pyramid-downscale reference (at viewport or quality-grid resolution)
//   - vp   — NN-scaled viewport from the shared render pipeline
//   - rc   — active render configuration
//
// The rendered terminal reconstruction is normalized to ref dimensions before
// scoring: smaller render grids are NN-upscaled, while denser render grids are
// pyramid-downscaled to the common quality grid.
func computeQuality(ref, vp image.Image, rc renderCfg) metrics.RenderQuality {
	var rendered image.Image
	switch {
	case rc.mode.useSextant():
		if rc.jobs > 1 {
			rendered = sextant.RenderToImageJ(vp, rc.sextantMode, rc.jobs)
		} else {
			rendered = sextant.RenderToImage(vp, rc.sextantMode)
		}
	case rc.mode.useGeomShape():
		if rc.jobs > 1 {
			rendered = geomshape.RenderToImageJWithSampler(vp, rc.geomShapeMode, rc.geomShapeSampler, rc.jobs)
		} else {
			rendered = geomshape.RenderToImageWithSampler(vp, rc.geomShapeMode, rc.geomShapeSampler)
		}
	case rc.mode.useQuad():
		if rc.jobs > 1 {
			rendered = quadblock.RenderToImageJ(vp, rc.quadOpts, rc.jobs)
		} else {
			rendered = quadblock.RenderToImage(vp, rc.quadOpts)
		}
	case rc.mode.useSpark():
		b := vp.Bounds()
		outCols := max(1, b.Dx()/rc.mode.pixCols(1))
		outRows := max(1, b.Dy()/rc.mode.pixRows(1))
		if rc.jobs > 1 {
			rendered = sparkline.RenderToImageJ(vp, outCols, outRows, rc.sparkMode, rc.jobs)
		} else {
			rendered = sparkline.RenderToImage(vp, outCols, outRows, rc.sparkMode)
		}
	default:
		rendered = vp
	}

	rb := ref.Bounds()
	vb := rendered.Bounds()

	if rb.Dx() != vb.Dx() || rb.Dy() != vb.Dy() {
		rendered = resizeRenderedImage(rendered, rb.Dx(), rb.Dy(), rc)
	}

	refLuma := metrics.LumaGrid(ref)
	rendLuma := metrics.LumaGrid(rendered)
	refSobel := metrics.SobelGrid(refLuma)
	rendSobel := metrics.SobelGrid(rendLuma)

	boundaryStep := metrics.GridK
	hasVerticalBoundaries := rc.mode.useQuad() || rc.mode.useSextant() || rc.mode.useGeomShape() || rc.mode.useSpark()
	score := metrics.RenderQuality{
		SSIM:       metrics.SSIMLuminance(ref, rendered),
		Blockiness: metrics.BlockinessFromGrids(refSobel, rendSobel, hasVerticalBoundaries, boundaryStep),
		EdgeCont:   metrics.EdgeContinuityFromGrids(refSobel, rendSobel),
	}
	return score
}
