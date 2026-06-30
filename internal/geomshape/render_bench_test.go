package geomshape

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func benchImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: uint8(x ^ y), A: 255})
		}
	}
	return img
}

func benchmarkRender(b *testing.B, mode Mode) {
	img := benchImage()
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := Render(&buf, img, mode); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRenderToImage(b *testing.B, mode Mode) {
	img := benchImage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderToImage(img, mode)
	}
}

func BenchmarkRenderShape(b *testing.B)        { benchmarkRender(b, ModeShape) }
func BenchmarkRenderGeom(b *testing.B)         { benchmarkRender(b, ModeGeom) }
func BenchmarkRenderBest(b *testing.B)         { benchmarkRender(b, ModeBest) }
func BenchmarkRenderToImageShape(b *testing.B) { benchmarkRenderToImage(b, ModeShape) }
func BenchmarkRenderToImageGeom(b *testing.B)  { benchmarkRenderToImage(b, ModeGeom) }
func BenchmarkRenderToImageBest(b *testing.B)  { benchmarkRenderToImage(b, ModeBest) }
