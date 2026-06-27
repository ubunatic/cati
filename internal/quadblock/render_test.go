package quadblock

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"codeberg.org/ubunatic/cati/internal/halfblock"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func rgba(r, g, b, a uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: a} }

var (
	red    = rgba(255, 0, 0, 255)
	green  = rgba(0, 255, 0, 255)
	blue   = rgba(0, 0, 255, 255)
	transp = rgba(0, 0, 0, 0)
)

// solidImage returns a w×h image filled with c.
func solidImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// makeImage builds an image from a row-major slice of colors (w×h pixels).
func makeImage(w, h int, pixels []color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, pixels[y*w+x])
		}
	}
	return img
}

func imagesEqual(a, b image.Image) bool {
	ab := a.Bounds()
	bb := b.Bounds()
	if ab != bb {
		return false
	}
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}

// ── quadChar table ────────────────────────────────────────────────────────────

func TestQuadCharTable(t *testing.T) {
	cases := []struct {
		mask uint8
		want rune
	}{
		{0b0000, ' '},
		{0b0001, '▗'}, // LR
		{0b0010, '▖'}, // LL
		{0b0011, '▄'}, // LL+LR
		{0b0100, '▝'}, // UR
		{0b0110, '▞'}, // UR+LL
		{0b0111, '▟'}, // UR+LL+LR
		{0b1000, '▘'}, // UL
		{0b1001, '▚'}, // UL+LR
		{0b1011, '▙'}, // UL+LL+LR
		{0b1100, '▀'}, // UL+UR
		{0b1101, '▜'}, // UL+UR+LR
		{0b1110, '▛'}, // UL+UR+LL
		{0b1111, '█'}, // all
	}
	for _, tc := range cases {
		got := quadChar[tc.mask]
		if got != tc.want {
			t.Errorf("quadChar[%04b] = %q, want %q", tc.mask, string(got), string(tc.want))
		}
	}
	// Approximated masks must still produce a valid (non-space) character.
	for _, m := range []uint8{0b0101, 0b1010} {
		if quadChar[m] == ' ' {
			t.Errorf("quadChar[%04b] must not be space (approximated mask)", m)
		}
	}
}

// ── buildMask ─────────────────────────────────────────────────────────────────

func TestBuildMask(t *testing.T) {
	cases := []struct {
		name    string
		pixels  [4]color.RGBA // UL, UR, LL, LR
		fg, bg  color.RGBA
		hasBG   bool
		wantMask uint8
	}{
		{
			name:     "all fg → 1111",
			pixels:   [4]color.RGBA{red, red, red, red},
			fg: red, hasBG: false,
			wantMask: 0b1111,
		},
		{
			name:     "all transparent → 0000",
			pixels:   [4]color.RGBA{transp, transp, transp, transp},
			fg: red, hasBG: false,
			wantMask: 0b0000,
		},
		{
			name:     "UL only → 1000",
			pixels:   [4]color.RGBA{red, transp, transp, transp},
			fg: red, hasBG: false,
			wantMask: 0b1000,
		},
		{
			name:     "top half → 1100",
			pixels:   [4]color.RGBA{red, red, transp, transp},
			fg: red, hasBG: false,
			wantMask: 0b1100,
		},
		{
			name:     "two-colour top=fg bot=bg → 1100",
			pixels:   [4]color.RGBA{red, red, blue, blue},
			fg: red, bg: blue, hasBG: true,
			wantMask: 0b1100,
		},
		{
			name:     "diagonal UL+LR → 1001",
			pixels:   [4]color.RGBA{red, blue, blue, red},
			fg: red, bg: blue, hasBG: true,
			wantMask: 0b1001,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildMask(tc.pixels, tc.fg, tc.bg, tc.hasBG)
			if got != tc.wantMask {
				t.Errorf("mask: got %04b, want %04b", got, tc.wantMask)
			}
		})
	}
}

// ── collectUnique ─────────────────────────────────────────────────────────────

