package braille

import (
	"image"
	"image/color"
	"io"
	"testing"
)

func BenchmarkRender(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 80, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 80; x++ {
			if (x+y)%3 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
			}
		}
	}

	opts := Options{Mode: ModeForegroundBackground, NoLinePrefix: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Render(io.Discard, img, 40, opts)
	}
}
