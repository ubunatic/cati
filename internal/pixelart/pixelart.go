// Package pixelart provides pixel-art-aware image scaling pre-passes.
//
// These functions are applied to the source image BEFORE ScaleToFit reduces it
// to terminal resolution. By upscaling first (or sharpening), subsequent
// nearest-neighbour downscaling produces fewer aliasing artefacts along edges.
package pixelart

import (
	"image"
	"image/color"
	"math"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func at(img image.Image, x, y int) color.RGBA {
	b := img.Bounds()
	x = max(b.Min.X, min(x, b.Max.X-1))
	y = max(b.Min.Y, min(y, b.Max.Y-1))
	r, g, bl, a := img.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)}
}

func set(dst *image.RGBA, x, y int, c color.RGBA) { dst.SetRGBA(x, y, c) }

func eqRGBA(a, b color.RGBA) bool { return a == b }

// ─── Scale2x (EPX) ────────────────────────────────────────────────────────────

// Scale2x doubles an image using the EPX / Scale2x algorithm (Kreed 1999).
//
// Each source pixel P with neighbours N (above), S (below), W (left), E (right)
// expands to a 2×2 output block. The four output pixels default to P; a corner
// is replaced by the neighbour colour when the two neighbours meeting at that
// corner agree with each other AND disagree with the opposite pair — indicating
// a sharp inward corner or diagonal edge:
//
//	TL ← N  if N==W && N!=E && W!=S
//	TR ← N  if N==E && N!=W && E!=S
//	BL ← S  if S==W && S!=N && W!=E
//	BR ← S  if S==E && S!=N && E!=W
//
// For photographs (continuous gradients) the equality conditions almost never
// fire; the function then degrades to a simple 2× nearest-neighbour upscale.
// It is most effective for high-contrast images with sharp 1-pixel-wide features
// (PCB traces, pixel art, thin line drawings).
func Scale2x(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w*2, h*2))

	for sy := b.Min.Y; sy < b.Max.Y; sy++ {
		dy := (sy - b.Min.Y) * 2
		for sx := b.Min.X; sx < b.Max.X; sx++ {
			dx := (sx - b.Min.X) * 2
			p := at(src, sx, sy)
			n := at(src, sx, sy-1)
			s := at(src, sx, sy+1)
			w_ := at(src, sx-1, sy)
			e := at(src, sx+1, sy)

			tl, tr, bl, br := p, p, p, p
			if eqRGBA(n, w_) && !eqRGBA(n, e) && !eqRGBA(w_, s) {
				tl = n
			}
			if eqRGBA(n, e) && !eqRGBA(n, w_) && !eqRGBA(e, s) {
				tr = n
			}
			if eqRGBA(s, w_) && !eqRGBA(s, n) && !eqRGBA(w_, e) {
				bl = s
			}
			if eqRGBA(s, e) && !eqRGBA(s, n) && !eqRGBA(e, w_) {
				br = s
			}
			set(dst, dx, dy, tl)
			set(dst, dx+1, dy, tr)
			set(dst, dx, dy+1, bl)
			set(dst, dx+1, dy+1, br)
		}
	}
	return dst
}

// ─── Scale3x (EPX-B / Scale3x) ────────────────────────────────────────────────

// Scale3x triples an image using the Scale3x algorithm. Each source pixel
// expands to a 3×3 block; the 8 border pixels of the block are conditionally
// replaced using the same edge-detection logic as Scale2x but extended to the
// 3×3 case. The centre pixel is always the source colour.
func Scale3x(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w*3, h*3))

	for sy := b.Min.Y; sy < b.Max.Y; sy++ {
		dy := (sy - b.Min.Y) * 3
		for sx := b.Min.X; sx < b.Max.X; sx++ {
			dx := (sx - b.Min.X) * 3

			p := at(src, sx, sy)
			n := at(src, sx, sy-1)
			s := at(src, sx, sy+1)
			ww := at(src, sx-1, sy)
			e := at(src, sx+1, sy)
			nw := at(src, sx-1, sy-1)
			ne := at(src, sx+1, sy-1)
			sw := at(src, sx-1, sy+1)
			se := at(src, sx+1, sy+1)

			// Condition flags.
			Nnw := eqRGBA(n, ww) && !eqRGBA(ww, s) && !eqRGBA(n, e)
			Nne := eqRGBA(n, e) && !eqRGBA(ww, n) && !eqRGBA(e, s)
			Ssw := eqRGBA(s, ww) && !eqRGBA(ww, n) && !eqRGBA(s, e)
			Sse := eqRGBA(s, e) && !eqRGBA(n, s) && !eqRGBA(ww, s)

			o := [9]color.RGBA{
				0: p, 1: p, 2: p,
				3: p, 4: p, 5: p,
				6: p, 7: p, 8: p,
			}
			if Nnw {
				o[0] = ww
			}
			if Nnw && !eqRGBA(n, ne) {
				o[1] = n
			}
			if Nne && !eqRGBA(n, nw) {
				o[1] = n
			}
			if Nne {
				o[2] = e
			}
			if Nnw && !eqRGBA(ww, sw) {
				o[3] = ww
			}
			if Ssw && !eqRGBA(ww, nw) {
				o[3] = ww
			}
			// o[4] = p (always)
			if Nne && !eqRGBA(e, se) {
				o[5] = e
			}
			if Sse && !eqRGBA(e, ne) {
				o[5] = e
			}
			if Ssw {
				o[6] = ww
			}
			if Ssw && !eqRGBA(s, se) {
				o[7] = s
			}
			if Sse && !eqRGBA(s, sw) {
				o[7] = s
			}
			if Sse {
				o[8] = e
			}

			for row := range 3 {
				for col := range 3 {
					set(dst, dx+col, dy+row, o[row*3+col])
				}
			}
			_ = nw
			_ = ne
			_ = sw
			_ = se
		}
	}
	return dst
}

// ─── Sharpen (unsharp mask) ───────────────────────────────────────────────────

// Sharpen applies a mild unsharp-mask to src. The sharpening radius is 1 pixel
// (3×3 kernel) and the strength is controlled by amount (0=no effect, 1=strong).
// A typical value is 0.5.
//
// At terminal scale every pixel represents a large source region, so details
// lost in the NN downscale appear at cell boundaries as soft gradients. Sharpening
// before downscaling moves those gradients closer to hard edges, giving the
// 2-colour cell encoder a cleaner split signal.
func Sharpen(src image.Image, amount float64) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			// 3×3 box-blur sample.
			var rB, gB, bB float64
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					c := at(src, x+dx, y+dy)
					rB += float64(c.R)
					gB += float64(c.G)
					bB += float64(c.B)
				}
			}
			rB /= 9; gB /= 9; bB /= 9

			c := at(src, x, y)
			rS := clamp(float64(c.R) + amount*(float64(c.R)-rB))
			gS := clamp(float64(c.G) + amount*(float64(c.G)-gB))
			bS := clamp(float64(c.B) + amount*(float64(c.B)-bB))
			set(dst, x-b.Min.X, y-b.Min.Y, color.RGBA{uint8(rS), uint8(gS), uint8(bS), c.A})
		}
	}
	return dst
}

func clamp(v float64) float64 { return math.Max(0, math.Min(255, v)) }

// ─── Convenience wrappers ─────────────────────────────────────────────────────

// Sharpen05 is a Sharpen with amount=0.5 — the recommended default.
func Sharpen05(src image.Image) image.Image { return Sharpen(src, 0.5) }

// Sharpen10 is a Sharpen with amount=1.0 — aggressive sharpening.
func Sharpen10(src image.Image) image.Image { return Sharpen(src, 1.0) }
