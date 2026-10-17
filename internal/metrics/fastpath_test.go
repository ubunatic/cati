package metrics

import (
	"image"
	"image/color"
	"math"
	"testing"

	"ubunatic.com/cati/v1/core"
)

// TestExtractLumaFastpathParity checks the typed fast paths against the
// simple At-based path across image types and alpha transparency.
func TestExtractLumaFastpathParity(t *testing.T) {
	r := image.Rect(0, 0, 13, 7)
	rgba, nrgba, gray := image.NewRGBA(r), image.NewNRGBA(r), image.NewGray(r)
	ycbcr := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)

	for y := range 7 {
		for x := range 13 {
			rgba.SetRGBA(x, y, color.RGBA{uint8(x * 19), uint8(y * 31), uint8(x * y), uint8((x + y*3) * 17)})
			nrgba.SetNRGBA(x, y, color.NRGBA{uint8(x * 7), uint8(y * 29), uint8(x + y), uint8((x*11 + y*13) % 256)})
			gray.SetGray(x, y, color.Gray{uint8(x*y + 3)})
			ycbcr.Y[ycbcr.YOffset(x, y)] = uint8((x*17 + y*19) % 256)
			ycbcr.Cb[ycbcr.COffset(x, y)] = uint8((x * 23) % 256)
			ycbcr.Cr[ycbcr.COffset(x, y)] = uint8((y * 29) % 256)
		}
	}
	defer func(v bool) { core.Fastpath = v }(core.Fastpath)
	images := map[string]image.Image{
		"RGBA":  rgba,
		"NRGBA": nrgba,
		"Gray":  gray,
		"YCbCr": ycbcr,
	}
	for name, img := range images {
		core.Fastpath = true
		fast, _, _ := extractLumaFlat(img, nil)
		core.Fastpath = false
		simple, _, _ := extractLumaFlat(img, nil)
		for i := range fast {
			if d := math.Abs(fast[i] - simple[i]); d > 1e-12 {
				t.Fatalf("%s[%d]: fast %v simple %v (diff %e > 1e-12)", name, i, fast[i], simple[i], d)
			}
		}
	}
}
