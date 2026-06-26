package cmd

import (
	"image"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
)

// computeQuality returns all perceptual quality metrics for a rendered frame.
//
//   - ref  — pyramid-downscale reference (at viewport or quality-grid resolution)
//   - vp   — NN-scaled viewport from rc.scaleToFit()
//   - rc   — active render configuration
//
// When ref is larger than vp (quality-grid resolution), vp is NN-upscaled to
// match ref before computing metrics. Blockiness boundary checks use the
// quality-grid step size (GridK) derived from the upscale factor.
func computeQuality(ref, vp image.Image, rc renderCfg) metrics.RenderQuality {
	var rendered image.Image
	if rc.useQuad {
		rendered = quadblock.RenderToImage(vp, rc.quadOpts)
	} else {
		rendered = vp
	}

	rb := ref.Bounds()
	vb := vp.Bounds()

	if rb.Dx() != vb.Dx() || rb.Dy() != vb.Dy() {
		rendered = halfblock.ScaleNN(rendered, rb.Dx(), rb.Dy())
	}

	refLuma := metrics.LumaGrid(ref)
	rendLuma := metrics.LumaGrid(rendered)
	refSobel := metrics.SobelGrid(refLuma)
	rendSobel := metrics.SobelGrid(rendLuma)

	boundaryStep := metrics.GridK
	score := metrics.RenderQuality{
		SSIM:       metrics.SSIMLuminance(ref, rendered),
		Blockiness: metrics.BlockinessFromGrids(refSobel, rendSobel, rc.useQuad, boundaryStep),
		EdgeCont:   metrics.EdgeContinuityFromGrids(refSobel, rendSobel),
	}
	return score
}
