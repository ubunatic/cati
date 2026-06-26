package pixelart

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

// ── Scale2x ────────────────────────────────────────────────────────────────────

func TestScale2x_Dims(t *testing.T) {
	img := solidImage(3, 2, rgba(255, 0, 0, 255))
	dst := Scale2x(img)
	b := dst.Bounds()
	if b.Dx() != 6 || b.Dy() != 4 {
		t.Errorf("Scale2x: got %dx%d, want 6x4", b.Dx(), b.Dy())
	}
}

func TestScale2x_Solid(t *testing.T) {
	// Solid 2x2 red → 4x4 all red.
	img := solidImage(2, 2, rgba(255, 0, 0, 255))
	dst := Scale2x(img)
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := dst.At(x, y).RGBA()
			if r == 0 || g != 0 || bl != 0 {
				t.Errorf("pixel (%d,%d) = (%d,%d,%d), want red", x, y, r/257, g/257, bl/257)
			}
		}
	}
}

func TestScale2x_Checkerboard(t *testing.T) {
	// 2x2 pattern: red, green, blue, white — pixels are all distinct so
	// Scale2x conditions never fire → pure NN 2x upscale. Check dims only.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, rgba(255, 0, 0, 255))
	img.Set(1, 0, rgba(0, 255, 0, 255))
	img.Set(0, 1, rgba(0, 0, 255, 255))
	img.Set(1, 1, rgba(255, 255, 255, 255))
	dst := Scale2x(img)
	b := dst.Bounds()
	if b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("dims: got %dx%d, want 4x4", b.Dx(), b.Dy())
	}
}

// ── Scale3x ────────────────────────────────────────────────────────────────────

func TestScale3x_Dims(t *testing.T) {
	img := solidImage(2, 2, rgba(255, 0, 0, 255))
	dst := Scale3x(img)
	b := dst.Bounds()
	if b.Dx() != 6 || b.Dy() != 6 {
		t.Errorf("Scale3x: got %dx%d, want 6x6", b.Dx(), b.Dy())
	}
}

func TestScale3x_Solid(t *testing.T) {
	img := solidImage(1, 1, rgba(0, 255, 0, 255))
	dst := Scale3x(img)
	b := dst.Bounds()
	if b.Dx() != 3 || b.Dy() != 3 {
		t.Errorf("dims: got %dx%d, want 3x3", b.Dx(), b.Dy())
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := dst.At(x, y).RGBA()
			if g == 0 || r != 0 || bl != 0 {
				t.Errorf("pixel (%d,%d) = (%d,%d,%d), want green", x, y, r/257, g/257, bl/257)
			}
		}
	}
}

// ── Sharpen ────────────────────────────────────────────────────────────────────

func TestSharpen_Dims(t *testing.T) {
	img := solidImage(5, 4, rgba(128, 128, 128, 255))
	dst := Sharpen(img, 0.5)
	b := dst.Bounds()
	if b.Dx() != 5 || b.Dy() != 4 {
		t.Errorf("Sharpen: got %dx%d, want 5x4", b.Dx(), b.Dy())
	}
}

func TestSharpen_ZeroAmount(t *testing.T) {
	// amount=0 → identity (within rounding).
	img := solidImage(3, 3, rgba(100, 150, 200, 255))
	dst := Sharpen(img, 0.0)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			r1, g1, b1, _ := img.At(x, y).RGBA()
			r2, g2, b2, _ := dst.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 {
				t.Errorf("amount=0: pixel (%d,%d) changed", x, y)
			}
		}
	}
}

// ── Convenience wrappers ───────────────────────────────────────────────────────

func TestSharpen05(t *testing.T) {
	img := solidImage(4, 4, rgba(128, 128, 128, 255))
	dst := Sharpen05(img)
	b := dst.Bounds()
	if b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("Sharpen05: got %dx%d, want 4x4", b.Dx(), b.Dy())
	}
}

func TestSharpen10(t *testing.T) {
	img := solidImage(4, 4, rgba(128, 128, 128, 255))
	dst := Sharpen10(img)
	b := dst.Bounds()
	if b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("Sharpen10: got %dx%d, want 4x4", b.Dx(), b.Dy())
	}
}
