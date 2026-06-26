// Package metrics provides perceptual quality metrics and image-processing
// primitives for comparing rendered terminal output against a reference.
//
// All functions are pure (depend only on image, image/color, math) — no I/O,
// no terminal interaction, no project-internal dependencies.
package metrics

import (
	"image"
	"image/color"
	"math"
)

// GridK is the subdivision factor for the quality metric reference grid.
// Each terminal cell is divided into GridK × GridK sub-pixels for SSIM,
// blockiness, and edge continuity. This puts all render modes (halfblock: 2×1
// sub-pixels per cell, quad: 2×2 sub-pixels per cell) on a common footing by
// NN-upscaling the rendered output to match the reference resolution.
const GridK = 4

// RenderQuality holds all perceptual quality metrics for one rendered frame.
type RenderQuality struct {
	SSIM       float64 // structural similarity [0,1]; 1 = perfect
	Blockiness float64 // 1 - excess block-boundary gradient [0,1]; 1 = no artefacts
	EdgeCont   float64 // weighted recall of reference edges [0,1]; 1 = all edges preserved
}

// luma returns the BT.709 luminance of c in [0, 1].
func luma(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535.0
}

// LumaGrid converts img to a 2-D slice of BT.709 luminance values [0,1].
// Indices are [row][col] relative to img.Bounds().Min.
func LumaGrid(img image.Image) [][]float64 {
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

// SobelGrid computes the Sobel gradient magnitude for every pixel of a luma
// grid. Border pixels are zero. Values are in [0, ~1] (max = 1.0 at a full
// black-to-white step with kernel weights 1,2,1).
func SobelGrid(g [][]float64) [][]float64 {
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

// BlockinessFromGrids returns a score in [0,1] measuring how many artificial
// block edges the rendered image introduces at cell-grid positions that do not
// exist in the reference. 1.0 = no excess block boundary gradients.
//
// For halfblock (useQuad=false), only horizontal boundaries (every step rows)
// are checked — halfblock has no sub-cell vertical structure. For quad, both
// horizontal and vertical boundaries (every step pixels) are checked.
// step is the quality-grid boundary stride (GridK for quality-grid refs,
// 2 for viewport-resolution refs).
func BlockinessFromGrids(refS, rendS [][]float64, useQuad bool, step int) float64 {
	h := len(refS)
	if h == 0 {
		return 1.0
	}
	w := len(refS[0])
	var totalExcess float64
	var n int

	if useQuad {
		for x := step; x < w-1; x += step {
			for y := 1; y < h-1; y++ {
				if excess := rendS[y][x] - refS[y][x]; excess > 0 {
					totalExcess += excess
				}
				n++
			}
		}
	}

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
	const maxExcess = 0.15
	return math.Max(0, 1.0-totalExcess/float64(n)/maxExcess)
}

// EdgeContinuityFromGrids returns the weighted recall of reference edges in
// the rendered image. Each reference pixel whose Sobel magnitude exceeds the
// threshold contributes weight proportional to its magnitude; credit is given
// if the rendered image has a comparable edge within a ±1 pixel neighbourhood.
// Returns 1.0 when the reference has no detectable edges.
func EdgeContinuityFromGrids(refS, rendS [][]float64) float64 {
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

// SSIMLuminance computes mean SSIM between a and b on the luminance channel
// using non-overlapping 8×8 windows (Wang et al. 2004). Returns 1.0 for
// identical images or images too small for even one window.
func SSIMLuminance(a, b image.Image) float64 {
	const (
		winSize = 8
		C1      = 0.01 * 0.01
		C2      = 0.03 * 0.03
	)
	ba := a.Bounds()
	w, h := ba.Dx(), ba.Dy()
	if w < winSize || h < winSize {
		return 1.0
	}
	var total float64
	var n int
	for y := 0; y+winSize <= h; y += winSize {
		for x := 0; x+winSize <= w; x += winSize {
			var sA, sB, sA2, sB2, sAB float64
			for dy := range winSize {
				for dx := range winSize {
					la := luma(a.At(ba.Min.X+x+dx, ba.Min.Y+y+dy))
					lb := luma(b.At(ba.Min.X+x+dx, ba.Min.Y+y+dy))
					sA += la
					sB += lb
					sA2 += la * la
					sB2 += lb * lb
					sAB += la * lb
				}
			}
			k := float64(winSize * winSize)
			muA, muB := sA/k, sB/k
			vA := sA2/k - muA*muA
			vB := sB2/k - muB*muB
			vAB := sAB/k - muA*muB
			l := (2*muA*muB + C1) / (muA*muA + muB*muB + C1)
			cs := (2*vAB + C2) / (vA + vB + C2)
			total += l * cs
			n++
		}
	}
	if n == 0 {
		return 1.0
	}
	return math.Max(0, math.Min(1, total/float64(n)))
}

// BoxDownscale returns a new image of size dstW×dstH by averaging (box-filter)
// the pixels of src that fall into each destination cell.
func BoxDownscale(src image.Image, dstW, dstH int) image.Image {
	sb := src.Bounds()
	srcW, srcH := sb.Dx(), sb.Dy()
	if srcW == 0 || srcH == 0 || dstW == 0 || dstH == 0 {
		return image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for dy := range dstH {
		y0 := dy * srcH / dstH
		y1 := max(y0+1, (dy+1)*srcH/dstH)
		for dx := range dstW {
			x0 := dx * srcW / dstW
			x1 := max(x0+1, (dx+1)*srcW/dstW)
			var rS, gS, bS, n float64
			for sy := y0; sy < y1 && sy < srcH; sy++ {
				for sx := x0; sx < x1 && sx < srcW; sx++ {
					r, g, b, _ := src.At(sb.Min.X+sx, sb.Min.Y+sy).RGBA()
					rS += float64(r)
					gS += float64(g)
					bS += float64(b)
					n++
				}
			}
			if n > 0 {
				dst.Set(dx, dy, color.RGBA{
					R: uint8(rS / n / 256),
					G: uint8(gS / n / 256),
					B: uint8(bS / n / 256),
					A: 255,
				})
			}
		}
	}
	return dst
}

// PyramidDownscale returns a high-quality downsample of src to dstW×dstH by
// repeatedly halving with box filter until within 2× of target, then a final
// box step.
func PyramidDownscale(src image.Image, dstW, dstH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	img := src
	for w > dstW*2 || h > dstH*2 {
		nw := max(w/2, dstW)
		nh := max(h/2, dstH)
		img = BoxDownscale(img, nw, nh)
		b = img.Bounds()
		w, h = b.Dx(), b.Dy()
	}
	if w != dstW || h != dstH {
		img = BoxDownscale(img, dstW, dstH)
	}
	return img
}

// QualityGridDims returns the quality-grid pixel dimensions for a rendered
// viewport of size vpW × vpH, using the given K factor.
// pixPerCol is the number of pixel columns per terminal cell (1 or 2).
// pixPerRow is the number of pixel rows per terminal cell (1 or 2).
func QualityGridDims(vpW, vpH int, pixPerCol, pixPerRow int, k int) (int, int) {
	cellW := vpW / pixPerCol
	cellH := vpH / pixPerRow
	return k * cellW, k * cellH
}

