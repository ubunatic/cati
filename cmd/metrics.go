package cmd

import (
	"image"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sparkline"
)

// computeQuality returns all perceptual quality metrics for a rendered frame.
//
//   - ref  — pyramid-downscale reference (at viewport or quality-grid resolution)
//   - vp   — NN-scaled viewport from rc.scaleToFit()
//   - rc   — active render configuration
//
// The rendered terminal reconstruction is normalized to ref dimensions before
// scoring: smaller render grids are NN-upscaled, while denser render grids are
// pyramid-downscaled to the common quality grid.
func computeQuality(ref, vp image.Image, rc renderCfg) metrics.RenderQuality {
	var rendered image.Image
	switch {
	case rc.mode.useQuad():
		rendered = quadblock.RenderToImage(vp, rc.quadOpts)
	case rc.mode.useSpark():
		b := vp.Bounds()
		outCols := max(1, b.Dx()/rc.mode.pixCols(1))
		outRows := max(1, b.Dy()/rc.mode.pixRows(1))
		rendered = sparkline.RenderToImage(vp, outCols, outRows, rc.sparkMode)
	default:
		rendered = vp
	}

	rb := ref.Bounds()
	vb := rendered.Bounds()

	if rb.Dx() != vb.Dx() || rb.Dy() != vb.Dy() {
		if vb.Dx() > rb.Dx() || vb.Dy() > rb.Dy() {
			rendered = metrics.PyramidDownscale(rendered, rb.Dx(), rb.Dy())
		} else {
			rendered = halfblock.ScaleNN(rendered, rb.Dx(), rb.Dy())
		}
	}

	refLuma := metrics.LumaGrid(ref)
	rendLuma := metrics.LumaGrid(rendered)
	refSobel := metrics.SobelGrid(refLuma)
	rendSobel := metrics.SobelGrid(rendLuma)

	boundaryStep := metrics.GridK
	hasVerticalBoundaries := rc.mode.useQuad() || rc.mode.useSpark()
	score := metrics.RenderQuality{
		SSIM:       metrics.SSIMLuminance(ref, rendered),
		Blockiness: metrics.BlockinessFromGrids(refSobel, rendSobel, hasVerticalBoundaries, boundaryStep),
		EdgeCont:   metrics.EdgeContinuityFromGrids(refSobel, rendSobel),
	}
	return score
}
