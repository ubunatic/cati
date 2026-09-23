package metrics

import (
	"image"
	"image/color"
	"math"
	"testing"

	"ubunatic.com/cati/v1/core"
)

// TestExtractLumaFastpathParity checks the typed fast paths against the
// simple At-based path for opaque images. Tolerance covers float rounding
// and the Gray/YCbCr conversion drift noted in extractLumaSimple.
func TestExtractLumaFastpathParity(t *testing.T) {
	r := image.Rect(0, 0, 13, 7)
	rgba, nrgba, gray := image.NewRGBA(r), image.NewNRGBA(r), image.NewGray(r)
	for y := range 7 {
		for x := range 13 {
			rgba.SetRGBA(x, y, color.RGBA{uint8(x * 19), uint8(y * 31), uint8(x * y), 255})
			nrgba.SetNRGBA(x, y, color.NRGBA{uint8(x * 7), uint8(y * 29), uint8(x + y), 255})
			gray.SetGray(x, y, color.Gray{uint8(x*y + 3)})
		}
	}
	defer func(v bool) { core.Fastpath = v }(core.Fastpath)
	for name, img := range map[string]image.Image{"RGBA": rgba, "NRGBA": nrgba, "Gray": gray} {
		core.Fastpath = true
		fast, _, _ := extractLumaFlat(img, nil)
		core.Fastpath = false
		simple, _, _ := extractLumaFlat(img, nil)
		for i := range fast {
			if d := math.Abs(fast[i] - simple[i]); d > 1e-3 {
				t.Fatalf("%s[%d]: fast %v simple %v", name, i, fast[i], simple[i])
			}
		}
	}
}
