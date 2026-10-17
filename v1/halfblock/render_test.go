package halfblock

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"

	"ubunatic.com/cati/internal/ansitest"
	"ubunatic.com/cati/v1/core"
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

func TestFastpathParity(t *testing.T) {
	previous := core.Fastpath
	defer func() { core.Fastpath = previous }()

	rect := image.Rect(-2, 3, 30, 22)
	rgbaImg := image.NewRGBA(rect)
	nrgbaImg := image.NewNRGBA(rect)
	grayImg := image.NewGray(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := color.RGBA{R: uint8((x + 2) * 7), G: uint8((y - 3) * 11), B: uint8((x + y + 16) * 5), A: 255}
			switch (x + y) % 7 {
			case 0:
				c.A = 0
			case 3:
				c.A = 128
			case 5:
				c.A = 1
			}
			nc := color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
			rgbaImg.SetRGBA(x, y, toRGBA(nc))
			nrgbaImg.SetNRGBA(x, y, nc)
			grayImg.SetGray(x, y, color.Gray{Y: uint8(x + y)})
		}
	}

	for _, tc := range []struct {
		name string
		img  image.Image
	}{
		{name: "RGBA", img: rgbaImg},
		{name: "NRGBA", img: nrgbaImg},
		{name: "Gray", img: grayImg},
		{name: "RGBA subimage odd height", img: rgbaImg.SubImage(image.Rect(-1, 5, 18, 18))},
		{name: "NRGBA subimage odd height", img: nrgbaImg.SubImage(image.Rect(-1, 5, 18, 18))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core.Fastpath = true
			var fast bytes.Buffer
			if err := Render(&fast, tc.img, 12, Options{Rows: 8, NoLinePrefix: true}); err != nil {
				t.Fatalf("fast Render: %v", err)
			}
			fastImage := RenderToImage(tc.img)
			fastImageJ := RenderToImageJ(tc.img, 4)
			fastScaled := ScaleNN(tc.img, 17, 11)

			core.Fastpath = false
			var simple bytes.Buffer
			if err := Render(&simple, tc.img, 12, Options{Rows: 8, NoLinePrefix: true}); err != nil {
				t.Fatalf("simple Render: %v", err)
			}
			simpleImage := RenderToImage(tc.img)
			simpleImageJ := RenderToImageJ(tc.img, 4)
			simpleScaled := ScaleNN(tc.img, 17, 11)

			if !bytes.Equal(fast.Bytes(), simple.Bytes()) {
				t.Fatal("ANSI output differs between fast and simple paths")
			}
			if fastImage.Bounds() != simpleImage.Bounds() || !bytes.Equal(fastImage.Pix, simpleImage.Pix) {
				t.Fatal("RenderToImage output differs between fast and simple paths")
			}
			if !bytes.Equal(fastImageJ.Pix, simpleImageJ.Pix) || !bytes.Equal(fastImageJ.Pix, fastImage.Pix) {
				t.Fatal("RenderToImageJ(jobs=4) output differs from fast/simple or jobs=1 paths")
			}
			fastRGBA, fastOK := fastScaled.(*image.RGBA)
			simpleRGBA, simpleOK := simpleScaled.(*image.RGBA)
			if !fastOK || !simpleOK || fastRGBA.Bounds() != simpleRGBA.Bounds() || !bytes.Equal(fastRGBA.Pix, simpleRGBA.Pix) {
				t.Fatal("ScaleNN output differs between fast and simple paths")
			}
		})
	}
}

// ── pairToCell ────────────────────────────────────────────────────────────────

