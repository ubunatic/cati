package halfblock

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

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

// ── pairToCell ────────────────────────────────────────────────────────────────

func TestPairToCell(t *testing.T) {
	t.Helper()
	red := rgba(255, 0, 0, 255)
	blue := rgba(0, 0, 255, 255)
	transp := rgba(0, 0, 0, 0)

	tests := []struct {
		name    string
		top     color.RGBA
		bot     color.RGBA
		wantCh  rune
		wantFG  bool
		wantBG  bool
		wantTrp bool
	}{
		{"both transparent", transp, transp, ' ', false, false, true},
		{"top only", red, transp, '▀', true, false, false},
		{"bot only", transp, blue, '▄', true, false, false},
		{"same color solid", red, red, '█', true, false, false},
		{"two colors", red, blue, '▀', true, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := pairToCell(tc.top, tc.bot)
			if c.ch != tc.wantCh {
				t.Errorf("ch: got %q want %q", string(c.ch), string(tc.wantCh))
			}
			if c.hasFG != tc.wantFG {
				t.Errorf("hasFG: got %v want %v", c.hasFG, tc.wantFG)
			}
			if c.hasBG != tc.wantBG {
				t.Errorf("hasBG: got %v want %v", c.hasBG, tc.wantBG)
			}
			if c.transparent != tc.wantTrp {
				t.Errorf("transparent: got %v want %v", c.transparent, tc.wantTrp)
			}
		})
	}
}

// ── Scale ─────────────────────────────────────────────────────────────────────

func TestScale(t *testing.T) {
	red := rgba(255, 0, 0, 255)
	orig := solidImage(100, 50, red)

	tests := []struct {
		name    string
		maxCols int
		wantW   int
		wantH   int
	}{
		{"no scale needed", 200, 100, 50},
		{"exact fit", 100, 100, 50},
		{"scale to half", 50, 50, 25},
		{"zero maxCols → no scale", 0, 100, 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := Scale(orig, tc.maxCols)
			b := out.Bounds()
			gotW, gotH := b.Dx(), b.Dy()
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("Scale(%d): got %dx%d want %dx%d", tc.maxCols, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

// ── ScaleToFit ────────────────────────────────────────────────────────────────

func TestScaleToFit(t *testing.T) {
	red := rgba(255, 0, 0, 255)
	// 100×50 pixels → aspect ratio 2:1
	orig := solidImage(100, 50, red)

	tests := []struct {
		name    string
		cols    int
		rows    int
		wantW   int
		wantH   int
	}{
		// Width-only constraint (rows=0)
		{"width only, fits", 200, 0, 100, 50},
		{"width only, exact", 100, 0, 100, 50},
		{"width only, half", 50, 0, 50, 25},

		// Height-only constraint (cols=0).
		// rows=10 → pixelH=20; 100px wide × 50px tall scaled to 20px tall
		// → width = 100*20/50 = 40 cols.
		{"height only, fits tall", 0, 50, 100, 50},
		{"height only, shrink", 0, 10, 40, 20},

		// Both constraints — tighter wins.
		// rows=10 → pixelH=20 → width budget 40; cols=50 is looser → height wins.
		{"both, height tighter", 50, 10, 40, 20},
		// rows=100 → pixelH=200 → height is not the constraint; cols=50 → width wins.
		{"both, width tighter", 50, 100, 50, 25},

		// No constraints → unchanged.
		{"no constraints", 0, 0, 100, 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := ScaleToFit(orig, tc.cols, tc.rows)
			b := out.Bounds()
			gotW, gotH := b.Dx(), b.Dy()
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("ScaleToFit(cols=%d, rows=%d): got %dx%d want %dx%d",
					tc.cols, tc.rows, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

// ── Render ────────────────────────────────────────────────────────────────────

func TestRender_SolidColor(t *testing.T) {
	red := rgba(255, 0, 0, 255)
	img := solidImage(4, 4, red)

	var sb strings.Builder
	if err := Render(&sb, img, img.Bounds().Dx(), Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()

	// Expect 2 terminal rows (4 pixel rows / 2) each followed by a newline.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), out)
	}
	// Every line must contain the █ character (solid color = same fg top & bot).
	for i, l := range lines {
		if !strings.ContainsRune(l, '█') {
			t.Errorf("line %d: expected █, got %q", i, l)
		}
	}
}

func TestRender_TransparentImage(t *testing.T) {
	transp := rgba(0, 0, 0, 0)
	img := solidImage(4, 2, transp)

	var sb strings.Builder
	if err := Render(&sb, img, img.Bounds().Dx(), Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	// ansiEraseLine (\x1b[2K) is always emitted per line — that's expected.
	// But no color (fg/bg) escape sequences must appear for a fully transparent image.
	if strings.Contains(out, "38;") || strings.Contains(out, "48;") {
		t.Errorf("transparent image emitted color ANSI escapes: %q", out)
	}
}

func TestRender_OddHeight(t *testing.T) {
	// 4x3 image (odd height) — last terminal row has only a top pixel.
	red := rgba(255, 0, 0, 255)
	img := solidImage(4, 3, red)

	var sb strings.Builder
	if err := Render(&sb, img, img.Bounds().Dx(), Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// ceil(3/2) = 2 terminal rows.
	if len(lines) != 2 {
		t.Errorf("expected 2 lines for 3-row image, got %d: %q", len(lines), out)
	}
}

func TestRender_TwoColors(t *testing.T) {
	// A single terminal row where top pixel = red, bottom pixel = blue.
	// This forces a ▀ (fg=red, bg=blue) output.
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	red := rgba(255, 0, 0, 255)
	blue := rgba(0, 0, 255, 255)
	for x := 0; x < 4; x++ {
		img.Set(x, 0, red)  // top pixel row
		img.Set(x, 1, blue) // bottom pixel row
	}

	var sb strings.Builder
	if err := Render(&sb, img, img.Bounds().Dx(), Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	// Each cell has red top / blue bottom → ▀ with fg=red bg=blue.
	if !strings.ContainsRune(out, '▀') {
		t.Errorf("expected ▀ characters in output: %q", out)
	}
	// Both fg (red) and bg (blue) ANSI sequences must appear.
	if !strings.Contains(out, "38;2;255;0;0") {
		t.Errorf("expected red fg escape in output: %q", out)
	}
	if !strings.Contains(out, "48;2;0;0;255") {
		t.Errorf("expected blue bg escape in output: %q", out)
	}
}

// ── PNG fixture integration ───────────────────────────────────────────────────

func TestRender_PNGFixtures(t *testing.T) {
	fixtures := []struct {
		file    string
		minRows int // minimum expected terminal rows
	}{
		{"../../testdata/solid_red_4x4.png", 2},
		{"../../testdata/checkerboard_4x4.png", 2},
		{"../../testdata/top_green_bot_yellow_6x4.png", 2},
		{"../../testdata/transparent_top_4x2.png", 1},
	}

	for _, fix := range fixtures {
		t.Run(fix.file, func(t *testing.T) {
			img, err := loadPNG(fix.file)
			if err != nil {
				t.Fatalf("loadPNG: %v", err)
			}
			var sb strings.Builder
			if err := Render(&sb, img, img.Bounds().Dx(), Options{}); err != nil {
				t.Fatalf("Render: %v", err)
			}
			lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
			if len(lines) < fix.minRows {
				t.Errorf("expected >= %d lines, got %d", fix.minRows, len(lines))
			}
		})
	}
}
