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
		if err := Render(io.Discard, img, mode); err != nil {
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

func BenchmarkRenderSextant(b *testing.B) { benchmarkRender(b, ModeSextant) }
func BenchmarkRenderGeom(b *testing.B)    { benchmarkRender(b, ModeGeom) }
func BenchmarkRenderBest(b *testing.B)    { benchmarkRender(b, ModeBest) }

func BenchmarkRenderToImageSextant(b *testing.B) { benchmarkRenderToImage(b, ModeSextant) }
func BenchmarkRenderToImageGeom(b *testing.B)    { benchmarkRenderToImage(b, ModeGeom) }
func BenchmarkRenderToImageBest(b *testing.B)    { benchmarkRenderToImage(b, ModeBest) }
