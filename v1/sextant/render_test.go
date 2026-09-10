package sextant

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"ubunatic.com/cati/v1/halfblock"
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
	if got := len(allMasks()); got != 64 {
		t.Fatalf("len(allMasks()) = %d, want 64", got)
	}
	seen := make(map[uint8]bool, 64)
	for i, mask := range allMasks() {
		if seen[mask] {
			t.Fatalf("allMasks() contains duplicate mask %06b", mask)
		}
		seen[mask] = true
		if mask != uint8(i) {
			t.Fatalf("allMasks()[%d] = %06b, want deterministic mask %06b", i, mask, i)
		}
	}
	for _, mask := range []uint8{0, 0b111111, leftColumnMask, rightColumnMask} {
		if _, ok := seen[mask]; !ok {
			t.Fatalf("allMasks() omits %06b", mask)
		}
	}
	if got := len(sextantRuneByMask); got != 64 {
		t.Fatalf("len(sextantRuneByMask) = %d, want 64", got)
	}
}

func TestSextantModeChoosesBestRepresentableMask(t *testing.T) {
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

func TestSextantModeUsesColumnAliases(t *testing.T) {
	left := color.RGBA{R: 255, A: 255}
	right := color.RGBA{B: 255, A: 255}
	pixels := [6]color.RGBA{left, right, left, right, left, right}
	cell := chooseCell(pixels, ModeSextant)
	if cell.mask != leftColumnMask || cell.ch != '▌' {
		t.Fatalf("vertical split = mask %06b rune %q, want mask %06b rune ▌", cell.mask, cell.ch, leftColumnMask)
	}
}

func TestSextantModeBeatsDirectMaskOnAntialiasing(t *testing.T) {
	pixels := [6]color.RGBA{
		{R: 255, A: 255}, {R: 210, G: 210, A: 255},
		{B: 220, A: 255}, {B: 255, A: 255},
		{G: 80, B: 80, A: 255}, {G: 100, B: 100, A: 255},
	}
	direct := directMask(pixels)
	_, directScore := scoreMask(pixels, direct)
	got := chooseCell(pixels, ModeSextant)
	_, bestScore := scoreMask(pixels, got.mask)
	if bestScore >= directScore {
		t.Fatalf("best mask %06b score=%d, direct mask %06b score=%d; want strict improvement", got.mask, bestScore, direct, directScore)
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

func TestSextantUniformCellUsesMinimumScore(t *testing.T) {
	pixels := [6]color.RGBA{{R: 20, G: 40, B: 60, A: 255}, {R: 20, G: 40, B: 60, A: 255}, {R: 20, G: 40, B: 60, A: 255}, {R: 20, G: 40, B: 60, A: 255}, {R: 20, G: 40, B: 60, A: 255}, {R: 20, G: 40, B: 60, A: 255}}
	got := chooseCell(pixels, ModeSextant)
	_, gotScore := scoreMask(pixels, got.mask)
	for _, mask := range allMasks() {
		_, score := scoreMask(pixels, mask)
		if gotScore > score {
			t.Fatalf("uniform cell mask %06b score=%d, but mask %06b scores %d", got.mask, gotScore, mask, score)
		}
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
	supported := make(map[uint8]struct{}, len(allMasks()))
	for _, mask := range allMasks() {
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

func TestSextantTransparentBottomEdgeDoesNotGrow(t *testing.T) {
	opaque := color.RGBA{R: 240, G: 80, A: 255}
	transparent := color.RGBA{}
	pixels := [6]color.RGBA{opaque, opaque, opaque, opaque, transparent, transparent}
	cell := chooseCell(pixels, ModeSextant)
	if emittedCoverage(cell, 4) || emittedCoverage(cell, 5) {
		t.Fatalf("selected cell %06b paints transparent bottom edge: %#v", cell.mask, cell)
	}
	if !emittedCoverage(cell, 0) || !emittedCoverage(cell, 1) || !emittedCoverage(cell, 2) || !emittedCoverage(cell, 3) {
		t.Fatalf("selected cell %06b failed to paint opaque top edge: %#v", cell.mask, cell)
	}
}

func TestSextantRenderToImagePreservesTransparentCoverage(t *testing.T) {
	opaque := color.RGBA{R: 240, G: 80, A: 255}
	src := patternImage(sextantBits("1234"), opaque, color.RGBA{})
	got := RenderToImage(src, ModeSextant)
	for i := 4; i < 6; i++ {
		p := toRGBA(got.At(i%2, i/2))
		if p.A != 0 {
			t.Fatalf("reconstructed transparent pixel %d = %#v, want transparent", i, p)
		}
	}
}

func TestSextantBackgroundEscapeCoversTransparentRegions(t *testing.T) {
	cell := cellResult{
		mask:  leftColumnMask,
		hasFG: true,
		hasBG: true,
	}
	for i := 0; i < 6; i++ {
		if !emittedCoverage(cell, i) {
			t.Fatalf("background cell does not cover region %d", i)
		}
	}
}

func TestSextantANSIAndImageCoverageAgree(t *testing.T) {
	opaque := color.RGBA{R: 240, G: 80, A: 255}
	src := patternImage(sextantBits("1234"), opaque, color.RGBA{})
	grid, err := RenderToGrid(src, 0, Options{Mode: ModeSextant})
	if err != nil {
		t.Fatalf("RenderToGrid: %v", err)
	}
	cell := chooseCell(sampleBlock(src, 0, 2, 0, 3), ModeSextant)
	if grid.Cells[0][0].Ch != cell.ch {
		t.Fatalf("grid rune %q, chooseCell rune %q", grid.Cells[0][0].Ch, cell.ch)
	}
	var ansi strings.Builder
	if err := Render(&ansi, src, 0, Options{Mode: ModeSextant, NoLinePrefix: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(ansi.String(), fgRGB(opaque)) || strings.Contains(ansi.String(), bgRGB(opaque)) {
		t.Fatalf("ANSI output does not match foreground-only coverage: %q", ansi.String())
	}
	got := RenderToImage(src, ModeSextant)
	for i := 0; i < 6; i++ {
		painted := toRGBA(got.At(i%2, i/2)).A != 0
		if painted != emittedCoverage(cell, i) {
			t.Fatalf("pixel %d painted=%v, emittedCoverage=%v; cell=%#v", i, painted, emittedCoverage(cell, i), cell)
		}
	}
}

func TestSextantSmallLogoWidthMatrixUsesSupportedGlyphs(t *testing.T) {
	img, err := halfblock.LoadImage("../../assets/cati_0001.png")
	if err != nil {
		t.Skipf("small logo fixture unavailable: %v", err)
	}
	for width := 8; width <= 20; width++ {
		grid, err := RenderToGrid(img, width, Options{Mode: ModeSextant})
		if err != nil {
			t.Fatalf("width %d: RenderToGrid: %v", width, err)
		}
		if grid.Width == 0 || grid.Height == 0 || grid.Width > width {
			t.Fatalf("width %d: grid=%dx%d, want non-empty width <= target", width, grid.Width, grid.Height)
		}
		for _, row := range grid.Cells {
			for _, cell := range row {
				if cell.Ch == 0 {
					t.Fatalf("width %d: emitted NUL glyph", width)
				}
			}
		}
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

func TestRender_NoLinePrefix(t *testing.T) {
	img := patternImage(0x3f, color.RGBA{R: 255, A: 255}, color.RGBA{})
	var sb strings.Builder
	if err := Render(&sb, img, 0, Options{Mode: ModeSextant, NoLinePrefix: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(sb.String(), ansiLinePrefix) || strings.Contains(sb.String(), "\r") {
		t.Fatalf("NoLinePrefix output contains line prefix: %q", sb.String())
	}
}
