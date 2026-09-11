package sparkline

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestModeName(t *testing.T) {
	tests := []struct {
		m    Mode
		want string
	}{
		{Vertical, "spark/vert"},
		{HalfSplit, "half/split"},
		{Spark, "spark"},
		{Quad, "spark+quad"},
		{Sextant, "spark/sextant"},
		{SixHalf, "six+half"},
		{Best, "spark+six"},
		{Mode(99), "spark/vert"},
	}
	for _, tc := range tests {
		if got := ModeName(tc.m); got != tc.want {
			t.Errorf("ModeName(%d) = %q, want %q", tc.m, got, tc.want)
		}
	}
}

func TestModes(t *testing.T) {
	ms := Modes()
	if len(ms) != 7 {
		t.Fatalf("Modes() returned %d entries, want 7", len(ms))
	}
	want := []Mode{Vertical, HalfSplit, Spark, Quad, Sextant, SixHalf, Best}
	for i, m := range want {
		if ms[i] != m {
			t.Errorf("Modes()[%d] = %v, want %v (all: %v)", i, ms[i], m, ms)
		}
	}
}

func hasCandidate(candidates []candidate, ch rune) bool {
	for _, cand := range candidates {
		if cand.ch == ch {
			return true
		}
	}
	return false
}

func TestCustomShapesDriveANSIAndReconstruction(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	on := color.RGBA{R: 240, G: 20, B: 10, A: 255}
	off := color.RGBA{B: 220, A: 255}
	img.Set(0, 0, on)
	img.Set(1, 0, off)
	img.Set(0, 1, off)
	img.Set(1, 1, off)
	opts := Options{CellW: 2, CellH: 2, AspectX: 2, Shapes: []Shape{
		{Ch: ' ', Width: 2, Height: 2, Mask: []bool{false, false, false, false}},
		{Ch: '▘', Width: 2, Height: 2, Mask: []bool{true, false, false, false}},
		{Ch: '█', Width: 2, Height: 2, Mask: []bool{true, true, true, true}},
	}}
	var out strings.Builder
	if err := Render(&out, img, 1, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "▘") {
		t.Fatalf("custom render = %q, want quadrant glyph", out.String())
	}
	reconstructed, err := RenderToImageWithOptions(img, 1, 1, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(reconstructed.At(0, 0)).(color.RGBA); got.R <= got.B {
		t.Fatalf("foreground reconstruction = %#v", got)
	}
}

func TestCustomShapesRejectMalformedMasks(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	_, err := RenderToGrid(img, 1, Options{CellW: 2, CellH: 2, Shapes: []Shape{{Ch: 'x', Width: 2, Height: 2, Mask: []bool{true}}}})
	if err == nil {
		t.Fatal("malformed custom mask succeeded")
	}
}

func TestCustomReconstructionCoversNonDivisibleWidth(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 5, 2))
	red := color.RGBA{R: 255, A: 255}
	for y := range 2 {
		for x := range 5 {
			img.Set(x, y, red)
		}
	}
	opts := Options{CellW: 2, CellH: 2, Shapes: []Shape{{Ch: '█', Width: 1, Height: 1, Mask: []bool{true}}}}
	got, err := RenderToImageWithOptions(img, 3, 1, opts)
	if err != nil {
		t.Fatal(err)
	}
	if edge := color.RGBAModel.Convert(got.At(4, 1)).(color.RGBA); edge.A != 255 || edge.R != 255 {
		t.Fatalf("right edge was not reconstructed: %#v", edge)
	}
}

func TestCandidateTables(t *testing.T) {
	if got := len(halfSplitCandidates); got != 6 {
		t.Fatalf("halfSplitCandidates = %d entries, want 6", got)
	}
	for _, ch := range []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'} {
		if !hasCandidate(sparkCandidates, ch) {
			t.Fatalf("sparkCandidates missing horizontal fill %q", ch)
		}
	}
	for _, ch := range []rune{'▘', '▝', '▖', '▗', '▚', '▞', '▛', '▜', '▙', '▟'} {
		if !hasCandidate(sparkQuadCandidates, ch) {
			t.Fatalf("sparkQuadCandidates missing quad glyph %q", ch)
		}
	}
	if !hasCandidate(sixHalfCandidates, '▀') || !hasCandidate(sixHalfCandidates, '▄') {
		t.Fatalf("sixHalfCandidates missing half-block split glyphs")
	}
	if len(bestCandidates) <= len(sparkCandidates) || len(bestCandidates) <= len(sextantCandidates) {
		t.Fatalf("bestCandidates = %d, want union larger than spark=%d and sextant=%d", len(bestCandidates), len(sparkCandidates), len(sextantCandidates))
	}
}

func TestSextantCandidateTable(t *testing.T) {
	if got := len(sextantCandidates); got != 62 {
		t.Fatalf("sextantCandidates = %d entries, want 62", got)
	}
	if !hasCandidate(sextantCandidates, '▌') || !hasCandidate(sextantCandidates, '▐') {
		t.Fatalf("sextantCandidates missing column fallback glyphs")
	}
}

