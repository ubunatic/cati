package imgutil

import (
	"image"
	"image/color"
	"testing"
)

func rgba(r, g, b, a uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: a} }

func solidImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// ── FitDims ──────────────────────────────────────────────────────────────────

// realCells are the three production render-mode cell geometries:
// {cellW, cellH, aspectX}. All satisfy cellW/(aspectX*cellH) = 1/2, so they
// must agree on char-row count and bottom-row fill for any source and width.
var realCells = []struct {
	name                  string
	cellW, cellH, aspectX int
}{
	{"halfblock", 1, 2, 1},
	{"quad", 2, 2, 2},
	{"spark", 4, 8, 1},
}

// fitGeom reduces a FitDims result to resolution-independent geometry:
// the number of char rows and whether the bottom row is a half (▀) or full row.
func fitGeom(cellH, targetH, extH int) (rows int, halfBottom bool) {
	rows = (targetH + extH) / cellH
	return rows, extH > 0
}

// TestFitDimsUnifiedGeometry asserts that all three render modes produce the
// SAME half-cell geometry (rows + bottom fill) for the same source and width.
// This is the invariant that was violated when the height was floored to integer
// pixels before the half-cell snap (halfblock/quad rendered "too short").
func TestFitDimsUnifiedGeometry(t *testing.T) {
	type src struct {
		name       string
		srcW, srcH int
	}
	sources := []src{
		{"vacation", 1042, 1383},
		{"darth", 687, 1168},
		{"soldering", 640, 480},
		{"baby", 640, 360},
		{"cross", 20, 20},
		{"tall", 100, 333},
		{"wide", 333, 100},
	}
	for _, s := range sources {
		for _, cols := range []int{1, 2, 3, 5, 6, 7, 10, 12, 17, 24, 30, 41} {
			var wantRows int
			var wantHalf bool
			for i, c := range realCells {
				_, tH, extH := FitDims(s.srcW, s.srcH, c.cellW, c.cellH, c.aspectX, cols, 0)
				rows, half := fitGeom(c.cellH, tH, extH)
				if i == 0 {
					wantRows, wantHalf = rows, half
					continue
				}
				if rows != wantRows || half != wantHalf {
					t.Errorf("%s -w%d: mode %s geometry {rows=%d half=%v} != halfblock {rows=%d half=%v}",
						s.name, cols, c.name, rows, half, wantRows, wantHalf)
				}
			}
		}
	}
}

// TestFitDimsHalfCellInvariant asserts extH is always 0 or exactly CellH/2, so
// the transparent tail never exceeds half a char (TestGoldenTransparentBound).
func TestFitDimsHalfCellInvariant(t *testing.T) {
	for _, c := range realCells {
		for _, cols := range []int{1, 3, 6, 7, 13, 24, 31, 100} {
			for _, srcH := range []int{100, 333, 360, 480, 1168, 1383} {
				_, _, extH := FitDims(1000, srcH, c.cellW, c.cellH, c.aspectX, cols, 0)
				if extH != 0 && extH != c.cellH/2 {
					t.Errorf("%s srcH=%d -w%d: extH=%d, want 0 or %d",
						c.name, srcH, cols, extH, c.cellH/2)
				}
			}
		}
	}
}

func TestAlignCellSizeRoundsDownHeight(t *testing.T) {
	gotW, gotH := AlignCellSize(11, 13, 2, 3)
	if gotW != 10 || gotH != 12 {
		t.Fatalf("AlignCellSize = %dx%d, want 10x12", gotW, gotH)
	}
}

func TestAlignCellSizePromotesSubCellDimensions(t *testing.T) {
	gotW, gotH := AlignCellSize(4, 4, 4, 8)
	if gotW != 4 || gotH != 8 {
		t.Fatalf("AlignCellSize = %dx%d, want 4x8", gotW, gotH)
	}

	gotW, gotH = AlignCellSize(4, 4, 4, 24)
	if gotW != 4 || gotH != 24 {
		t.Fatalf("AlignCellSize = %dx%d, want 4x24", gotW, gotH)
	}
}

// ── FitPixelDims ───────────────────────────────────────────────────────────────

