package sparkline

import (
	"image"
	"image/color"
	"math"
	"testing"

	"ubunatic.com/cati/v1/core"
)

// searchReference is a straightforward candidate-by-candidate pixel scan. It
// intentionally avoids bit masks, precomputed candidates, and typed Pix reads.
func searchReference(img image.Image, x0, y0, w, h int, candidates []candidate) cellResult {
	best := cellResult{Ch: ' ', Err: math.MaxFloat64}
	bestTransparent, bestSplit := math.MaxInt, math.MaxInt
	for _, cand := range candidates {
		var fg, bg acc
		var fn, bn, ft, bt int
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, a := img.At(x0+x, y0+y).RGBA()
				p := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
				isFG := cand.mask(x, y, w, h)
				if p.A == 0 {
					if isFG {
						ft++
					} else {
						bt++
					}
					continue
				}
				if isFG {
					fg.r += float64(p.R)
					fg.g += float64(p.G)
					fg.b += float64(p.B)
					fn++
				} else {
					bg.r += float64(p.R)
					bg.g += float64(p.G)
					bg.b += float64(p.B)
					bn++
				}
			}
		}
		fc, bc := avgColorOpaque(fg, fn), avgColorOpaque(bg, bn)
		if fc.A == 0 && bc.A != 0 && cand.ch != ' ' {
			continue
		}
		var err float64
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, a := img.At(x0+x, y0+y).RGBA()
				if a == 0 {
					continue
				}
				p := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
				if cand.mask(x, y, w, h) {
					err += sqDist(p, fc)
				} else {
					err += sqDist(p, bc)
				}
			}
		}
		if bc.A != 0 {
			err += float64(bt * 195075)
		}
		if fc.A != 0 || bc.A != 0 {
			err += float64(ft * 195075)
		}
		split := 0
		if fc.A != 0 && bc.A != 0 {
			split = 1
		}
		if err < best.Err || (err == best.Err && ft < bestTransparent) || (err == best.Err && ft == bestTransparent && split < bestSplit) {
			best = cellResult{Ch: cand.ch, FG: fc, BG: bc, Err: err, MaskFn: cand.mask, HasMask: true}
			bestTransparent, bestSplit = ft, split
		}
	}
	return best
}

func TestReferenceParity(t *testing.T) {
	candidates := candidatesForMode(Vertical)
	fixtures := map[string]image.Image{
		"rgba":                referenceImage(image.NewRGBA(image.Rect(0, 0, 4, 8))),
		"nrgba_partial_alpha": referenceImage(image.NewNRGBA(image.Rect(0, 0, 4, 8))),
		"gray":                referenceImage(image.NewGray(image.Rect(0, 0, 4, 8))),
		"odd_subimage":        referenceSubImage(),
	}
	for name, img := range fixtures {
		t.Run(name, func(t *testing.T) {
			b := img.Bounds()
			got := findBestCandidateFast(img, b.Min.X, b.Min.X+3, b.Min.Y, b.Min.Y+7, makeBitCandidates(candidates, 4, 8))
			want := searchReference(img, b.Min.X, b.Min.Y, 4, 8, candidates)
			if got.Ch != want.Ch || got.FG != want.FG || got.BG != want.BG || got.Err != want.Err {
				t.Fatalf("got %+v, reference %+v", got, want)
			}
		})
	}
}

func referenceImage(img image.Image) image.Image {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBA{R: uint8(x*41 + 3), G: uint8(y*27 + 7), B: uint8((x+y)*19 + 1), A: []uint8{0, 1, 128, 254, 255}[(x+y+40)%5]}
			switch m := img.(type) {
			case *image.RGBA:
				m.SetRGBA(x, y, core.NRGBAToRGBA(c.R, c.G, c.B, c.A))
			case *image.NRGBA:
				m.SetNRGBA(x, y, c)
			case *image.Gray:
				m.SetGray(x, y, color.Gray{Y: c.R})
			}
		}
	}
	return img
}

func referenceSubImage() image.Image {
	parent := referenceImage(image.NewNRGBA(image.Rect(-4, 3, 11, 17))).(*image.NRGBA)
	return parent.SubImage(image.Rect(1, 6, 6, 15))
}
