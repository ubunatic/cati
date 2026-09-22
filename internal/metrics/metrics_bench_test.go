package metrics

import (
	"image"
	"image/color"
	"testing"
)

func BenchmarkSSIMLuminance(b *testing.B) {
	w, h := 320, 160
	imgA := image.NewRGBA(image.Rect(0, 0, w, h))
	imgB := image.NewRGBA(image.Rect(0, 0, w, h))

	// Fill with some pattern
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			imgA.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: uint8((x + y) % 256), A: 255})
			imgB.Set(x, y, color.RGBA{R: uint8((x + 10) % 256), G: uint8(y % 256), B: uint8((x + y + 5) % 256), A: 255})
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = SSIMLuminance(imgA, imgB)
	}
}
