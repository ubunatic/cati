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
		{LowerHorizontal, "spark/lower"},
		{LeftVertical, "spark/left"},
		{Mode(99), "spark/lower"},
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
	if ms[0] != LowerHorizontal || ms[1] != LeftVertical {
		t.Errorf("Modes() order incorrect: got %v", ms)
	}
}

func TestCycle(t *testing.T) {
	if got := Cycle(LeftVertical); got != LowerHorizontal {
		t.Errorf("Cycle(LeftVertical) = %d, want LowerHorizontal", got)
	}
	if got := Cycle(LowerHorizontal); got != LeftVertical {
		t.Errorf("Cycle(LowerHorizontal) = %d, want LeftVertical", got)
	}
	if got := CyclePrev(LowerHorizontal); got != LeftVertical {
		t.Errorf("CyclePrev(LowerHorizontal) = %d, want LeftVertical", got)
	}
	if got := CyclePrev(LeftVertical); got != LowerHorizontal {
		t.Errorf("CyclePrev(LeftVertical) = %d, want LowerHorizontal", got)
	}
	if got := Cycle(Mode(99)); got != LowerHorizontal {
		t.Errorf("Cycle(99) = %d, want LowerHorizontal", got)
	}
}

func TestCharLowerHorizontal(t *testing.T) {
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
		ch, swap := Char(LowerHorizontal, tc.v)
		if ch != tc.want {
			t.Errorf("Char(LowerHorizontal, %v) = %c, want %c", tc.v, ch, tc.want)
		}
		if swap {
			t.Errorf("Char(LowerHorizontal, %v): swapFgBg = true, want false", tc.v)
		}
	}
}

func TestCharLeftVertical(t *testing.T) {
	ch, swap := Char(LeftVertical, 0)
	if ch != '\u258F' {
		t.Errorf("Char(LeftVertical, 0) = %c, want ▏", ch)
	}
	if swap {
		t.Errorf("Char(LeftVertical, 0): swapFgBg = true, want false")
	}

	ch, swap = Char(LeftVertical, 1)
	if ch != '\u2588' {
		t.Errorf("Char(LeftVertical, 1) = %c, want █", ch)
	}
}

func TestCharEdgeCases(t *testing.T) {
	// Negative value treated as 0
	ch, swap := Char(LowerHorizontal, -0.5)
	if ch != '\u2581' {
		t.Errorf("Char(LowerHorizontal, -0.5) = %c, want ▁", ch)
	}
	_ = swap

	// Very large value treated as 1
	ch, _ = Char(LowerHorizontal, 2.0)
	if ch != '\u2588' {
		t.Errorf("Char(LowerHorizontal, 2.0) = %c, want █", ch)
	}
}

func TestString(t *testing.T) {
	values := []float64{0, 0.25, 0.5, 0.75, 1.0}
	s := String(LowerHorizontal, values)
	expected := "\u2581\u2583\u2585\u2587\u2588"
	if s != expected {
		t.Errorf("String(LowerHorizontal) = %q, want %q", s, expected)
	}
}

func TestStringEmpty(t *testing.T) {
	s := String(LowerHorizontal, nil)
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
	err := RenderOpts(&buf, img, 2, 1, LowerHorizontal)
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



func TestRenderOptsLeftVertical(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	var buf strings.Builder
	err := RenderOpts(&buf, img, 1, 1, LeftVertical)
	if err != nil {
		t.Fatalf("RenderOpts(LeftVertical) error: %v", err)
	}

	output := buf.String()
	// Left vertical should use left blocks (U+258F)
	if strings.Contains(output, "\u258F") || strings.Contains(output, "\u258E") {
		return // found a left block char
	}
	t.Error("RenderOpts(LeftVertical) missing left block characters")
}