func TestFitPixelDims(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		maxW, maxH   int
		wantW, wantH int
	}{
		{"fits exactly", 100, 50, 100, 50, 100, 50},
		{"width constrained", 100, 50, 50, 100, 50, 25},
		{"height constrained", 100, 50, 200, 25, 50, 25},
		{"zero src dims", 0, 0, 80, 40, 80, 40},
		{"never upscale", 10, 10, 100, 100, 10, 10},
		{"square fit", 50, 50, 30, 40, 30, 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotW, gotH := FitPixelDims(tc.srcW, tc.srcH, tc.maxW, tc.maxH)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("FitPixelDims(%d,%d, %d,%d) = (%d,%d), want (%d,%d)",
					tc.srcW, tc.srcH, tc.maxW, tc.maxH, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

// ── AppendTransparentRows & FillTransparentRows ────────────────────────────────

func TestAppendTransparentRows(t *testing.T) {
	red := rgba(255, 0, 0, 255)
	green := rgba(0, 255, 0, 255)

	t.Run("standard origin bounds", func(t *testing.T) {
		src := image.NewRGBA(image.Rect(0, 0, 2, 2))
		src.Set(0, 0, red)
		src.Set(1, 0, green)
		src.Set(0, 1, green)
		src.Set(1, 1, red)

		addH := 2
		out := AppendTransparentRows(src, addH)

		wantBounds := image.Rect(0, 0, 2, 4)
		if out.Bounds() != wantBounds {
			t.Fatalf("bounds got %v, want %v", out.Bounds(), wantBounds)
		}

		// Verify original content rows
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				if got, want := out.At(x, y), src.At(x, y); got != want {
					t.Errorf("pixel at (%d,%d) got %v, want %v", x, y, got, want)
				}
			}
		}

		// Verify appended transparent rows
		transparent := color.RGBA{0, 0, 0, 0}
		for y := 2; y < 4; y++ {
			for x := 0; x < 2; x++ {
				r, g, b, a := out.At(x, y).RGBA()
				tr, tg, tb, ta := transparent.RGBA()
				if r != tr || g != tg || b != tb || a != ta {
					t.Errorf("pixel at (%d,%d) got RGBA(%d,%d,%d,%d), want transparent RGBA(0,0,0,0)", x, y, r, g, b, a)
				}
			}
		}
	})

	t.Run("non-zero origin bounds", func(t *testing.T) {
		src := image.NewRGBA(image.Rect(5, 10, 7, 12))
		src.Set(5, 10, red)
		src.Set(6, 10, green)
		src.Set(5, 11, green)
		src.Set(6, 11, red)

		addH := 3
		out := AppendTransparentRows(src, addH)

		wantBounds := image.Rect(0, 0, 2, 5)
		if out.Bounds() != wantBounds {
			t.Fatalf("bounds got %v, want %v", out.Bounds(), wantBounds)
		}

		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				if got, want := out.At(x, y), src.At(5+x, 10+y); got != want {
					t.Errorf("pixel at (%d,%d) got %v, want %v", x, y, got, want)
				}
			}
		}

		for y := 2; y < 5; y++ {
			for x := 0; x < 2; x++ {
				r, g, b, a := out.At(x, y).RGBA()
				if r != 0 || g != 0 || b != 0 || a != 0 {
					t.Errorf("pixel at (%d,%d) got RGBA(%d,%d,%d,%d), want transparent RGBA(0,0,0,0)", x, y, r, g, b, a)
				}
			}
		}
	})

	t.Run("zero added height", func(t *testing.T) {
		src := solidImage(3, 3, red)
		out := AppendTransparentRows(src, 0)
		if out.Bounds() != src.Bounds() {
			t.Fatalf("bounds got %v, want %v", out.Bounds(), src.Bounds())
		}
		for y := 0; y < 3; y++ {
			for x := 0; x < 3; x++ {
				if got, want := out.At(x, y), src.At(x, y); got != want {
					t.Errorf("pixel at (%d,%d) got %v, want %v", x, y, got, want)
				}
			}
		}
	})
}

