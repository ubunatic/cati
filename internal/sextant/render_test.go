package sextant

import (
	"image"
	"image/color"
	"testing"
)

func patternImage(mask uint8, on, off color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for i := 0; i < 6; i++ {
		c := off
		if maskContains(mask, i) {
			c = on
		}
		img.Set(i%2, i/2, c)
	}
	return img
}

func transparentImage(w, h int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w, h))
}

func TestSextantMaskTable(t *testing.T) {
	tests := []struct {
		name string
		mask uint8
		want rune
	}{
		{"single top-left", sextantBit(1), '\U0001FB00'},
		{"top row", sextantBit(1) | sextantBit(2), '\U0001FB02'},
		{"middle pair", sextantBit(3) | sextantBit(4), '\U0001FB0B'},
		{"bottom pair", sextantBit(5) | sextantBit(6), '\U0001FB2D'},
		{"almost full", sextantBits("23456"), '\U0001FB3B'},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskName(tc.mask); got == "" && tc.mask != 0 {
				t.Fatalf("maskName(%06b) returned empty", tc.mask)
			}
			if got := sextantRuneByMask[tc.mask]; got != tc.want {
				t.Fatalf("sextantRuneByMask[%06b] = %q, want %q", tc.mask, got, tc.want)
			}
		})
	}
}

func TestSextantCandidateCount(t *testing.T) {
	if got := len(sextantMasks); got != 60 {
		t.Fatalf("len(sextantMasks) = %d, want 60", got)
	}
	// 60 native sextant glyphs + the two half-block column patterns (▌ ▐).
	if got := len(sextantRuneByMask); got != 62 {
		t.Fatalf("len(sextantRuneByMask) = %d, want 62", got)
	}
}

func TestSextantModeUsesDirectMask(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	cases := []struct {
		name string
		img  image.Image
		mask uint8
	}{
		{"empty", transparentImage(2, 3), 0},
		{"single bit", patternImage(sextantBit(1), on, off), sextantBit(1)},
		{"corner pair", patternImage(sextantBit(1)|sextantBit(6), on, off), sextantBit(1) | sextantBit(6)},
		{"full cell", patternImage(sextantBits("23456"), on, on), 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pixels := sampleBlock(tc.img, 0, 2, 0, 3)
			cell := chooseCell(pixels, ModeSextant)
			if cell.mask != tc.mask {
				t.Fatalf("chooseCell(2x3) mask = %06b, want %06b", cell.mask, tc.mask)
			}
		})
	}
}

// TestSextantNoNulGlyph guards the regression where the two pure-column masks
// (left 1·3·5, right 2·4·6) had no glyph and were emitted as rune(0) — a
// zero-width NUL that shifted the row and left the right edge unfilled. Every
// mask must resolve to either a printable rune or the explicit space cell.
func TestSextantNoNulGlyph(t *testing.T) {
	for m := 0; m < 64; m++ {
		cell, _ := scoreMask([6]color.RGBA{
			{255, 255, 255, 255}, {255, 255, 255, 255},
			{255, 255, 255, 255}, {255, 255, 255, 255},
			{255, 255, 255, 255}, {255, 255, 255, 255},
		}, uint8(m))
		if cell.ch == 0 && !cell.transparent {
			t.Fatalf("scoreMask(mask=%06b) produced rune(0) on an opaque cell", m)
		}
	}
	if got := sextantRuneByMask[leftColumnMask]; got != '▌' {
		t.Fatalf("leftColumnMask rune = %q, want ▌", got)
	}
	if got := sextantRuneByMask[rightColumnMask]; got != '▐' {
		t.Fatalf("rightColumnMask rune = %q, want ▐", got)
	}
}

func TestSextantModeStaysOnSupportedMasks(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	cases := []struct {
		name string
		img  image.Image
	}{
		{"single bit", patternImage(sextantBit(1), on, off)},
		{"full cell", patternImage(sextantBits("23456"), on, on)},
	}
	supported := make(map[uint8]struct{}, len(sextantMasks))
	for _, mask := range sextantMasks {
		supported[mask] = struct{}{}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pixels := sampleBlock(tc.img, 0, 2, 0, 3)
			cell := chooseCell(pixels, ModeSextant)
			if cell.mask == 0 && cell.hasBG {
				return
			}
			if _, ok := supported[cell.mask]; !ok {
				t.Fatalf("chooseCell mask = %06b, want supported sextant mask", cell.mask)
			}
		})
	}
}

func TestRenderToImageRoundTrip(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	src := patternImage(sextantBit(1)|sextantBit(6), on, off)
	got := RenderToImage(src, ModeSextant)
	gb := got.Bounds()
	if gb.Dx() != 2 || gb.Dy() != 3 {
		t.Fatalf("RenderToImage size = %dx%d, want 2x3", gb.Dx(), gb.Dy())
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			gotPixel := toRGBA(got.At(x, y))
			wantPixel := toRGBA(src.At(x, y))
			if gotPixel != wantPixel {
				t.Fatalf("pixel (%d,%d) = %#v, want %#v", x, y, gotPixel, wantPixel)
			}
		}
	}
}