func TestCollectUnique(t *testing.T) {
	t.Run("all same", func(t *testing.T) {
		u := collectUnique([4]color.RGBA{red, red, red, red})
		if len(u) != 1 || !eqRGB(u[0], red) {
			t.Errorf("got %v", u)
		}
	})
	t.Run("all transparent", func(t *testing.T) {
		u := collectUnique([4]color.RGBA{transp, transp, transp, transp})
		if len(u) != 0 {
			t.Errorf("expected empty, got %v", u)
		}
	})
	t.Run("two colours", func(t *testing.T) {
		u := collectUnique([4]color.RGBA{red, blue, red, blue})
		if len(u) != 2 {
			t.Errorf("expected 2 unique, got %d", len(u))
		}
	})
	t.Run("transparent mixed in", func(t *testing.T) {
		u := collectUnique([4]color.RGBA{transp, red, transp, red})
		if len(u) != 1 {
			t.Errorf("expected 1 unique, got %d", len(u))
		}
	})
}

// ── compileCell ───────────────────────────────────────────────────────────────

func TestCompileCell_AllTransparent(t *testing.T) {
	c := compileCell([4]color.RGBA{transp, transp, transp, transp}, nil, nil, Options{})
	if !c.transparent {
		t.Error("expected transparent cell")
	}
}

func TestCompileCell_SolidColor(t *testing.T) {
	c := compileCell([4]color.RGBA{red, red, red, red}, nil, nil, Options{})
	if c.transparent {
		t.Error("expected non-transparent")
	}
	if c.ch != '█' {
		t.Errorf("ch: got %q, want █", string(c.ch))
	}
	if !eqRGB(c.fg, red) {
		t.Errorf("fg: got %v, want red", c.fg)
	}
	if c.hasBG {
		t.Error("hasBG should be false for solid color")
	}
}

func TestCompileCell_TopHalf(t *testing.T) {
	// UL+UR = red, LL+LR = transparent → top half red
	c := compileCell([4]color.RGBA{red, red, transp, transp}, nil, nil, Options{})
	if c.ch != '▀' {
		t.Errorf("ch: got %q, want ▀", string(c.ch))
	}
	if !eqRGB(c.fg, red) {
		t.Errorf("fg: got %v, want red", c.fg)
	}
}

func TestCompileCell_BottomHalf(t *testing.T) {
	// UL+UR = transparent, LL+LR = blue → bottom half blue
	c := compileCell([4]color.RGBA{transp, transp, blue, blue}, nil, nil, Options{})
	if c.ch != '▄' {
		t.Errorf("ch: got %q, want ▄", string(c.ch))
	}
	if !eqRGB(c.fg, blue) {
		t.Errorf("fg: got %v, want blue", c.fg)
	}
}

func TestCompileCell_TwoColours(t *testing.T) {
	// top row = red, bottom row = blue → ▀ fg=red bg=blue
	c := compileCell([4]color.RGBA{red, red, blue, blue}, nil, nil, Options{})
	if c.ch != '▀' {
		t.Errorf("ch: got %q, want ▀", string(c.ch))
	}
	if !eqRGB(c.fg, red) {
		t.Errorf("fg: got %v, want red", c.fg)
	}
	if !c.hasBG || !eqRGB(c.bg, blue) {
		t.Errorf("bg: got %v hasBG=%v, want blue", c.bg, c.hasBG)
	}
}

func TestCompileCell_Diagonal(t *testing.T) {
	// UL+LR = red, UR+LL = blue → ▚ fg=red bg=blue
	c := compileCell([4]color.RGBA{red, blue, blue, red}, nil, nil, Options{})
	if c.ch != '▚' {
		t.Errorf("ch: got %q, want ▚", string(c.ch))
	}
}

func TestCompileCell_VerticalSplit(t *testing.T) {
	// Left column = red, right column = blue → vertical split.
	cases := []struct {
		name string
		opts Options
	}{
		{name: "default", opts: Options{}},
		{name: "diameter", opts: Options{Diameter: true}},
		{name: "kmeans", opts: Options{KMeans: 3}},
		{name: "pca2", opts: Options{PCA2: true}},
		{name: "lumsplit", opts: Options{LumSplit: true}},
		{name: "edgesnap", opts: Options{EdgeSnap: true}},
		{name: "splithalf", opts: Options{SplitHalf: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := compileCell([4]color.RGBA{red, blue, red, blue}, nil, nil, tc.opts)
			if c.ch != '▌' {
				t.Fatalf("ch: got %q, want ▌", string(c.ch))
			}
			if !eqRGB(c.fg, red) || !c.hasBG || !eqRGB(c.bg, blue) {
				t.Fatalf("fg/bg: got fg=%v bg=%v hasBG=%v, want red/blue", c.fg, c.bg, c.hasBG)
			}
		})
	}
}

