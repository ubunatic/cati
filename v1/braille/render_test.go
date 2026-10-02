package braille

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func patternImage(mask uint8, fg, bg color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			idx := y*2 + x
			if maskContains(mask, idx) {
				img.SetRGBA(x, y, fg)
			} else {
				img.SetRGBA(x, y, bg)
			}
		}
	}
	return img
}

func TestGlyphs(t *testing.T) {
	glyphs := Glyphs()
	if len(glyphs) != 256 {
		t.Fatalf("Glyphs() returned %d runes, want 256", len(glyphs))
	}
	if glyphs[0] != '⠀' || glyphs[255] != '⣿' {
		t.Fatalf("Glyphs() first/last = %q/%q, want U+2800 / U+28FF", glyphs[0], glyphs[255])
	}
}

func TestRenderDotsMode(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	bg := color.RGBA{}
	// Dot 1 (top-left) mask: 0x01
	img := patternImage(0x01, red, bg)

	var buf bytes.Buffer
	opts := Options{Mode: ModeDots, NoLinePrefix: true}
	if err := Render(&buf, img, 1, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if out == "" {
		t.Fatal("Render output empty")
	}
}

func TestRenderForegroundBackgroundMode(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	// Dot 1 (top-left) mask: 0x01
	img := patternImage(0x01, red, blue)

	var buf bytes.Buffer
	opts := Options{Mode: ModeForegroundBackground, NoLinePrefix: true}
	if err := Render(&buf, img, 1, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if out == "" {
		t.Fatal("Render output empty")
	}
}

func TestRenderToImage(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	img := patternImage(0x01, red, blue)

	reconstructed := RenderToImage(img, ModeForegroundBackground)
	if reconstructed.Bounds() != img.Bounds() {
		t.Fatalf("Bounds mismatch: got %v, want %v", reconstructed.Bounds(), img.Bounds())
	}
}
