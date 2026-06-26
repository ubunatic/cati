package cmd

import (
	"image"
	"math"

	"codeberg.org/ubunatic/cati/internal/quadblock"
)

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
// For halfblock (useQuad=false), only horizontal boundaries (every 2 rows) are
// checked — halfblock has no sub-cell vertical structure. For quad, both
// horizontal and vertical boundaries (every 2 pixels) are checked.
func blockinessFromGrids(refS, rendS [][]float64, rc renderCfg) float64 {
	h := len(refS)
	if h == 0 {
		return 1.0
	}
	w := len(refS[0])
	var totalExcess float64
	var n int

	// Vertical cell boundaries (quad only — every 2 pixel columns).
	if rc.useQuad {
		for x := 2; x < w-1; x += 2 {
			for y := 1; y < h-1; y++ {
				if excess := rendS[y][x] - refS[y][x]; excess > 0 {
					totalExcess += excess
				}
				n++
			}
		}
	}

	// Horizontal cell boundaries — both modes, every 2 pixel rows.
	for y := 2; y < h-1; y += 2 {
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
//   - ref  — pyramid-downscale reference at viewport pixel dimensions
//             (from buildRef or pyramidDownscale(rawFrame, vpW, vpH))
//   - vp   — NN-scaled viewport from rc.scaleToFit()
//   - rc   — active render configuration
func computeQuality(ref, vp image.Image, rc renderCfg) RenderQuality {
	var rendered image.Image
	if rc.useQuad {
		rendered = quadblock.RenderToImage(vp, rc.quadOpts)
	} else {
		rendered = vp
	}

	refLuma := lumaGrid(ref)
	rendLuma := lumaGrid(rendered)
	refSobel := sobelGrid(refLuma)
	rendSobel := sobelGrid(rendLuma)

	return RenderQuality{
		SSIM:       ssimLuminance(ref, rendered),
		Blockiness: blockinessFromGrids(refSobel, rendSobel, rc),
		EdgeCont:   edgeContinuityFromGrids(refSobel, rendSobel),
	}
}
