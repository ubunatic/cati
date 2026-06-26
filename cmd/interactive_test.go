package cmd

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/input"
	"codeberg.org/ubunatic/cati/internal/quadblock"
)

// ── interactive (error paths) ─────────────────────────────────────────────────

func TestInteractive_MissingFile(t *testing.T) {
	err := interactive("nonexistent.png", 0, 0, renderCfg{}, false)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// ── fitPixelDims ─────────────────────────────────────────────────────────────

func TestFitPixelDims(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		maxW, maxH   int
		wantW, wantH int
	}{
		{"fits exactly", 100, 50, 100, 100, 100, 50},
		{"smaller than max", 50, 25, 100, 100, 50, 25},
		{"width constraint", 200, 100, 100, 200, 100, 50},
		{"height constraint", 200, 100, 400, 50, 100, 50},
		{"both, width tighter", 200, 100, 60, 60, 60, 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotW, gotH := imgutil.FitPixelDims(tc.srcW, tc.srcH, tc.maxW, tc.maxH)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("imgutil.FitPixelDims(%d,%d,%d,%d) = %dx%d, want %dx%d",
					tc.srcW, tc.srcH, tc.maxW, tc.maxH, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

// ── mouse parsing (via input.Spec) ───────────────────────────────────────────

func TestParseSGRMouse(t *testing.T) {
	s := input.DefaultSpec()
	tests := []struct {
		name             string
		input            string
		wantBtn          int
		wantCol, wantRow int
		wantRelease      bool
		wantOK           bool
	}{
		// Scroll
		{"scroll up at 10,5", "\x1b[<64;10;5M", 64, 10, 5, false, true},
		{"scroll down at 20,3", "\x1b[<65;20;3M", 65, 20, 3, false, true},
		// Clicks
		{"left press at 1,1", "\x1b[<0;1;1M", 0, 1, 1, false, true},
		{"left release at 1,1", "\x1b[<0;1;1m", 0, 1, 1, true, true},
		// Drag (left button held + motion: btn = 0 + 32 = 32)
		{"left drag at 15,7", "\x1b[<32;15;7M", 32, 15, 7, false, true},
		// Non-mouse
		{"plain key ESC", "\x1b", 0, 0, 0, false, false},
		{"arrow key up", "\x1b[A", 0, 0, 0, false, false},
		{"plain plus", "+", 0, 0, 0, false, false},
		{"empty string", "", 0, 0, 0, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, ok := s.ParseMouse(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if m.Btn != tc.wantBtn || m.Col != tc.wantCol || m.Row != tc.wantRow || m.Release != tc.wantRelease {
				t.Errorf("got btn=%d col=%d row=%d release=%v, want btn=%d col=%d row=%d release=%v",
					m.Btn, m.Col, m.Row, m.Release, tc.wantBtn, tc.wantCol, tc.wantRow, tc.wantRelease)
			}
		})
	}
}

// ── SGR button predicates (via input.MouseEvent methods) ─────────────────────

func TestSGRPredicates(t *testing.T) {
	s := input.DefaultSpec()
	type row struct {
		tok      string
		isScroll bool
		isDrag   bool
		button   int
	}
	tests := []row{
		{"\x1b[<0;1;1M", false, false, 0},    // left click
		{"\x1b[<1;1;1M", false, false, 1},    // middle click
		{"\x1b[<2;1;1M", false, false, 2},    // right click
		{"\x1b[<32;1;1M", false, true, 0},    // left drag
		{"\x1b[<33;1;1M", false, true, 1},    // middle drag
		{"\x1b[<34;1;1M", false, true, 2},    // right drag
		{"\x1b[<64;1;1M", true, false, 0},    // scroll up
		{"\x1b[<65;1;1M", true, false, 1},    // scroll down
		{"\x1b[<68;1;1M", true, false, 0},    // scroll up + shift
		{"\x1b[<36;1;1M", false, true, 0},    // left drag + shift (32+4)
	}
	for _, tc := range tests {
		m, ok := s.ParseMouse(tc.tok)
		if !ok {
			t.Errorf("ParseMouse(%q) failed", tc.tok)
			continue
		}
		if got := m.IsScroll(); got != tc.isScroll {
			t.Errorf("IsScroll(%q) = %v, want %v", tc.tok, got, tc.isScroll)
		}
		if got := m.IsDrag(); got != tc.isDrag {
			t.Errorf("IsDrag(%q) = %v, want %v", tc.tok, got, tc.isDrag)
		}
		if got := m.Button; got != tc.button {
			t.Errorf("Button(%q) = %v, want %v", tc.tok, got, tc.button)
		}
	}
}

// ── dragState pan math ────────────────────────────────────────────────────────

func TestDragPanMath(t *testing.T) {
	// Start drag at terminal column 40, row 10, with pan (20, 10).
	drag := dragState{
		active:    true,
		startCol:  40,
		startRow:  10,
		startPanX: 20,
		startPanY: 10,
	}
	state := viewState{zoom: 2.0, panX: 20, panY: 10}

	// Simulate drag 10 cols right, 3 rows down.
	c, r := 50, 13
	state.panX = drag.startPanX - (c - drag.startCol)
	state.panY = drag.startPanY - (r-drag.startRow)*2

	// Dragging right (image moves right → panX decreases).
	if state.panX != 10 {
		t.Errorf("panX = %d, want 10", state.panX)
	}
	// Dragging down 3 rows = 6 pixel rows → panY decreases.
	if state.panY != 4 {
		t.Errorf("panY = %d, want 4", state.panY)
	}
}

// ── zoomAtCursor ─────────────────────────────────────────────────────────────

func TestZoomAtCursor(t *testing.T) {
	// Image 100px wide, terminal 80 cols, zoom 1.0, pan 0.
	// Cursor at col 40 (centre).
	// After doubling zoom, pixel 40 of the 80px scaled image → pixel 80 of 160px.
	// New panX should be 80 - 40 = 40.
	state := viewState{zoom: 1.0, panX: 0, panY: 0}
	zoomAtCursor(&state, 2.0, 40, 0)
	if state.panX != 40 {
		t.Errorf("panX = %d, want 40", state.panX)
	}
	if state.zoom != 2.0 {
		t.Errorf("zoom = %f, want 2.0", state.zoom)
	}
}

// ── cropImage ────────────────────────────────────────────────────────────────

func TestCropImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	red := color.RGBA{R: 255, A: 255}
	img.Set(5, 5, red)

	crop := imgutil.CropImage(img, 4, 4, 4, 4)
	b := crop.Bounds()
	if b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("crop size = %dx%d, want 4x4", b.Dx(), b.Dy())
	}
	// Pixel (1,1) in the crop should be red (original (5,5)).
	got := crop.At(b.Min.X+1, b.Min.Y+1)
	r, _, _, _ := got.RGBA()
	if r == 0 {
		t.Errorf("expected red pixel at crop (1,1), got %v", got)
	}
}

// ── maxZoom ──────────────────────────────────────────────────────────────────

func TestMaxZoom(t *testing.T) {
	tests := []struct {
		name                string
		srcW, srcH          int
		termCols, termRows  int
		mode                renderMode
		want                float64
	}{
		{
			name:     "halfblock image larger than viewport",
			srcW:     1920, srcH: 1080,
			termCols: 80, termRows: 40,
			mode: modeHalfblock,
			want: 24.0,
		},
		{
			name:     "quad image larger than viewport",
			srcW:     1920, srcH: 1080,
			termCols: 80, termRows: 40,
			mode: modeQuad,
			want: 24.0,
		},
		{
			name:     "halfblock image fits exactly",
			srcW:     80, srcH: 40,
			termCols: 80, termRows: 40,
			mode: modeHalfblock,
			want: 1.0,
		},
		{
			name:     "halfblock small image",
			srcW:     40, srcH: 20,
			termCols: 80, termRows: 40,
			mode: modeHalfblock,
			want: 1.0,
		},
		{
			name:     "zero srcW",
			srcW:     0, srcH: 100,
			termCols: 80, termRows: 40,
			mode: modeHalfblock,
			want: 1.0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := maxZoom(tc.srcW, tc.srcH, tc.termCols, tc.termRows, tc.mode)
			if got != tc.want {
				t.Errorf("maxZoom(%d,%d,%d,%d,%v) = %v, want %v",
					tc.srcW, tc.srcH, tc.termCols, tc.termRows, tc.mode, got, tc.want)
			}
		})
	}
}

// TestMaxZoomOneToOne verifies that at max zoom with halfblock, the viewport
// image is a 1:1 crop of the source (each viewport pixel = one source pixel).
func TestMaxZoomOneToOne(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 6, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 6; x++ {
			src.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 40),
				G: uint8(y * 60),
				B: uint8(x + y*10),
				A: 255,
			})
		}
	}

	const termCols, termRows = 10, 4
	zoom := maxZoom(6, 4, termCols, termRows, modeHalfblock)
	if zoom != 1.0 {
		t.Fatalf("maxZoom for small image = %v, want 1.0", zoom)
	}

	state := viewState{zoom: zoom, panX: 0, panY: 0}
	rc := renderCfg{id: 4} // halfblock
	vp := buildViewport(src, &state, termCols, termRows, rc)
	b := vp.Bounds()

	if b.Dx() != 6 || b.Dy() != 4 {
		t.Fatalf("viewport size = %dx%d, want 6×4", b.Dx(), b.Dy())
	}

	for y := 0; y < 4; y++ {
		for x := 0; x < 6; x++ {
			got := vp.At(x, y)
			want := src.At(x, y)
			r1, g1, b1, a1 := got.RGBA()
			r2, g2, b2, a2 := want.RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Errorf("vp.At(%d,%d) = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
					x, y, r1, g1, b1, a1, r2, g2, b2, a2)
			}
		}
	}
}