func TestFillTransparentRows(t *testing.T) {
	blue := rgba(0, 0, 255, 255)
	trans := rgba(0, 0, 0, 0)

	t.Run("replaces transparent bottom rows", func(t *testing.T) {
		src := image.NewRGBA(image.Rect(0, 0, 2, 4))
		src.Set(0, 0, blue)
		src.Set(1, 0, blue)
		src.Set(0, 1, blue)
		src.Set(1, 1, blue)
		src.Set(0, 2, trans)
		src.Set(1, 2, trans)
		src.Set(0, 3, trans)
		src.Set(1, 3, trans)

		out := FillTransparentRows(src)
		if out == src {
			t.Fatalf("expected new image instance, got original")
		}

		for y := 0; y < 4; y++ {
			for x := 0; x < 2; x++ {
				r, g, b, a := out.At(x, y).RGBA()
				br, bg, bb, ba := blue.RGBA()
				if r != br || g != bg || b != bb || a != ba {
					t.Errorf("at (%d,%d) got RGBA(%d,%d,%d,%d), want blue", x, y, r, g, b, a)
				}
			}
		}
	})

	t.Run("no transparent tail returns original", func(t *testing.T) {
		src := solidImage(2, 2, blue)
		out := FillTransparentRows(src)
		if out != src {
			t.Errorf("expected original image returned unchanged when no transparent tail")
		}
	})

	t.Run("all transparent rows returns original", func(t *testing.T) {
		src := solidImage(2, 2, trans)
		out := FillTransparentRows(src)
		if out != src {
			t.Errorf("expected original image returned unchanged when all rows transparent")
		}
	})
}

func TestScaleNN_ResizingAndBounds(t *testing.T) {
	red := rgba(255, 0, 0, 255)
	green := rgba(0, 255, 0, 255)
	blue := rgba(0, 0, 255, 255)
	white := rgba(255, 255, 255, 255)

	// Helper to create a 2x2 image with distinctive colors in each quadrant
	create2x2 := func(minX, minY int) *image.RGBA {
		img := image.NewRGBA(image.Rect(minX, minY, minX+2, minY+2))
		img.Set(minX+0, minY+0, red)
		img.Set(minX+1, minY+0, green)
		img.Set(minX+0, minY+1, blue)
		img.Set(minX+1, minY+1, white)
		return img
	}

	t.Run("upscaling 2x2 to 4x4", func(t *testing.T) {
		src := create2x2(0, 0)
		dst := ScaleNN(src, 4, 4)

		b := dst.Bounds()
		if b.Dx() != 4 || b.Dy() != 4 {
			t.Fatalf("expected 4x4 bounds, got %dx%d", b.Dx(), b.Dy())
		}

		expected := [][]color.RGBA{
			{red, red, green, green},
			{red, red, green, green},
			{blue, blue, white, white},
			{blue, blue, white, white},
		}

		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				r, g, b, a := dst.At(x, y).RGBA()
				wantR, wantG, wantB, wantA := expected[y][x].RGBA()
				if r != wantR || g != wantG || b != wantB || a != wantA {
					t.Errorf("at (%d,%d) got RGBA(%d,%d,%d,%d), want RGBA(%d,%d,%d,%d)",
						x, y, r, g, b, a, wantR, wantG, wantB, wantA)
				}
			}
		}
	})

	t.Run("downscaling 4x4 to 2x2", func(t *testing.T) {
		src := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				var c color.RGBA
				switch {
				case x < 2 && y < 2:
					c = red
				case x >= 2 && y < 2:
					c = green
				case x < 2 && y >= 2:
					c = blue
				default:
					c = white
				}
				src.Set(x, y, c)
			}
		}

		dst := ScaleNN(src, 2, 2)
		b := dst.Bounds()
		if b.Dx() != 2 || b.Dy() != 2 {
			t.Fatalf("expected 2x2 bounds, got %dx%d", b.Dx(), b.Dy())
		}

		expected := [][]color.RGBA{
			{red, green},
			{blue, white},
		}

		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				r, g, b, a := dst.At(x, y).RGBA()
				wantR, wantG, wantB, wantA := expected[y][x].RGBA()
				if r != wantR || g != wantG || b != wantB || a != wantA {
					t.Errorf("at (%d,%d) got RGBA(%d,%d,%d,%d), want RGBA(%d,%d,%d,%d)",
						x, y, r, g, b, a, wantR, wantG, wantB, wantA)
				}
			}
		}
	})

	t.Run("source with non-zero origin bounds", func(t *testing.T) {
		src := create2x2(10, 20)
		dst := ScaleNN(src, 4, 4)

		b := dst.Bounds()
		if b.Min.X != 0 || b.Min.Y != 0 || b.Dx() != 4 || b.Dy() != 4 {
			t.Fatalf("expected dst bounds Rect(0,0,4,4), got %v", b)
		}

		expected := [][]color.RGBA{
			{red, red, green, green},
			{red, red, green, green},
			{blue, blue, white, white},
			{blue, blue, white, white},
		}

		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				r, g, b, a := dst.At(x, y).RGBA()
				wantR, wantG, wantB, wantA := expected[y][x].RGBA()
				if r != wantR || g != wantG || b != wantB || a != wantA {
					t.Errorf("at (%d,%d) got RGBA(%d,%d,%d,%d), want RGBA(%d,%d,%d,%d)",
						x, y, r, g, b, a, wantR, wantG, wantB, wantA)
				}
			}
		}
	})
}

