package cmd

import (
	"image"
	"math"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/quadblock"
)

// qualityGridK is the subdivision factor for the quality metric reference grid.
// Each terminal cell is divided into qualityGridK × qualityGridK sub-pixels for
// SSIM, blockiness, and edge continuity. This puts all render modes (halfblock:
// 2×1 sub-pixels per cell, quad: 2×2 sub-pixels per cell) on a common footing
// by NN-upscaling the rendered output to match the reference resolution.
const qualityGridK = 4

// RenderQuality holds all perceptual quality metrics for one rendered frame.
type RenderQuality struct {
	SSIM       float64 // structural similarity [0,1]; 1 = perfect
	Blockiness float64 // 1 - excess block-boundary gradient [0,1]; 1 = no artefacts
	EdgeCont   float64 // weighted recall of reference edges [0,1]; 1 = all edges preserved
}

// lumaGrid converts img to a 2-D slice of BT.709 luminance values [0,1].
// Indices are [row][col] relative to img.Bounds().Min.
func lumaGrid(img image.Image) [][]float64 {
	b := img.Bounds()
	h, w := b.Dy(), b.Dx()
	g := make([][]float64, h)
	for y := range g {
		g[y] = make([]float64, w)
		for x := range g[y] {
			g[y][x] = luma(img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return g
}

// sobelGrid computes the Sobel gradient magnitude for every pixel of a luma
// grid. Border pixels are zero. Values are in [0, ~1] (max = 1.0 at a full
// black-to-white step with kernel weights 1,2,1).
func sobelGrid(g [][]float64) [][]float64 {
	h := len(g)
	s := make([][]float64, h)
	for y := range s {
		s[y] = make([]float64, len(g[y]))
	}
	if h < 3 {
		return s
	}
	for y := 1; y < h-1; y++ {
		w := len(g[y])
		for x := 1; x < w-1; x++ {
			gx := -g[y-1][x-1] - 2*g[y][x-1] - g[y+1][x-1] +
				g[y-1][x+1] + 2*g[y][x+1] + g[y+1][x+1]
			gy := -g[y-1][x-1] - 2*g[y-1][x] - g[y-1][x+1] +
				g[y+1][x-1] + 2*g[y+1][x] + g[y+1][x+1]
			s[y][x] = math.Sqrt(gx*gx+gy*gy) / 4.0
		}
	}
	return s
}

// blockinessFromGrids returns a score in [0,1] measuring how many artificial
// block edges the rendered image introduces at cell-grid positions that do not
// exist in the reference. 1.0 = no excess block boundary gradients.
//
// For halfblock (useQuad=false), only horizontal boundaries (every step rows)
// are checked — halfblock has no sub-cell vertical structure. For quad, both
// horizontal and vertical boundaries (every step pixels) are checked.
// step is the quality-grid boundary stride (qualityGridK for quality-grid refs,
// 2 for viewport-resolution refs).
func blockinessFromGrids(refS, rendS [][]float64, rc renderCfg, step int) float64 {
	h := len(refS)
	if h == 0 {
		return 1.0
	}
	w := len(refS[0])
	var totalExcess float64
	var n int

	// Vertical cell boundaries (quad only).
	if rc.useQuad {
		for x := step; x < w-1; x += step {
			for y := 1; y < h-1; y++ {
				if excess := rendS[y][x] - refS[y][x]; excess > 0 {
					totalExcess += excess
				}
				n++
			}
		}
	}

	// Horizontal cell boundaries — both modes.
	for y := step; y < h-1; y += step {
		for x := 1; x < w-1; x++ {
			if excess := rendS[y][x] - refS[y][x]; excess > 0 {
				totalExcess += excess
			}
			n++
		}
	}

	if n == 0 {
		return 1.0
	}
	// Normalise: a mean excess of maxExcess maps to score 0.
	// 0.15 ≈ 60 % of a full-contrast step at every checked position,
	// which is completely blocky in practice.
	const maxExcess = 0.15
	return math.Max(0, 1.0-totalExcess/float64(n)/maxExcess)
}

// edgeContinuityFromGrids returns the weighted recall of reference edges in
// the rendered image. Each reference pixel whose Sobel magnitude exceeds the
// threshold contributes weight proportional to its magnitude; credit is given
// if the rendered image has a comparable edge within a ±1 pixel neighbourhood.
// Returns 1.0 when the reference has no detectable edges.
func edgeContinuityFromGrids(refS, rendS [][]float64) float64 {
	h := len(refS)
	if h == 0 {
		return 1.0
	}
	const threshold = 0.05
	var weightedTotal, weightedHit float64
	for y := 1; y < h-1; y++ {
		w := len(refS[y])
		for x := 1; x < w-1; x++ {
			re := refS[y][x]
			if re < threshold {
				continue
			}
			weightedTotal += re
			// Best matching edge in ±1 neighbourhood of rendered image.
			near := math.Max(rendS[y][x],
				math.Max(rendS[y][x-1],
					math.Max(rendS[y][x+1],
						math.Max(rendS[y-1][x], rendS[y+1][x]))))
			weightedHit += re * math.Min(1.0, near/re)
		}
	}
	if weightedTotal == 0 {
		return 1.0
	}
	return weightedHit / weightedTotal
}

// computeQuality returns all perceptual quality metrics for a rendered frame.
//
//   - ref  — pyramid-downscale reference (at viewport or quality-grid resolution)
//   - vp   — NN-scaled viewport from rc.scaleToFit()
//   - rc   — active render configuration
//
// When ref is larger than vp (quality-grid resolution), vp is NN-upscaled to
// match ref before computing metrics. Blockiness boundary checks use the
// quality-grid step size (qualityGridK) derived from the upscale factor.
func computeQuality(ref, vp image.Image, rc renderCfg) RenderQuality {
	var rendered image.Image
	if rc.useQuad {
		rendered = quadblock.RenderToImage(vp, rc.quadOpts)
	} else {
		rendered = vp
	}

	rb := ref.Bounds()
	vb := vp.Bounds()

	// When ref is at quality-grid resolution, NN-upscale rendered to match.
	if rb.Dx() != vb.Dx() || rb.Dy() != vb.Dy() {
		rendered = halfblock.ScaleNN(rendered, rb.Dx(), rb.Dy())
	}

	refLuma := lumaGrid(ref)
	rendLuma := lumaGrid(rendered)
	refSobel := sobelGrid(refLuma)
	rendSobel := sobelGrid(rendLuma)

	// Boundary step for blockiness: quality grid positions are at multiples
	// of qualityGridK (since each terminal cell row/col is K sub-pixels).
	boundaryStep := qualityGridK
	score := RenderQuality{
		SSIM:       ssimLuminance(ref, rendered),
		Blockiness: blockinessFromGrids(refSobel, rendSobel, rc, boundaryStep),
		EdgeCont:   edgeContinuityFromGrids(refSobel, rendSobel),
	}
	return score
}
