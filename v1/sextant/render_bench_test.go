package sextant

import (
	"image"
	"image/color"
	"io"
	"testing"
)

var sinkImage *image.RGBA

func benchmarkImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 19) % 256),
				G: uint8((y * 23) % 256),
				B: uint8((x*3 + y*7) % 256),
				A: 255,
			})
		}
	}
	return img
}

func benchmarkRender(b *testing.B, mode Mode) {
	img := benchmarkImage(256, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Render(io.Discard, img, 0, Options{Mode: mode}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRenderToImage(b *testing.B, mode Mode) {
	img := benchmarkImage(256, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkImage = RenderToImage(img, mode)
	}
}

func BenchmarkScoreMask(b *testing.B) {
	pixels := [6]color.RGBA{
		{R: 255, G: 0, B: 0, A: 255},
		{R: 200, G: 50, B: 10, A: 255},
		{R: 0, G: 255, B: 0, A: 255},
		{R: 10, G: 200, B: 50, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 50, G: 10, B: 200, A: 255},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scoreMask(pixels, uint8(i&63))
	}
}

func BenchmarkChooseCell(b *testing.B) {
	pixels := [6]color.RGBA{
		{R: 255, G: 0, B: 0, A: 255},
		{R: 200, G: 50, B: 10, A: 255},
		{R: 0, G: 255, B: 0, A: 255},
		{R: 10, G: 200, B: 50, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 50, G: 10, B: 200, A: 255},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chooseCell(pixels, ModeSextant)
	}
}

func BenchmarkRenderSextant(b *testing.B) { benchmarkRender(b, ModeSextant) }

func BenchmarkRenderToImageSextant(b *testing.B) { benchmarkRenderToImage(b, ModeSextant) }
