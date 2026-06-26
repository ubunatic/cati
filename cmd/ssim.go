package cmd

import (
	"image"
	"image/color"
	"math"

	"codeberg.org/ubunatic/cati/internal/quadblock"
)

// luma returns the BT.709 luminance of c in [0, 1].
func luma(c color.Color) float64 {
	r, g, b, _ := c.RGBA() // 0–65535
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535.0
}

// ssimLuminance computes mean SSIM between a and b on the luminance channel
// using non-overlapping 8×8 windows (Wang et al. 2004). Returns 1.0 for
// identical images or images too small for even one window.
func ssimLuminance(a, b image.Image) float64 {
	const (
		winSize = 8
		// Constants normalised to [0,1] luminance range.
		C1 = 0.01 * 0.01
		C2 = 0.03 * 0.03
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

// boxDownscale returns a new image of size dstW×dstH by averaging (box-filter)
// the pixels of src that fall into each destination cell. This is a high-quality
// reference for downscaling, unlike nearest-neighbour.
func boxDownscale(src image.Image, dstW, dstH int) image.Image {
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

// pyramidDownscale returns a high-quality downsample of src to dstW×dstH by
// repeatedly halving with box filter until within 2× of target, then a final
// box step. This approximates Lanczos quality for large downscale ratios: it is
// sharper than a single-pass box (less blurry) yet free of NN aliasing. It is
// the correct reference for SSIM because:
//   - blurry renders can no longer score high by matching a blurry reference
//   - halfblock NN output differs from it → SSIM < 1.0 on textured images
func pyramidDownscale(src image.Image, dstW, dstH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	img := src
	for w > dstW*2 || h > dstH*2 {
		nw := max(w/2, dstW)
		nh := max(h/2, dstH)
		img = boxDownscale(img, nw, nh)
		b = img.Bounds()
		w, h = b.Dx(), b.Dy()
	}
	if w != dstW || h != dstH {
		img = boxDownscale(img, dstW, dstH)
	}
	return img
}

// blockMeanReconstruct replaces each blockW×blockH tile with its mean colour,
// modelling the worst-case colour quantisation of a block-based renderer.
func blockMeanReconstruct(img image.Image, blockW, blockH int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for by := 0; by < h; by += blockH {
		for bx := 0; bx < w; bx += blockW {
			var rS, gS, bS, n float64
			for dy := range blockH {
				for dx := range blockW {
					if bx+dx >= w || by+dy >= h {
						continue
					}
					r, g, bl, _ := img.At(b.Min.X+bx+dx, b.Min.Y+by+dy).RGBA()
					rS += float64(r)
					gS += float64(g)
					bS += float64(bl)
					n++
				}
			}
			if n == 0 {
				continue
			}
			mean := color.RGBA{
				R: uint8(rS / n / 256),
				G: uint8(gS / n / 256),
				B: uint8(bS / n / 256),
				A: 255,
			}
			for dy := range blockH {
				for dx := range blockW {
					if bx+dx < w && by+dy < h {
						dst.Set(bx+dx, by+dy, mean)
					}
				}
			}
		}
	}
	return dst
}

// buildRef returns a pyramid-downscale reference at the same pixel dimensions
// as the viewport. The reference is computed from the original image (not the
// already-NN-scaled viewport), so all rendering modes are measured against the
// same ideal. pyramidDownscale is sharper than a single-pass box filter (blurry
// renders cannot match it by blurring more) and different from NN (halfblock's
// output is not identical to the reference, so SSIM < 1.0 on textured images).
//
// The geometry mirrors buildViewport; state must already be clamped (call
// buildViewport first).
//
// NOTE: keep the geometry in sync with buildViewport if either changes.
func buildRef(orig image.Image, state viewState, termCols, termRows int, rc renderCfg) image.Image {
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig
	}
	pixCols := termCols
	if rc.useQuad {
		pixCols = termCols * 2
	}
	fitSrcW := srcW
	if rc.useQuad {
		fitSrcW = srcW * 2
	}
	fitW, fitH := fitPixelDims(fitSrcW, srcH, pixCols, termRows*2)
	scaledW := max(1, int(math.Round(float64(fitW)*state.zoom)))
	scaledH := max(1, int(math.Round(float64(fitH)*state.zoom)))

	viewW := min(pixCols, scaledW-state.panX)
	viewH := min(termRows*2, scaledH-state.panY)
	if viewW <= 0 || viewH <= 0 {
		return orig
	}
	// Map viewport pixel bounds back to orig coordinates.
	x0 := state.panX * srcW / scaledW
	y0 := state.panY * srcH / scaledH
	x1 := min((state.panX+viewW)*srcW/scaledW, srcW)
	y1 := min((state.panY+viewH)*srcH/scaledH, srcH)
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	region := cropImage(orig, x0, y0, x1-x0, y1-y0)
	return pyramidDownscale(region, viewW, viewH)
}

// renderSSIM returns SSIM(ref, rendered) where rendered is the faithful
// pixel reconstruction of the block-char output. For halfblock this is vp
// itself (lossless in colour). For quad modes it is RenderToImage — the exact
// fg/bg assignment each algorithm makes per 2×2 block — so different quad
// algorithms produce different SSIM scores. ref should be a box-filter
// downsample of the original source region.
func renderSSIM(ref, vp image.Image, rc renderCfg) float64 {
	if rc.useQuad {
		return ssimLuminance(ref, quadblock.RenderToImage(vp, rc.quadOpts))
	}
	return ssimLuminance(ref, vp)
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