func TestCycle(t *testing.T) {
	if got := Cycle(Vertical); got != HalfSplit {
		t.Errorf("Cycle(Vertical) = %d, want HalfSplit", got)
	}
	if got := Cycle(Quad); got != Sextant {
		t.Errorf("Cycle(Quad) = %d, want Sextant", got)
	}
	if got := Cycle(Best); got != Vertical {
		t.Errorf("Cycle(Best) = %d, want Vertical", got)
	}
	if got := CyclePrev(Vertical); got != Best {
		t.Errorf("CyclePrev(Vertical) = %d, want Best", got)
	}
	if got := CyclePrev(Quad); got != Spark {
		t.Errorf("CyclePrev(Quad) = %d, want Spark", got)
	}
	if got := Cycle(Mode(99)); got != Vertical {
		t.Errorf("Cycle(99) = %d, want Vertical", got)
	}
}

func TestCharVertical(t *testing.T) {
	tests := []struct {
		v    float64
		want rune
	}{
		{0, '\u2581'},
		{0.124, '\u2581'},
		{0.125, '\u2582'},
		{0.25, '\u2583'},
		{0.375, '\u2584'},
		{0.5, '\u2585'},
		{0.625, '\u2586'},
		{0.75, '\u2587'},
		{0.875, '\u2588'},
		{1.0, '\u2588'},
	}
	for _, tc := range tests {
		ch, swap := Char(Vertical, tc.v)
		if ch != tc.want {
			t.Errorf("Char(Vertical, %v) = %c, want %c", tc.v, ch, tc.want)
		}
		if swap {
			t.Errorf("Char(Vertical, %v): swapFgBg = true, want false", tc.v)
		}
	}
}

func TestCharEdgeCases(t *testing.T) {
	// Negative value treated as 0
	ch, swap := Char(Vertical, -0.5)
	if ch != '\u2581' {
		t.Errorf("Char(Vertical, -0.5) = %c, want ▁", ch)
	}
	_ = swap

	// Very large value treated as 1
	ch, _ = Char(Vertical, 2.0)
	if ch != '\u2588' {
		t.Errorf("Char(Vertical, 2.0) = %c, want █", ch)
	}
}

func TestString(t *testing.T) {
	values := []float64{0, 0.25, 0.5, 0.75, 1.0}
	s := String(Vertical, values)
	expected := "\u2581\u2583\u2585\u2587\u2588"
	if s != expected {
		t.Errorf("String(Vertical) = %q, want %q", s, expected)
	}
}

func TestStringEmpty(t *testing.T) {
	s := String(Vertical, nil)
	if s != "" {
		t.Errorf("String(nil) = %q, want empty", s)
	}
}

func TestScaleToFit(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))

	// Scale down to fit 40x20
	scaled := ScaleToFit(img, 40, 20)
	b := scaled.Bounds()
	if b.Dx() > 40 || b.Dy() > 20 {
		t.Errorf("ScaleToFit(100x50, 40x20) = %dx%d, want ≤40x20", b.Dx(), b.Dy())
	}

	// Scale already-fitting image
	scaled = ScaleToFit(img, 200, 100)
	b = scaled.Bounds()
	if b.Dx() != 100 || b.Dy() != 50 {
		t.Errorf("ScaleToFit(100x50, 200x100) = %dx%d, want 100x50", b.Dx(), b.Dy())
	}

	// Empty image
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	scaled = ScaleToFit(empty, 10, 10)
	if scaled.Bounds().Dx() != 0 || scaled.Bounds().Dy() != 0 {
		t.Errorf("ScaleToFit(empty) should be empty")
	}
}

func TestRenderOptsOutput(t *testing.T) {
	// Create a test image: 16x8 pixels → 2×1 terminal cells
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 32), B: 128, A: 255})
		}
	}

	var buf strings.Builder
	err := RenderOpts(&buf, img, 2, 1, Vertical)
	if err != nil {
		t.Fatalf("RenderOpts returned error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("RenderOpts produced empty output")
	}

	// Should contain ANSI escape sequences (foreground colors)
	if !strings.Contains(output, "\x1b[38;2;") {
		t.Error("RenderOpts output missing ANSI foreground color escapes")
	}

	// Should contain ANSI background color escapes
	if !strings.Contains(output, "\x1b[48;2;") {
		t.Error("RenderOpts output missing ANSI background color escapes")
	}

	// Should contain the reset sequence
	if !strings.Contains(output, "\x1b[0m") {
		t.Error("RenderOpts output missing ANSI reset")
	}

	// Should contain block characters (only need to check for non-space, non-newline runes)
	hasBlock := false
	for _, r := range output {
		if r >= '\u2581' && r <= '\u2588' {
			hasBlock = true
			break
		}
	}
	if !hasBlock {
		t.Error("RenderOpts output missing block characters")
	}
}

