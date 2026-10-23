package metrics

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestExtractLumaReferenceParity(t *testing.T) {
	r := image.Rect(0, 0, 13, 7)
	rgba, nrgba, gray := image.NewRGBA(r), image.NewNRGBA(r), image.NewGray(r)
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			alpha := []uint8{0, 1, 128, 254, 255}[(x+y)%5]
			nc := color.NRGBA{R: uint8(x*19 + 5), G: uint8(y*31 + 7), B: uint8(x*y + 11), A: alpha}
			nrgba.SetNRGBA(x, y, nc)
			rgba.SetRGBA(x, y, color.RGBA{
				R: uint8((uint16(nc.R) * uint16(alpha)) / 255),
				G: uint8((uint16(nc.G) * uint16(alpha)) / 255),
				B: uint8((uint16(nc.B) * uint16(alpha)) / 255), A: alpha,
			})
			gray.SetGray(x, y, color.Gray{Y: uint8(x*y + 3)})
		}
	}
	ycbcr := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)
	for i := range ycbcr.Y {
		ycbcr.Y[i] = uint8(i*17 + 3)
	}
	for i := range ycbcr.Cb {
		ycbcr.Cb[i], ycbcr.Cr[i] = uint8(i*23), uint8(i*29)
	}
	fixtures := map[string]image.Image{"RGBA": rgba, "NRGBA": nrgba, "Gray": gray, "YCbCr": ycbcr}
	for name, img := range fixtures {
		t.Run(name, func(t *testing.T) {
			got, w, h := extractLumaFlat(img, nil)
			want := extractLumaSimple(img, make([]float64, w*h), img.Bounds())
			if w != 13 || h != 7 || len(got) != len(want) {
				t.Fatalf("dimensions = %dx%d, len %d", w, h, len(got))
			}
			for i := range got {
				if d := math.Abs(got[i] - want[i]); d > 1e-12 {
					t.Fatalf("pixel %d: got %.12f, reference %.12f", i, got[i], want[i])
				}
			}
		})
	}
}
