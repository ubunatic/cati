package metrics

import (
	"image"
	"image/color"
	"math"
	"testing"
)

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

// ── Luma ──────────────────────────────────────────────────────────────────────

func TestLuma(t *testing.T) {
	tests := []struct {
		name string
		c    color.Color
		want float64
	}{
		{"black", rgba(0, 0, 0, 255), 0},
		{"white", rgba(255, 255, 255, 255), 1.0},
		{"red", rgba(255, 0, 0, 255), 0.2126},
		{"green", rgba(0, 255, 0, 255), 0.7152},
		{"blue", rgba(0, 0, 255, 255), 0.0722},
		{"gray", rgba(128, 128, 128, 255), 0.502}, // BT.709 gray ≈ 0.502
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := luma(tc.c)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("luma(%v) = %.4f, want %.4f", tc.c, got, tc.want)
			}
		})
	}
}

// ── LumaGrid ──────────────────────────────────────────────────────────────────

func TestLumaGrid_Dims(t *testing.T) {
	img := solidImage(4, 3, rgba(255, 0, 0, 255))
	g := LumaGrid(img)
	if len(g) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(g))
	}
	for i, row := range g {
		if len(row) != 4 {
			t.Fatalf("row %d: expected 4 cols, got %d", i, len(row))
		}
	}
	// All pixels are red → luma = 0.2126
	for y := range g {
		for x := range g[y] {
			if math.Abs(g[y][x]-0.2126) > 0.001 {
				t.Errorf("g[%d][%d] = %.4f, want 0.2126", y, x, g[y][x])
			}
		}
	}
}

// ── SobelGrid ─────────────────────────────────────────────────────────────────

func TestSobelGrid_Uniform(t *testing.T) {
	g := [][]float64{
		{0.5, 0.5, 0.5, 0.5},
		{0.5, 0.5, 0.5, 0.5},
		{0.5, 0.5, 0.5, 0.5},
	}
	s := SobelGrid(g)
	for y := range s {
		for x := range s[y] {
			if s[y][x] != 0 {
				t.Errorf("uniform: s[%d][%d] = %.4f, want 0", y, x, s[y][x])
			}
		}
	}
}

func TestSobelGrid_StepEdge(t *testing.T) {
	// 10x6 image: first 4 cols black (0), cols 4-9 white (1) → vertical edge at col 4.
	g := make([][]float64, 6)
	for y := range g {
		g[y] = make([]float64, 10)
		for x := 4; x < 10; x++ {
			g[y][x] = 1.0
		}
	}
	s := SobelGrid(g)
	// Position (2, 2) is 2 pixels left of edge → kernel cols 1-3, all black → 0.
	if s[2][2] != 0 {
		t.Errorf("pre-edge[2][2] = %.4f, want 0", s[2][2])
	}
	// Position (2, 4) is at the edge → should have positive gradient.
	if s[2][4] <= 0 {
		t.Errorf("edge[2][4] = %.4f, want > 0", s[2][4])
	}
}

// ── BlockinessFromGrids ────────────────────────────────────────────────────────

func TestBlockinessFromGrids_Uniform(t *testing.T) {
	ref := [][]float64{
		{0, 0, 0},
		{0, 0, 0},
		{0, 0, 0},
	}
	rend := [][]float64{
		{0, 0, 0},
		{0, 0, 0},
		{0, 0, 0},
	}
	score := BlockinessFromGrids(ref, rend, true, 2)
	if score != 1.0 {
		t.Errorf("uniform: got %.4f, want 1.0", score)
	}
}

func TestBlockinessFromGrids_Halfblock(t *testing.T) {
	// Halfblock checks only horizontal boundaries at step intervals.
	// With step=2 and a 7-wide grid, horizontal checks at rows 2, 4.
	// Place excess edge gradient at row 2 (the first checked row).
	ref := [][]float64{
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
	}
	rend := [][]float64{
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 1, 1, 1, 1, 1, 0}, // row 2 has excess block-edge gradient
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
	}
	score := BlockinessFromGrids(ref, rend, false, 2)
	if score >= 1.0 {
		t.Errorf("with block edges: got %.4f, want < 1.0", score)
	}
}