func TestRenderOptsHonorsNonZeroBounds(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			src.Set(x, y, color.RGBA{R: uint8(20 * x), G: uint8(30 * y), B: 90, A: 255})
		}
	}

	crop := src.SubImage(image.Rect(4, 0, 8, 8))
	normalized := image.NewRGBA(image.Rect(0, 0, 4, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			normalized.Set(x, y, src.At(4+x, y))
		}
	}

	var got, want strings.Builder
	if err := RenderOpts(&got, crop, 1, 1, Vertical); err != nil {
		t.Fatalf("RenderOpts(crop): %v", err)
	}
	if err := RenderOpts(&want, normalized, 1, 1, Vertical); err != nil {
		t.Fatalf("RenderOpts(normalized): %v", err)
	}
	if got.String() != want.String() {
		t.Fatalf("RenderOpts(non-zero bounds) differed from zero-origin copy\ngot:  %q\nwant: %q", got.String(), want.String())
	}
}

func TestRenderToImageHonorsNonZeroBounds(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			src.Set(x, y, color.RGBA{R: uint8(20 * x), G: uint8(30 * y), B: 90, A: 255})
		}
	}

	crop := src.SubImage(image.Rect(4, 0, 8, 8))
	normalized := image.NewRGBA(image.Rect(0, 0, 4, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			normalized.Set(x, y, src.At(4+x, y))
		}
	}

	got := RenderToImage(crop, 1, 1, Vertical)
	want := RenderToImage(normalized, 1, 1, Vertical)
	if got.Bounds().Dx() != want.Bounds().Dx() || got.Bounds().Dy() != want.Bounds().Dy() {
		t.Fatalf("RenderToImage bounds = %v, want dims %v", got.Bounds(), want.Bounds())
	}
	gb := got.Bounds()
	wb := want.Bounds()
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			if got.At(gb.Min.X+x, gb.Min.Y+y) != want.At(wb.Min.X+x, wb.Min.Y+y) {
				t.Fatalf("RenderToImage pixel %d,%d differs", x, y)
			}
		}
	}
}

func TestReconstructedCellColorIncludesEmittedTransparentCoverage(t *testing.T) {
	cell := cellResult{
		Ch: '▘',
		FG: color.RGBA{R: 200, A: 255},
		BG: color.RGBA{B: 100, A: 255},
	}

	if got := reconstructedCellColor(cell, 0, 0, 2, 2); got != cell.FG {
		t.Fatalf("foreground coverage = %#v, want %#v", got, cell.FG)
	}
	if got := reconstructedCellColor(cell, 1, 1, 2, 2); got != cell.BG {
		t.Fatalf("background coverage = %#v, want %#v", got, cell.BG)
	}

	transparent := cellResult{Ch: '▘', FG: color.RGBA{R: 200, A: 255}}
	if got := reconstructedCellColor(transparent, 1, 1, 2, 2); got != (color.RGBA{}) {
		t.Fatalf("unpainted coverage = %#v, want transparent", got)
	}
}

func TestRenderTransparentHalfEmitsOnlyForeground(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := 0; x < 2; x++ {
		img.SetRGBA(x, 0, color.RGBA{R: 200, G: 20, A: 255})
	}

	var out strings.Builder
	if err := Render(&out, img, 1, Options{Mode: HalfSplit, NoLinePrefix: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if want := "\x1b[38;2;200;20;0m▀\x1b[0m\n"; out.String() != want {
		t.Fatalf("ANSI output = %q, want exactly %q", out.String(), want)
	}
	if strings.Contains(out.String(), "\x1b[48;") {
		t.Fatalf("transparent lower half unexpectedly emitted background: %q", out.String())
	}
}

func TestFindBestCandidateRejectsBackgroundOnlyNonSpace(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(1, 0, color.RGBA{B: 255, A: 255})
	candidates := []candidate{
		{ch: '▌', mask: func(x, _, _, _ int) bool { return x == 0 }},
		{ch: ' ', mask: func(_, _, _, _ int) bool { return false }},
	}

	got := findBestCandidate(img, img.Bounds(), 0, 1, 0, 0, candidates)
	if got.Ch != ' ' {
		t.Fatalf("background-only non-space candidate was selected: %#v", got)
	}
}

func TestRenderOptsSparkQuad(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			c := color.RGBA{R: 0, G: 0, B: 255, A: 255}
			if x < 2 && y < 4 {
				c = color.RGBA{R: 255, G: 0, B: 0, A: 255}
			}
			img.Set(x, y, c)
		}
	}

	var buf strings.Builder
	err := RenderOpts(&buf, img, 1, 1, Quad)
	if err != nil {
		t.Fatalf("RenderOpts(Quad) error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "▘") {
		t.Fatalf("RenderOpts(Quad) = %q, want upper-left quad glyph", output)
	}
}

func TestRenderOptsSextantUsesSextantGlyphs(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			switch {
			case y < 3 && x < 2:
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			case y < 3:
				img.Set(x, y, color.RGBA{G: 255, A: 255})
			default:
				img.Set(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}

	var buf strings.Builder
	if err := RenderOpts(&buf, img, 1, 1, Sextant); err != nil {
		t.Fatalf("RenderOpts(Sextant): %v", err)
	}

	output := buf.String()
	found := false
	for _, r := range output {
		if r >= 0x1FB00 && r <= 0x1FB3B {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("RenderOpts(Sextant) = %q, want sextant glyph", output)
	}
}
