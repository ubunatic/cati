package sparkline

import (
	"image"
	"image/color"
	"testing"
)

var sinkCellResult cellResult

func sparkGlyphBenchmarkImage(w, h int) image.Image {
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

func benchmarkFindBestCell(b *testing.B, mode Mode) {
	img := sparkGlyphBenchmarkImage(256, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for y := 0; y < 128; y += 8 {
			y1 := y + 7
			for x := 0; x < 256; x += 4 {
				x1 := x + 3
				sinkCellResult = FindBestCell(img, img.Bounds(), x, x1, y, y1, mode)
			}
		}
	}
}

func BenchmarkFindBestCellVertical(b *testing.B) {
	benchmarkFindBestCell(b, Vertical)
}

func BenchmarkFindBestCellQuad(b *testing.B) {
	benchmarkFindBestCell(b, Quad)
}