func TestBlockinessFromGrids_QuadVertical(t *testing.T) {
	ref := [][]float64{
		{0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0},
	}
	rend := [][]float64{
		{0, 0, 0, 0, 0},
		{0, 0, 1, 0, 0},
		{0, 0, 1, 0, 0},
		{0, 0, 1, 0, 0},
		{0, 0, 0, 0, 0},
	}
	// Quad also checks vertical boundaries.
	scoreHalf := BlockinessFromGrids(ref, rend, false, 2) // halfblock: only horizontal
	scoreQuad := BlockinessFromGrids(ref, rend, true, 2)  // quad: horizontal + vertical
	if scoreHalf > scoreQuad {
		t.Errorf("quad (%.4f) should detect more blockiness than halfblock (%.4f)", scoreQuad, scoreHalf)
	}
}

// ── EdgeContinuityFromGrids ────────────────────────────────────────────────────

func TestEdgeContinuityFromGrids_NoEdges(t *testing.T) {
	ref := [][]float64{
		{0, 0, 0},
		{0, 0, 0},
		{0, 0, 0},
	}
	rend := [][]float64{
		{0, 0, 0},
		{0, 0, 0},
		{0, 0, 0},
	}
	score := EdgeContinuityFromGrids(ref, rend)
	if score != 1.0 {
		t.Errorf("no edges: got %.4f, want 1.0", score)
	}
}

func TestEdgeContinuityFromGrids_Perfect(t *testing.T) {
	// Reference has one strong edge pixel, rendered matches exactly.
	ref := [][]float64{
		{0, 0, 0, 0, 0},
		{0, 0.5, 0, 0, 0},
		{0, 0, 0, 0, 0},
	}
	rend := [][]float64{
		{0, 0, 0, 0, 0},
		{0, 0.5, 0, 0, 0},
		{0, 0, 0, 0, 0},
	}
	score := EdgeContinuityFromGrids(ref, rend)
	if math.Abs(score-1.0) > 0.001 {
		t.Errorf("perfect match: got %.4f, want 1.0", score)
	}
}

// ── SSIMLuminance ──────────────────────────────────────────────────────────────

func TestSSIMLuminance_Identical(t *testing.T) {
	img := solidImage(16, 16, rgba(128, 128, 128, 255))
	ssim := SSIMLuminance(img, img)
	if ssim != 1.0 {
		t.Errorf("identical: got %.4f, want 1.0", ssim)
	}
}

func TestSSIMLuminance_Different(t *testing.T) {
	a := solidImage(16, 16, rgba(0, 0, 0, 255))
	b := solidImage(16, 16, rgba(255, 255, 255, 255))
	ssim := SSIMLuminance(a, b)
	if ssim >= 1.0 {
		t.Errorf("different: got %.4f, want < 1.0", ssim)
	}
}

func TestSSIMLuminance_TooSmall(t *testing.T) {
	a := solidImage(4, 4, rgba(128, 128, 128, 255))
	b := solidImage(4, 4, rgba(0, 0, 0, 255))
	ssim := SSIMLuminance(a, b)
	if ssim != 1.0 {
		t.Errorf("too small: got %.4f, want 1.0", ssim)
	}
}

// ── BoxDownscale ────────────────────────────────────────────────────────────────

func TestBoxDownscale_Dims(t *testing.T) {
	img := solidImage(8, 8, rgba(255, 0, 0, 255))
	dst := BoxDownscale(img, 4, 4)
	b := dst.Bounds()
	if b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("dims: got %dx%d, want 4x4", b.Dx(), b.Dy())
	}
}