func TestReconstructionMatchesSourceANSIColors(t *testing.T) {
	previous := core.Fastpath
	t.Cleanup(func() { core.Fastpath = previous })
	parent := image.NewNRGBA(image.Rect(-3, 2, 6, 9))
	for y := 2; y < 9; y++ {
		for x := -3; x < 6; x++ {
			parent.SetNRGBA(x, y, color.NRGBA{
				R: 210, G: uint8((x + 3) * 23), B: uint8(y * 21),
				A: []uint8{0, 1, 128, 255}[(x+y+3)%4],
			})
		}
	}
	src := parent.SubImage(image.Rect(-1, 3, 4, 8))
	for _, fast := range []bool{false, true} {
		core.Fastpath = fast
		for _, jobs := range []int{1, 4} {
			got := RenderToImageJ(src, jobs)
			if got.Bounds() != src.Bounds() {
				t.Fatalf("bounds = %v, want %v", got.Bounds(), src.Bounds())
			}
			for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
				for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
					r, g, b, a := src.At(x, y).RGBA()
					want := color.RGBA{}
					if a != 0 {
						want = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
					}
					if pixel := got.RGBAAt(x, y); pixel != want {
						t.Fatalf("fast=%v jobs=%d pixel (%d,%d) = %v, want %v", fast, jobs, x, y, pixel, want)
					}
				}
			}
		}
	}
}

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
		name  string
		cols  int
		rows  int
		wantW int
		wantH int
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

	rows := parseRenderCells(t, out, 2, 4)
	for y, row := range rows {
		for x, cell := range row {
			want := ansitest.Cell{Rune: '█', FG: ansitest.RGB(255, 0, 0)}
			if cell != want {
				t.Errorf("cell (%d,%d) = %+v, want %+v", x, y, cell, want)
			}
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

func TestRender_NoLinePrefix(t *testing.T) {
	img := solidImage(2, 2, rgba(255, 0, 0, 255))
	var sb strings.Builder
	if err := Render(&sb, img, img.Bounds().Dx(), Options{NoLinePrefix: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(sb.String(), ansiLinePrefix) || strings.Contains(sb.String(), "\r") {
		t.Fatalf("NoLinePrefix output contains line prefix: %q", sb.String())
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
	rows := parseRenderCells(t, out, 1, 4)
	for x, cell := range rows[0] {
		want := ansitest.Cell{Rune: '▀', FG: ansitest.RGB(255, 0, 0), BG: ansitest.RGB(0, 0, 255)}
		if cell != want {
			t.Errorf("cell (%d,0) = %+v, want %+v", x, cell, want)
		}
	}
}

func TestRenderToImage_ANSIColorsAreOpaque(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 40, B: 10, A: 128})
	img.SetNRGBA(0, 1, color.NRGBA{})

	var ansi strings.Builder
	if err := Render(&ansi, img, 1, Options{NoLinePrefix: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	topCell := parseRenderCells(t, ansi.String(), 1, 1)[0][0]
	if want := (ansitest.Cell{Rune: '▀', FG: ansitest.RGB(100, 20, 5)}); topCell != want {
		t.Fatalf("top cell = %+v, want %+v", topCell, want)
	}

	serial := RenderToImage(img)
	got := serial.RGBAAt(0, 0)
	if got != (color.RGBA{R: 100, G: 20, B: 5, A: 255}) {
		t.Fatalf("RenderToImage painted pixel = %#v, want opaque premultiplied ANSI color", got)
	}
	if got := serial.RGBAAt(0, 1); got != (color.RGBA{}) {
		t.Fatalf("RenderToImage transparent half = %#v, want transparent", got)
	}
	parallel := RenderToImageJ(img, 2)
	if parallel.RGBAAt(0, 0) != serial.RGBAAt(0, 0) || parallel.RGBAAt(0, 1) != serial.RGBAAt(0, 1) {
		t.Fatalf("worker reconstruction differs: serial=%v/%v parallel=%v/%v",
			serial.RGBAAt(0, 0), serial.RGBAAt(0, 1), parallel.RGBAAt(0, 0), parallel.RGBAAt(0, 1))
	}

	bottom := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	bottom.SetNRGBA(0, 1, color.NRGBA{R: 10, G: 40, B: 200, A: 128})
	var bottomANSI strings.Builder
	if err := Render(&bottomANSI, bottom, 1, Options{NoLinePrefix: true}); err != nil {
		t.Fatalf("Render bottom-only: %v", err)
	}
	bottomCell := parseRenderCells(t, bottomANSI.String(), 1, 1)[0][0]
	if want := (ansitest.Cell{Rune: '▄', FG: ansitest.RGB(5, 20, 100)}); bottomCell != want {
		t.Fatalf("bottom cell = %+v, want %+v", bottomCell, want)
	}
	if got := RenderToImage(bottom).RGBAAt(0, 1); got != (color.RGBA{R: 5, G: 20, B: 100, A: 255}) {
		t.Fatalf("bottom painted pixel = %#v, want opaque premultiplied ANSI color", got)
	}
}

func TestRenderToImage_ANSIBackgroundIsOpaque(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	img.SetRGBA(0, 1, color.RGBA{B: 255, A: 255})

	var ansi strings.Builder
	if err := Render(&ansi, img, 1, Options{NoLinePrefix: true}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	cell := parseRenderCells(t, ansi.String(), 1, 1)[0][0]
	if want := (ansitest.Cell{Rune: '▀', FG: ansitest.RGB(255, 0, 0), BG: ansitest.RGB(0, 0, 255)}); cell != want {
		t.Fatalf("two-colour cell = %+v, want %+v", cell, want)
	}
	got := RenderToImage(img)
	if got.RGBAAt(0, 0) != (color.RGBA{R: 255, A: 255}) || got.RGBAAt(0, 1) != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("opaque two-color reconstruction = %v/%v", got.RGBAAt(0, 0), got.RGBAAt(0, 1))
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

func parseRenderCells(t *testing.T, out string, wantRows, wantCols int) [][]ansitest.Cell {
	t.Helper()
	rows, err := ansitest.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != wantRows {
		t.Fatalf("render rows = %d, want %d", len(rows), wantRows)
	}
	for y, row := range rows {
		if len(row) != wantCols {
			t.Fatalf("row %d has %d cells, want %d", y, len(row), wantCols)
		}
	}
	return rows
}

func TestRenderColourStateDoesNotLeakAcrossRows(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for x := range 2 {
		img.SetRGBA(x, 0, rgba(255, 0, 0, 255))
		img.SetRGBA(x, 1, rgba(0, 0, 255, 255))
		img.SetRGBA(x, 2, rgba(0, 255, 0, 255))
	}
	var out strings.Builder
	if err := Render(&out, img, 2, Options{}); err != nil {
		t.Fatal(err)
	}
	rows := parseRenderCells(t, out.String(), 2, 2)
	for x, cell := range rows[1] {
		want := ansitest.Cell{Rune: '▀', FG: ansitest.RGB(0, 255, 0)}
		if cell != want {
			t.Errorf("partial bottom cell (%d,1) = %+v, want %+v", x, cell, want)
		}
	}
}
