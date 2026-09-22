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
	"sync"
)

var lumaBufferPool = sync.Pool{
	New: func() any {
		b := make([]float64, 0, 1024*1024)
		return &b
	},
}

func getLumaBuffer(needed int) ([]float64, *[]float64) {
	ptr := lumaBufferPool.Get().(*[]float64)
	buf := *ptr
	if cap(buf) < needed {
		buf = make([]float64, needed)
		*ptr = buf
	} else {
		buf = buf[:needed]
	}
	return buf, ptr
}

func putLumaBuffer(ptr *[]float64, buf []float64) {
	*ptr = buf[:0]
	lumaBufferPool.Put(ptr)
}

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

// PSNR computes peak signal-to-noise ratio between two equally sized images.
// RGB and alpha channels are compared from the image.Color RGBA values after
// conversion to 8-bit channels; alpha is included as a fourth channel. The peak value is 255. Identical images return
// +Inf. Images with different dimensions or no pixels return 0.
func PSNR(a, b image.Image) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() || ab.Dx() <= 0 || ab.Dy() <= 0 {
		return 0
	}
	var sum float64
	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			ar, ag, abv, aa := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			br, bg, bbv, ba := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			for _, d := range [4]float64{
				float64(ar>>8) - float64(br>>8),
				float64(ag>>8) - float64(bg>>8),
				float64(abv>>8) - float64(bbv>>8),
				float64(aa>>8) - float64(ba>>8),
			} {
				sum += d * d
			}
		}
	}
	if sum == 0 {
		return math.Inf(1)
	}
	mse := sum / float64(ab.Dx()*ab.Dy()*4)
	return 10 * math.Log10((255*255)/mse)
}

// luma returns the BT.709 luminance of c in [0, 1].
func luma(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535.0
}

const (
	cr709 = 0.2126 / 255.0
	cg709 = 0.7152 / 255.0
	cb709 = 0.0722 / 255.0
	cgray = 1.0 / 255.0
)

var (
	lutR    [256]float64
	lutG    [256]float64
	lutB    [256]float64
	lutGray [256]float64
)

func init() {
	for i := 0; i < 256; i++ {
		lutR[i] = cr709 * float64(i)
		lutG[i] = cg709 * float64(i)
		lutB[i] = cb709 * float64(i)
		lutGray[i] = cgray * float64(i)
	}
}