func TestBoxDownscale_Average(t *testing.T) {
	// 2x2 image: top-left red, others black → 1x1 downscale should be dark red.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, rgba(255, 0, 0, 255))
	img.Set(1, 0, rgba(0, 0, 0, 255))
	img.Set(0, 1, rgba(0, 0, 0, 255))
	img.Set(1, 1, rgba(0, 0, 0, 255))

	dst := BoxDownscale(img, 1, 1)
	r, g, b, _ := dst.At(0, 0).RGBA()
	// Mean of 4 pixels: red component = (255 + 0 + 0 + 0) / 4 ≈ 63
	r8 := r / 257
	if r8 != 63 || g != 0 || b != 0 {
		t.Errorf("avg: got R=%d G=%d B=%d, want R=63 G=0 B=0", r8, g/257, b/257)
	}
}

// ── PyramidDownscale ────────────────────────────────────────────────────────────

func TestPyramidDownscale_Dims(t *testing.T) {
	img := solidImage(100, 80, rgba(255, 0, 0, 255))
	dst := PyramidDownscale(img, 25, 20)
	b := dst.Bounds()
	if b.Dx() != 25 || b.Dy() != 20 {
		t.Errorf("dims: got %dx%d, want 25x20", b.Dx(), b.Dy())
	}
}

func TestPyramidDownscale_NoOp(t *testing.T) {
	img := solidImage(8, 8, rgba(128, 128, 128, 255))
	dst := PyramidDownscale(img, 8, 8)
	b := dst.Bounds()
	if b.Dx() != 8 || b.Dy() != 8 {
		t.Errorf("same size: got %dx%d, want 8x8", b.Dx(), b.Dy())
	}
}

func TestPyramidDownscale_Large(t *testing.T) {
	img := solidImage(400, 300, rgba(255, 0, 0, 255))
	dst := PyramidDownscale(img, 10, 8)
	b := dst.Bounds()
	if b.Dx() != 10 || b.Dy() != 8 {
		t.Errorf("large: got %dx%d, want 10x8", b.Dx(), b.Dy())
	}
}

// ── QualityGridDims ────────────────────────────────────────────────────────────

func TestQualityGridDims(t *testing.T) {
	tests := []struct {
		name                 string
		vpW, vpH             int
		pixPerCol, pixPerRow int
		k                    int
		wantW, wantH         int
	}{
		{"halfblock, K=4, 80x40", 80, 40, 1, 2, 4, 320, 80},
		{"quad, K=4, 80x40", 80, 40, 2, 2, 4, 160, 80},
		{"sparkline, K=4, 320x320", 320, 320, 4, 8, 4, 320, 160},
		{"halfblock, K=2, 40x20", 40, 20, 1, 2, 2, 80, 20},
		{"quad, K=2, 40x20", 40, 20, 2, 2, 2, 40, 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gw, gh := QualityGridDims(tc.vpW, tc.vpH, tc.pixPerCol, tc.pixPerRow, tc.k)
			if gw != tc.wantW || gh != tc.wantH {
				t.Errorf("got %dx%d, want %dx%d", gw, gh, tc.wantW, tc.wantH)
			}
		})
	}
}

// ── LumaGrid with SobelGrid integration ────────────────────────────────────────

func TestLumaGridThenSobelGrid(t *testing.T) {
	img := solidImage(8, 8, rgba(0, 0, 0, 255))
	g := LumaGrid(img)
	s := SobelGrid(g)
	for y := range s {
		for x := range s[y] {
			if s[y][x] != 0 {
				t.Errorf("luma+Sobel uniform: s[%d][%d] = %.4f", y, x, g[y][x])
			}
		}
	}
}

// ── RenderQuality struct ───────────────────────────────────────────────────────

func TestRenderQuality_ZeroValue(t *testing.T) {
	var q RenderQuality
	if q.SSIM != 0 || q.Blockiness != 0 || q.EdgeCont != 0 {
		t.Error("zero RenderQuality should have all fields zero")
	}
}
