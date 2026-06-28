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
		{Quad, "spark/quad"},
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
	if len(ms) != 2 {
		t.Fatalf("Modes() returned %d entries, want 2", len(ms))
	}
	if ms[0] != Vertical || ms[1] != Quad {
		t.Errorf("Modes() order incorrect: got %v", ms)
	}
}

func TestCycle(t *testing.T) {
	if got := Cycle(Quad); got != Vertical {
		t.Errorf("Cycle(Quad) = %d, want Vertical", got)
	}
	if got := Cycle(Vertical); got != Quad {
		t.Errorf("Cycle(Vertical) = %d, want Quad", got)
	}
	if got := CyclePrev(Vertical); got != Quad {
		t.Errorf("CyclePrev(Vertical) = %d, want Quad", got)
	}
	if got := CyclePrev(Quad); got != Vertical {
		t.Errorf("CyclePrev(Quad) = %d, want Vertical", got)
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