// TestMaxZoomQuadConvergence verifies that all quad render modes produce
// byte-identical ANSI output when rendering at max zoom. At this zoom every
// terminal cell covers ≤ 1 source column × 2 source rows: with a horizontally
// NN-upscaled viewport each 2×2 block has ≤ 2 unique colors making all quad
// algorithms choose the same color pair.
func TestMaxZoomQuadConvergence(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	// Row 0-1: red, row 2-3: blue — each 2×2 block is either solid red or
	// solid blue, giving all quad algorithms an unambiguous choice.
	for x := 0; x < 4; x++ {
		src.SetNRGBA(x, 0, color.NRGBA{R: 255, A: 255})
		src.SetNRGBA(x, 1, color.NRGBA{R: 255, A: 255})
		src.SetNRGBA(x, 2, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
		src.SetNRGBA(x, 3, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
	}

	const termCols, termRows = 4, 2
	zoom := maxZoom(4, 4, termCols, termRows, modeQuad)
	if zoom != 1.0 {
		t.Fatalf("maxZoom for small image = %v, want 1.0", zoom)
	}

	state := viewState{zoom: zoom, panX: 0, panY: 0}

	// Collect all active quad modes and their outputs.
	type modeResult struct {
		name string
		out  string
	}
	var results []modeResult

	for _, m := range renderModes {
		if !m.cfg.mode.useQuad() {
			continue
		}
		vp := buildViewport(src, &state, termCols, termRows, m.cfg)
		var buf bytes.Buffer
		if err := quadblock.RenderOpts(&buf, vp, m.cfg.quadOpts); err != nil {
			t.Fatalf("RenderOpts(%s): %v", m.name, err)
		}
		results = append(results, modeResult{m.name, buf.String()})
	}

	if len(results) < 2 {
		t.Skip("need at least 2 quad modes for convergence test")
	}

	ref := results[0].out
	for _, r := range results[1:] {
		if r.out != ref {
			t.Errorf("quad mode %q differs from %q\n%s", r.name, results[0].name, diffTermOutput(ref, r.out))
		}
	}
}

// diffTermOutput shows the first differing position between two ANSI strings.
func diffTermOutput(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			ctx := 8
			start := i - ctx
			if start < 0 {
				start = 0
			}
			end := i + ctx
			if end > minLen {
				end = minLen
			}
			return fmt.Sprintf("first diff at byte %d:\n  a: …%q…\n  b: …%q…",
				i, a[start:end], b[start:end])
		}
	}
	if len(a) != len(b) {
		return fmt.Sprintf("length diff: a=%d b=%d", len(a), len(b))
	}
	return "identical"
}

