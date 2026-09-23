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

func BenchmarkFindBestCellHalfSplit(b *testing.B) {
	benchmarkFindBestCell(b, HalfSplit)
}

func BenchmarkFindBestCellSpark(b *testing.B) {
	benchmarkFindBestCell(b, Spark)
}

func BenchmarkFindBestCellQuad(b *testing.B) {
	benchmarkFindBestCell(b, Quad)
}

func BenchmarkFindBestCellSextant(b *testing.B) {
	benchmarkFindBestCell(b, Sextant)
}

func BenchmarkFindBestCellSixHalf(b *testing.B) {
	benchmarkFindBestCell(b, SixHalf)
}

func BenchmarkFindBestCellBest(b *testing.B) {
	benchmarkFindBestCell(b, Best)
}

func BenchmarkRenderCustomShapes(b *testing.B) {
	img := sparkGlyphBenchmarkImage(240, 144) // 40x24 cells of 6x6
	shapes := make([]Shape, 0, 90)
	for i := 0; i < 90; i++ {
		mask := make([]bool, 36)
		for j := 0; j < 36; j++ {
			mask[j] = (j+i)%3 == 0
		}
		shapes = append(shapes, Shape{
			Ch:     rune(0x2800 + i),
			Width:  6,
			Height: 6,
			Mask:   mask,
		})
	}
	opts := Options{
		CellW:  6,
		CellH:  6,
		Shapes: shapes,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := RenderToImageWithOptions(img, 40, 24, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}
