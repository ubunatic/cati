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
	name                   string
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

// ── FitPixelDims ───────────────────────────────────────────────────────────────

func TestFitPixelDims(t *testing.T) {
	tests := []struct {
		name          string
		srcW, srcH    int
		maxW, maxH    int
		wantW, wantH  int
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