// ── zoomSteps / stepIdx ──────────────────────────────────────────────────────

func TestZoomSteps(t *testing.T) {
	tests := []struct {
		name    string
		mz      float64
		srcW    int
		wantLen int
		want0   float64
		wantN   float64
	}{
		{"maxZoom=1, srcW=5 → 5 steps", 1.0, 5, 5, 1.0, 0.2},
		{"maxZoom=2, srcW=10 → 10 steps", 2.0, 10, 10, 2.0, 0.2},
		{"maxZoom=24, srcW=48 → 48 steps", 24.0, 48, 48, 24.0, 0.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := zoomSteps(tc.mz, tc.srcW)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			if got[0] != tc.want0 {
				t.Errorf("steps[0] = %v, want %v", got[0], tc.want0)
			}
			if abs(got[len(got)-1]-tc.wantN) > 1e-9 {
				t.Errorf("steps[last] = %v, want %v", got[len(got)-1], tc.wantN)
			}
			for i := 1; i < len(got); i++ {
				if got[i] >= got[i-1] {
					t.Errorf("steps[%d]=%v ≥ steps[%d]=%v (expected descending)", i, got[i], i-1, got[i-1])
				}
			}
		})
	}
}

func TestStepIdx(t *testing.T) {
	steps := zoomSteps(24.0, 48) // mz=24, srcW=48 → 48 steps
	tests := []struct {
		name string
		zoom float64
		want int
	}{
		{"exact max = 24 → index 0", 24.0, 0},
		{"between 24 and 12 → index 1", 18.0, 1},
		{"exact 12 → index 1", 12.0, 1},
		{"exact 8 → index 2", 8.0, 2},
		{"exact 4.8 → index 4", 4.8, 4},
		{"above max → clamped to 0", 100.0, 0},
		{"below min → clamped to last", 0.1, len(steps) - 1},
		{"exact 1 → index K-1 = 23", 1.0, 23},
		{"exact 0.5 → last index", 0.5, len(steps) - 1},
		{"zero zoom → last index", 0.0, len(steps) - 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stepIdx(tc.zoom, steps)
			if got != tc.want {
				t.Errorf("stepIdx(%v, steps[0..%d]) = %d, want %d", tc.zoom, len(steps)-1, got, tc.want)
			}
		})
	}
}