func TestFitPixelDims_PreservesAspect(t *testing.T) {
	// For a 2:1 image, ratio should be approximately preserved.
	// Integer truncation can cause up to ~2% error; we allow ±3%.
	srcW, srcH := 100, 50
	for _, maxW := range []int{100, 60, 30} {
		for _, maxH := range []int{50, 30, 15} {
			w, h := FitPixelDims(srcW, srcH, maxW, maxH)
			if w <= 0 || h <= 0 {
				t.Errorf("dims must be positive: got %dx%d", w, h)
				continue
			}
			gotRatio := float64(w) / float64(h)
			wantRatio := float64(srcW) / float64(srcH)
			if gotRatio > wantRatio*1.03 || gotRatio < wantRatio*0.97 {
				t.Errorf("aspect ratio not preserved for max=%dx%d: src %d:%d (%.2f), got %d:%d (%.2f)",
					maxW, maxH, srcW, srcH, wantRatio, w, h, gotRatio)
			}
		}
	}
}

// ── CropImage ──────────────────────────────────────────────────────────────────

func TestCropImage_Dims(t *testing.T) {
	img := solidImage(20, 16, rgba(255, 0, 0, 255))
	cropped := CropImage(img, 2, 2, 8, 6)
	b := cropped.Bounds()
	if b.Dx() != 8 || b.Dy() != 6 {
		t.Errorf("dims: got %dx%d, want 8x6", b.Dx(), b.Dy())
	}
}

func TestCropImage_Pixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, rgba(uint8(x*64), uint8(y*64), 128, 255))
		}
	}
	cropped := CropImage(img, 1, 1, 2, 2)
	bc := cropped.Bounds()
	// SubImage preserves bounds; access relative to Min.
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			got := cropped.At(bc.Min.X+dx, bc.Min.Y+dy)
			want := img.At(1+dx, 1+dy)
			r1, g1, b1, _ := got.RGBA()
			r2, g2, b2, _ := want.RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 {
				t.Errorf("crop[%d,%d] got (%d,%d,%d), want (%d,%d,%d)",
					dx, dy, r1/257, g1/257, b1/257, r2/257, g2/257, b2/257)
			}
		}
	}
}

func TestCropImage_SubImage(t *testing.T) {
	// *image.RGBA supports SubImage — verify zero-copy path doesn't crash.
	img := solidImage(10, 10, rgba(255, 0, 0, 255))
	cropped := CropImage(img, 0, 0, 10, 10)
	b := cropped.Bounds()
	if b.Dx() != 10 || b.Dy() != 10 {
		t.Errorf("full crop dims: got %dx%d", b.Dx(), b.Dy())
	}
}

// ── ScaleNN ──────────────────────────────────────────────────────────────────

func TestScaleNN_GuardClauses(t *testing.T) {
	emptyImg := image.NewRGBA(image.Rect(0, 0, 0, 0))
	if got := ScaleNN(emptyImg, 10, 10); got != emptyImg {
		t.Errorf("ScaleNN with empty image did not return original image pointer")
	}

	validImg := solidImage(10, 10, rgba(255, 0, 0, 255))
	tests := []struct {
		name string
		w, h int
	}{
		{"zero width", 0, 10},
		{"zero height", 10, 0},
		{"negative width", -1, 10},
		{"negative height", 10, -1},
		{"identical dimensions", 10, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScaleNN(validImg, tc.w, tc.h); got != validImg {
				t.Errorf("ScaleNN(%d, %d) did not return original image pointer", tc.w, tc.h)
			}
		})
	}
}