func TestCompileCell_Quantisation(t *testing.T) {
	// 3 colours: red (2px), blue (1px), green (1px) → must pick red+one other
	c := compileCell([4]color.RGBA{red, red, blue, green}, nil, nil, Options{})
	if c.transparent {
		t.Error("expected non-transparent")
	}
	// fg must be red (majority)
	if !eqRGB(c.fg, red) {
		t.Errorf("fg: got %v, want red (majority colour)", c.fg)
	}
}

func TestCompileCell_NeighborContinuity(t *testing.T) {
	// Cell has 3 colours: red (2), blue (1), green (1).
	// Left neighbour uses (red, blue).  Continuity should favour blue over green.
	leftCell := &quadCell{ch: '▀', fg: red, bg: blue, hasFG: true, hasBG: true}
	c := compileCell([4]color.RGBA{red, red, blue, green}, leftCell, nil, Options{})

	if !eqRGB(c.fg, red) {
		t.Errorf("fg: got %v, want red", c.fg)
	}
	// bg should be blue (continued from neighbour), not green
	if !c.hasBG || !eqRGB(c.bg, blue) {
		t.Errorf("bg: got %v hasBG=%v; neighbour continuity should prefer blue", c.bg, c.hasBG)
	}
}

// ── Render ────────────────────────────────────────────────────────────────────

func TestRender_SolidColor(t *testing.T) {
	img := solidImage(4, 4, red)
	var sb strings.Builder
	if err := Render(&sb, img); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// 4 pixel rows → 2 terminal rows
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), out)
	}
	for i, l := range lines {
		if !strings.ContainsRune(l, '█') {
			t.Errorf("line %d: expected █ for solid image, got %q", i, l)
		}
	}
}

func TestRender_AllTransparent(t *testing.T) {
	img := solidImage(4, 4, transp)
	var sb strings.Builder
	if err := Render(&sb, img); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	if strings.Contains(out, "38;") || strings.Contains(out, "48;") {
		t.Errorf("transparent image emitted color escapes: %q", out)
	}
}

func TestRender_TwoColorHalves(t *testing.T) {
	// A 4×2 image: pixel row 0 = red (UL+UR), pixel row 1 = blue (LL+LR).
	// Each terminal cell sees top=red, bot=blue → ▀ fg=red bg=blue.
	img := makeImage(4, 2, []color.RGBA{
		red, red, red, red,
		blue, blue, blue, blue,
	})
	var sb strings.Builder
	if err := Render(&sb, img); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	if !strings.ContainsRune(out, '▀') {
		t.Errorf("expected ▀ for top-red/bottom-blue image: %q", out)
	}
	if !strings.Contains(out, "38;2;255;0;0") {
		t.Errorf("expected red fg escape: %q", out)
	}
	if !strings.Contains(out, "48;2;0;0;255") {
		t.Errorf("expected blue bg escape: %q", out)
	}
}

func TestRender_LeftRightSplit(t *testing.T) {
	img := makeImage(2, 2, []color.RGBA{
		red, blue,
		red, blue,
	})
	var sb strings.Builder
	if err := Render(&sb, img); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	if !strings.ContainsRune(out, '▌') {
		t.Fatalf("expected ▌ for left-red/right-blue image: %q", out)
	}
	if !strings.Contains(out, "38;2;255;0;0") {
		t.Errorf("expected red fg escape: %q", out)
	}
	if !strings.Contains(out, "48;2;0;0;255") {
		t.Errorf("expected blue bg escape: %q", out)
	}
}

func TestRender_OddDimensions(t *testing.T) {
	// 3×3 image → 2×2 terminal cells (odd dims padded with transparent)
	img := solidImage(3, 3, green)
	var sb strings.Builder
	if err := Render(&sb, img); err != nil {
		t.Fatalf("Render: %v", err)
	}
	lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 terminal rows for 3-pixel-high image, got %d", len(lines))
	}
}

func TestRender_SinglePixel(t *testing.T) {
	img := solidImage(1, 1, red)
	var sb strings.Builder
	if err := Render(&sb, img); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := sb.String()
	// 1×1 pixel → 1 terminal cell → 1 line with UL quadrant only (▘)
	if !strings.ContainsRune(out, '▘') {
		t.Errorf("expected ▘ for single-pixel image: %q", out)
	}
}

