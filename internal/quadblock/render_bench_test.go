package quadblock

import (
	"image"
	"image/color"
	"io"
	"testing"
)

func quadBenchmarkImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 13) % 256),
				G: uint8((y * 17) % 256),
				B: uint8((x*5 + y*9) % 256),
				A: 255,
			})
		}
	}
	return img
}

func BenchmarkRenderSerial(b *testing.B) {
	img := quadBenchmarkImage(256, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := RenderOpts(io.Discard, img, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderToImageSerial(b *testing.B) {
	img := quadBenchmarkImage(256, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderToImage(img, Options{})
	}
}