func TestZoomSequenceRoundTrip(t *testing.T) {
	const mz, srcW = 24.0, 48
	steps := zoomSteps(mz, srcW)
	for i, z := range steps {
		got := stepIdx(z, steps)
		if got != i {
			t.Errorf("step %d (zoom=%v): stepIdx returned %d", i, z, got)
		}
	}
}

// TestZoomLevelPixels verifies that at each zoom step the viewport image
// correctly maps source pixels to viewport pixels.
func TestZoomLevelPixels(t *testing.T) {
	// 20×10 source with unique colors: R = x*257, G = y*257, B = x+y
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 10),
				G: uint8(y * 20),
				B: uint8(x + y),
				A: 255,
			})
		}
	}

	const termCols, termRows = 10, 5
	mz := maxZoom(20, 10, termCols, termRows, modeHalfblock)
	steps := zoomSteps(mz, 20)

	// Get the fit dimensions that buildViewport uses internally.
	fw, fh := imgutil.FitPixelDims(20, 10, termCols, modeHalfblock.pixRows(termRows))

	for i, zoom := range steps {
		state := viewState{zoom: zoom, panX: 0, panY: 0}
		rc := renderCfg{id: 4}
		vp := buildViewport(src, &state, termCols, termRows, rc)
		b := vp.Bounds()

		// Compute the effective scaled width the same way buildViewport does.
		sw := max(1, int(math.Round(float64(fw)*zoom)))

		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				sx := x * 20 / sw
				sy := y * 10 / (max(1, int(math.Round(float64(fh)*zoom))))
				if sx >= 20 || sy >= 10 {
					continue
				}
				got := vp.At(x, y)
				want := src.At(sx, sy)
				r1, g1, b1, a1 := got.RGBA()
				r2, g2, b2, a2 := want.RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Errorf("step %d (zoom=%v): vp.At(%d,%d) = (%d,%d,%d,%d), want src.At(%d,%d) = (%d,%d,%d,%d)",
						i, zoom, x, y, r1, g1, b1, a1, sx, sy, r2, g2, b2, a2)
				}
			}
		}
	}
}

// abs returns the absolute value of f.
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