// extractLumaFlat converts img to a contiguous 1-D slice of BT.709 luminance values [0,1]
// in row-major order (y*w + x). Fast paths are provided for common image types.
func extractLumaFlat(img image.Image, buf []float64) ([]float64, int, int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, 0, 0
	}
	needed := w * h
	if cap(buf) >= needed {
		buf = buf[:needed]
	} else {
		buf = make([]float64, needed)
	}

	switch m := img.(type) {
	case *image.RGBA:
		startOff := m.PixOffset(b.Min.X, b.Min.Y)
		if startOff == 0 && m.Stride == w*4 && len(m.Pix) >= w*h*4 {
			pix := m.Pix[:w*h*4]
			_ = pix[w*h*4-1]
			_ = buf[w*h-1]
			x, i := 0, 0
			for ; x+7 < w*h; x, i = x+8, i+32 {
				buf[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
				buf[x+1] = lutR[pix[i+4]] + lutG[pix[i+5]] + lutB[pix[i+6]]
				buf[x+2] = lutR[pix[i+8]] + lutG[pix[i+9]] + lutB[pix[i+10]]
				buf[x+3] = lutR[pix[i+12]] + lutG[pix[i+13]] + lutB[pix[i+14]]
				buf[x+4] = lutR[pix[i+16]] + lutG[pix[i+17]] + lutB[pix[i+18]]
				buf[x+5] = lutR[pix[i+20]] + lutG[pix[i+21]] + lutB[pix[i+22]]
				buf[x+6] = lutR[pix[i+24]] + lutG[pix[i+25]] + lutB[pix[i+26]]
				buf[x+7] = lutR[pix[i+28]] + lutG[pix[i+29]] + lutB[pix[i+30]]
			}
			for ; x < w*h; x, i = x+1, i+4 {
				buf[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
			}
		} else {
			for y := 0; y < h; y++ {
				rowOff := startOff + y*m.Stride
				pix := m.Pix[rowOff : rowOff+w*4]
				outRow := buf[y*w : (y+1)*w]
				x, i := 0, 0
				for ; x+7 < w; x, i = x+8, i+32 {
					outRow[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
					outRow[x+1] = lutR[pix[i+4]] + lutG[pix[i+5]] + lutB[pix[i+6]]
					outRow[x+2] = lutR[pix[i+8]] + lutG[pix[i+9]] + lutB[pix[i+10]]
					outRow[x+3] = lutR[pix[i+12]] + lutG[pix[i+13]] + lutB[pix[i+14]]
					outRow[x+4] = lutR[pix[i+16]] + lutG[pix[i+17]] + lutB[pix[i+18]]
					outRow[x+5] = lutR[pix[i+20]] + lutG[pix[i+21]] + lutB[pix[i+22]]
					outRow[x+6] = lutR[pix[i+24]] + lutG[pix[i+25]] + lutB[pix[i+26]]
					outRow[x+7] = lutR[pix[i+28]] + lutG[pix[i+29]] + lutB[pix[i+30]]
				}
				for ; x < w; x, i = x+1, i+4 {
					outRow[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
				}
			}
		}
	case *image.NRGBA:
		startOff := m.PixOffset(b.Min.X, b.Min.Y)
		if startOff == 0 && m.Stride == w*4 && len(m.Pix) >= w*h*4 {
			pix := m.Pix[:w*h*4]
			_ = pix[w*h*4-1]
			_ = buf[w*h-1]
			x, i := 0, 0
			for ; x+7 < w*h; x, i = x+8, i+32 {
				buf[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
				buf[x+1] = lutR[pix[i+4]] + lutG[pix[i+5]] + lutB[pix[i+6]]
				buf[x+2] = lutR[pix[i+8]] + lutG[pix[i+9]] + lutB[pix[i+10]]
				buf[x+3] = lutR[pix[i+12]] + lutG[pix[i+13]] + lutB[pix[i+14]]
				buf[x+4] = lutR[pix[i+16]] + lutG[pix[i+17]] + lutB[pix[i+18]]
				buf[x+5] = lutR[pix[i+20]] + lutG[pix[i+21]] + lutB[pix[i+22]]
				buf[x+6] = lutR[pix[i+24]] + lutG[pix[i+25]] + lutB[pix[i+26]]
				buf[x+7] = lutR[pix[i+28]] + lutG[pix[i+29]] + lutB[pix[i+30]]
			}
			for ; x < w*h; x, i = x+1, i+4 {
				buf[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
			}
		} else {
			for y := 0; y < h; y++ {
				rowOff := startOff + y*m.Stride
				pix := m.Pix[rowOff : rowOff+w*4]
				outRow := buf[y*w : (y+1)*w]
				x, i := 0, 0
				for ; x+7 < w; x, i = x+8, i+32 {
					outRow[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
					outRow[x+1] = lutR[pix[i+4]] + lutG[pix[i+5]] + lutB[pix[i+6]]
					outRow[x+2] = lutR[pix[i+8]] + lutG[pix[i+9]] + lutB[pix[i+10]]
					outRow[x+3] = lutR[pix[i+12]] + lutG[pix[i+13]] + lutB[pix[i+14]]
					outRow[x+4] = lutR[pix[i+16]] + lutG[pix[i+17]] + lutB[pix[i+18]]
					outRow[x+5] = lutR[pix[i+20]] + lutG[pix[i+21]] + lutB[pix[i+22]]
					outRow[x+6] = lutR[pix[i+24]] + lutG[pix[i+25]] + lutB[pix[i+26]]
					outRow[x+7] = lutR[pix[i+28]] + lutG[pix[i+29]] + lutB[pix[i+30]]
				}
				for ; x < w; x, i = x+1, i+4 {
					outRow[x] = lutR[pix[i]] + lutG[pix[i+1]] + lutB[pix[i+2]]
				}
			}
		}
	case *image.Gray:
		startOff := m.PixOffset(b.Min.X, b.Min.Y)
		for y := 0; y < h; y++ {
			rowOff := startOff + y*m.Stride
			pix := m.Pix[rowOff : rowOff+w]
			outRow := buf[y*w : (y+1)*w]
			for x := 0; x < w; x++ {
				outRow[x] = lutGray[pix[x]]
			}
		}
	case *image.YCbCr:
		yStart := m.YOffset(b.Min.X, b.Min.Y)
		cStart := m.COffset(b.Min.X, b.Min.Y)
		for y := 0; y < h; y++ {
			yOff := yStart + y*m.YStride
			cOff := cStart + (y>>1)*m.CStride
			outRow := buf[y*w : (y+1)*w]
			for x := 0; x < w; x++ {
				ci := x >> 1
				r, g, b := color.YCbCrToRGB(m.Y[yOff+x], m.Cb[cOff+ci], m.Cr[cOff+ci])
				outRow[x] = lutR[r] + lutG[g] + lutB[b]
			}
		}
	default:
		const inv65535 = 1.0 / 65535.0
		for y := 0; y < h; y++ {
			outRow := buf[y*w : (y+1)*w]
			for x := 0; x < w; x++ {
				r, g, b, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				outRow[x] = (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) * inv65535
			}
		}
	}
	return buf, w, h
}

// LumaGrid converts img to a 2-D slice of BT.709 luminance values [0,1].
// Indices are [row][col] relative to img.Bounds().Min.
func LumaGrid(img image.Image) [][]float64 {
	b := img.Bounds()
	h, w := b.Dy(), b.Dx()
	if h <= 0 || w <= 0 {
		return make([][]float64, h)
	}
	flat, _, _ := extractLumaFlat(img, nil)
	g := make([][]float64, h)
	for y := range g {
		g[y] = make([]float64, w)
		copy(g[y], flat[y*w:(y+1)*w])
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
		invK    = 1.0 / 64.0
	)
	ba, bb := a.Bounds(), b.Bounds()
	w, h := ba.Dx(), ba.Dy()
	if w < winSize || h < winSize || bb.Dx() != w || bb.Dy() != h {
		return 1.0
	}

	rawA, ptrA := getLumaBuffer(w * h)
	rawB, ptrB := getLumaBuffer(w * h)

	bufA, _, _ := extractLumaFlat(a, rawA)
	bufB, _, _ := extractLumaFlat(b, rawB)

	var total float64
	var n int

	for y := 0; y+winSize <= h; y += winSize {
		for x := 0; x+winSize <= w; x += winSize {
			off := y*w + x
			pA := bufA[off:]
			pB := bufB[off:]
			_ = pA[7*w+7]
			_ = pB[7*w+7]

			var sA0, sA1, sB0, sB1 float64
			var sA2_0, sA2_1, sB2_0, sB2_1, sAB0, sAB1 float64

			for dy := 0; dy < 8; dy++ {
				rowOff := dy * w
				a0, a1, a2, a3, a4, a5, a6, a7 := pA[rowOff], pA[rowOff+1], pA[rowOff+2], pA[rowOff+3], pA[rowOff+4], pA[rowOff+5], pA[rowOff+6], pA[rowOff+7]
				b0, b1, b2, b3, b4, b5, b6, b7 := pB[rowOff], pB[rowOff+1], pB[rowOff+2], pB[rowOff+3], pB[rowOff+4], pB[rowOff+5], pB[rowOff+6], pB[rowOff+7]

				sA0 += a0 + a1 + a2 + a3
				sA1 += a4 + a5 + a6 + a7
				sB0 += b0 + b1 + b2 + b3
				sB1 += b4 + b5 + b6 + b7

				sA2_0 += a0*a0 + a1*a1 + a2*a2 + a3*a3
				sA2_1 += a4*a4 + a5*a5 + a6*a6 + a7*a7
				sB2_0 += b0*b0 + b1*b1 + b2*b2 + b3*b3
				sB2_1 += b4*b4 + b5*b5 + b6*b6 + b7*b7
				sAB0 += a0*b0 + a1*b1 + a2*b2 + a3*b3
				sAB1 += a4*b4 + a5*b5 + a6*b6 + a7*b7
			}
			sA := sA0 + sA1
			sB := sB0 + sB1
			sA2 := sA2_0 + sA2_1
			sB2 := sB2_0 + sB2_1
			sAB := sAB0 + sAB1

			muA, muB := sA*invK, sB*invK
			vA := sA2*invK - muA*muA
			vB := sB2*invK - muB*muB
			vAB := sAB*invK - muA*muB
			l := (2*muA*muB + C1) / (muA*muA + muB*muB + C1)
			cs := (2*vAB + C2) / (vA + vB + C2)
			total += l * cs
			n++
		}
	}
	putLumaBuffer(ptrA, bufA)
	putLumaBuffer(ptrB, bufB)

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