func TestRenderToImage_LeftRightSplit(t *testing.T) {
	img := makeImage(2, 2, []color.RGBA{
		red, blue,
		red, blue,
	})
	out := RenderToImage(img, Options{})
	if !imagesEqual(img, out) {
		t.Fatalf("RenderToImage mismatch for vertical split")
	}
}

// ── ScaleToFit ────────────────────────────────────────────────────────────────

func TestScaleToFit(t *testing.T) {
	// Source: 100×50 px (2:1 aspect).
	// The 2× horizontal stretch is always applied: stretchedW = 200.
	// maxW = cols*2, maxH = rows*2.
	orig := solidImage(100, 50, red)

	cases := []struct {
		name       string
		cols, rows int
		wantW, wantH int
	}{
		// stretchedW=200 ≤ maxW=200, srcH=50 ≤ maxH=∞ → scale to 200×50.
		{"cols=100 stretches 2×", 100, 0, 200, 50},
		// stretchedW=200 > maxW=20 → targetH=50*20/200=5, targetW=20.
		{"cols=10 shrinks", 10, 0, 20, 5},
		// stretchedW=200, srcH=50 > maxH=10 → targetW=200*10/50=40, targetH=10.
		{"rows=5 shrinks", 0, 5, 40, 10},
		// Width tighter: cols=10 → targetW=20, targetH=5; rows=100 → maxH=200 ≥ 5 ✓
		{"cols tight", 10, 100, 20, 5},
		// Height tighter: cols=100 → maxW=200 ≥ stretchedW=200 → no width cut;
		// rows=5 → maxH=10 < srcH=50 → targetW=200*10/50=40, targetH=10.
		{"rows tight", 100, 5, 40, 10},
		// No constraints → always apply 2× horizontal stretch → 200×50.
		{"unconstrained", 0, 0, 200, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := ScaleToFit(orig, tc.cols, tc.rows)
			b := out.Bounds()
			gotW, gotH := b.Dx(), b.Dy()
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("ScaleToFit(cols=%d,rows=%d): got %dx%d, want %dx%d",
					tc.cols, tc.rows, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestScaleToFit_HorizontalStretch(t *testing.T) {
	// A 4×4 source with generous constraints: the 2× horizontal stretch is
	// applied (stretchedW=8) and fits within maxW=200 → result is 8×4.
	small := solidImage(4, 4, red)
	out := ScaleToFit(small, 100, 100)
	b := out.Bounds()
	if b.Dx() != 8 || b.Dy() != 4 {
		t.Errorf("ScaleToFit should apply 2× horizontal stretch: got %dx%d, want 8x4", b.Dx(), b.Dy())
	}
}

// ── PNG fixture integration ───────────────────────────────────────────────────

func TestRender_PNGFixtures(t *testing.T) {
	fixtures := []struct {
		file    string
		minRows int
	}{
		{"../../testdata/solid_red_4x4.png", 2},
		{"../../testdata/checkerboard_4x4.png", 2},
		{"../../testdata/top_green_bot_yellow_6x4.png", 2},
		{"../../testdata/transparent_top_4x2.png", 1},
	}
	for _, fix := range fixtures {
		t.Run(fix.file, func(t *testing.T) {
			img, err := halfblock.LoadImage(fix.file)
			if err != nil {
				t.Fatalf("LoadImage: %v", err)
			}
			var sb strings.Builder
			if err := Render(&sb, img); err != nil {
				t.Fatalf("Render: %v", err)
			}
			lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
			if len(lines) < fix.minRows {
				t.Errorf("expected >= %d lines, got %d", fix.minRows, len(lines))
			}
		})
	}
}

func TestRender_SampleImages(t *testing.T) {
	// Smoke test: render larger sample images without panicking.
	samples := []string{
		"../../assets/samples/sample-001-soldering-practice-2025.jpg",
		"../../assets/samples/sample-002-summer-vacation.jpg",
	}
	for _, path := range samples {
		t.Run(path, func(t *testing.T) {
			img, err := halfblock.LoadImage(path)
			if err != nil {
				t.Skipf("LoadImage: %v", err)
			}
			img = ScaleToFit(img, 40, 20)
			var sb strings.Builder
			if err := Render(&sb, img); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if sb.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}
