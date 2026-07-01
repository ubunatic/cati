package halfblock

import (
	"image"
	"image/color"
	"io"
	"testing"
)

func benchmarkImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 7) % 256),
				G: uint8((y * 11) % 256),
				B: uint8((x + y*3) % 256),
				A: 255,
			})
		}
	}
	return img
}

func BenchmarkRenderSerial(b *testing.B) {
	img := benchmarkImage(256, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Render(io.Discard, img, img.Bounds().Dx(), Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderToImageSerial(b *testing.B) {
	img := benchmarkImage(256, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderToImage(img)
	}
}
