package cmd

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"math"
	"testing"

	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/input"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sextant"
	"codeberg.org/ubunatic/cati/internal/sparkline"
	"codeberg.org/ubunatic/cati/internal/viewgeom"
	spec "codeberg.org/ubunatic/cati/spec"
)

// ── interactive (error paths) ─────────────────────────────────────────────────

func TestInteractive_MissingFile(t *testing.T) {
	err := interactive("nonexistent.png", 0, 0, renderCfg{}, false, "")
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
		{"\x1b[<0;1;1M", false, false, 0}, // left click
		{"\x1b[<1;1;1M", false, false, 1}, // middle click
		{"\x1b[<2;1;1M", false, false, 2}, // right click
		{"\x1b[<32;1;1M", false, true, 0}, // left drag
		{"\x1b[<33;1;1M", false, true, 1}, // middle drag
		{"\x1b[<34;1;1M", false, true, 2}, // right drag
		{"\x1b[<64;1;1M", true, false, 0}, // scroll up
		{"\x1b[<65;1;1M", true, false, 1}, // scroll down
		{"\x1b[<68;1;1M", true, false, 0}, // scroll up + shift
		{"\x1b[<36;1;1M", false, true, 0}, // left drag + shift (32+4)
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
	drag := viewgeom.NewPanAnchor(40, 10, 20, 10)
	state := viewState{zoom: 2.0, panX: 20, panY: 10}

	// Simulate drag 10 cols right, 3 rows down.
	c, r := 50, 13
	state.panX, state.panY = modeHalfblock.viewSpec().PanFromAnchor(drag, c, r)

	// Dragging right (image moves right → panX decreases).
	if state.panX != 10 {
		t.Errorf("panX = %d, want 10", state.panX)
	}
	// Dragging down 3 rows = 6 pixel rows → panY decreases.
	if state.panY != 4 {
		t.Errorf("panY = %d, want 4", state.panY)
	}
}

func TestPanMathUsesModeFootprint(t *testing.T) {
	tests := []struct {
		name  string
		mode  renderMode
		wantX int
		wantY int
	}{
		{"halfblock", modeHalfblock, 10, 4},
		{"quad", modeQuad, 0, 4},
		{"spark", modeSpark, -20, -14},
		{"sextant", modeSextant, 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			geom := tc.mode.viewSpec()
			anchor := viewgeom.NewPanAnchor(40, 10, 20, 10)
			gotX, gotY := geom.PanFromAnchor(anchor, 50, 13)
			if gotX != tc.wantX || gotY != tc.wantY {
				t.Fatalf("PanFromAnchor = %d,%d, want %d,%d", gotX, gotY, tc.wantX, tc.wantY)
			}
		})
	}
}

// ── zoomAtCursor ─────────────────────────────────────────────────────────────

func TestZoomAtCursor(t *testing.T) {
	// Image 100px wide, terminal 80 cols, zoom 1.0, pan 0.
	// Cursor at col 40 (centre).
	// After doubling zoom, pixel 40 of the 80px scaled image → pixel 80 of 160px.
	// New panX should be 80 - 40 = 40.
	state := viewState{zoom: 1.0, panX: 0, panY: 0}
	zoomAtCursor(&state, 2.0, 40, 0, modeHalfblock)
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

func TestResolveViewerTermSizeTreatsHeightAsImageRows(t *testing.T) {
	autoCols, autoRows := resolveTermSize(0, 0)
	width := min(80, autoCols)
	imageRows := min(22, max(1, autoRows-viewerChromeRows))
	cols, rows := resolveViewerTermSize(width, imageRows)
	if cols != width {
		t.Fatalf("cols = %d, want %d", cols, width)
	}
	wantRows := min(autoRows, imageRows+viewerChromeRows)
	if rows != wantRows {
		t.Fatalf("rows = %d, want image rows %d + chrome rows %d", rows, imageRows, viewerChromeRows)
	}

	vc := newViewerCore("image_viewer", width, imageRows, renderCfg{}, false, input.DefaultSpec(), nil, nil, nil, nil, nil)
	if got := vc.viewRows(); got != imageRows {
		t.Fatalf("viewRows = %d, want explicit image height %d", got, imageRows)
	}
}

func TestResolveViewerTermSizeClampsOversizedExplicitDimensions(t *testing.T) {
	autoCols, autoRows := resolveTermSize(0, 0)
	cols, rows := resolveViewerTermSize(autoCols+1000, autoRows+1000)
	if cols != autoCols {
		t.Fatalf("cols = %d, want terminal width %d", cols, autoCols)
	}
	if rows != autoRows {
		t.Fatalf("rows = %d, want terminal height %d", rows, autoRows)
	}
	if got := (&viewerCore{termRows: rows}).viewRows(); got != max(1, autoRows-viewerChromeRows) {
		t.Fatalf("viewRows = %d, want clamped image rows %d", got, max(1, autoRows-viewerChromeRows))
	}
}

func TestStaticAndInteractiveExplicitHeightUseSameImageRows(t *testing.T) {
	src := horizontalGradientNRGBA(32, 32)
	rc := renderCfg{}
	const width, imageRows = 20, 7

	static := prepareRenderedImage(src, nil, width, imageRows, rc, "")
	state := viewState{zoom: initialZoomRatio("", src.Bounds().Dx(), src.Bounds().Dy(), width, imageRows, rc.mode)}
	interactive := buildViewport(src, &state, width, imageRows, rc)

	var staticOut, interactiveOut bytes.Buffer
	if err := rc.render(&staticOut, static); err != nil {
		t.Fatalf("static render: %v", err)
	}
	if err := rc.render(&interactiveOut, interactive); err != nil {
		t.Fatalf("interactive render: %v", err)
	}
	if staticOut.String() != interactiveOut.String() {
		t.Fatalf("static and interactive image output differ\n%s", diffTermOutput(staticOut.String(), interactiveOut.String()))
	}
}

// ── maxZoom ──────────────────────────────────────────────────────────────────

func TestMaxZoom(t *testing.T) {
	tests := []struct {
		name               string
		srcW, srcH         int
		termCols, termRows int
		mode               renderMode
		want               float64
	}{
		{
			name: "halfblock image larger than viewport",
			srcW: 1920, srcH: 1080,
			termCols: 80, termRows: 40,
			mode: modeHalfblock,
			want: 24.0,
		},
		{
			name: "quad image larger than viewport",
			srcW: 1920, srcH: 1080,
			termCols: 80, termRows: 40,
			mode: modeQuad,
			want: 24.0,
		},
		{
			name: "halfblock image fits exactly",
			srcW: 80, srcH: 40,
			termCols: 80, termRows: 40,
			mode: modeHalfblock,
			want: 1.0,
		},
		{
			name: "halfblock small image",
			srcW: 40, srcH: 20,
			termCols: 80, termRows: 40,
			mode: modeHalfblock,
			want: 1.0,
		},
		{
			name: "zero srcW",
			srcW: 0, srcH: 100,
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

func TestEllipsizeRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"fits", "cat.png", 8, "cat.png"},
		{"truncate with ellipsis", "very_long_name.png", 10, "very_lo..."},
		{"tight budget", "abcdef", 3, "abc"},
		{"empty budget", "abcdef", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ellipsizeRunes(tc.in, tc.max); got != tc.want {
				t.Fatalf("ellipsizeRunes(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestViewerHintVarsFitsWidth(t *testing.T) {
	meta := MediaMeta{
		Name:     "very_long_filename_for_the_viewer_hint_bar.png",
		SrcW:     "1920",
		SrcH:     "1080",
		DispW:    "80",
		DispH:    "24",
		DispMode: "half",
	}
	hint := "{ meta.name_short | dim }  { render_mode | dim }  { zoom_level | dim }  S:{ ssim | dim }  { meta.src_res | dim }  { meta.disp_res | dim }  { last_key | dim }"
	vars := viewerHintVars(meta, 80, hint, map[string]string{
		"last_key":    "q",
		"render_mode": "halfblock",
		"zoom_level":  "1:1",
		"ssim":        "0.999",
	})
	if got := tplWidth(hint, vars); got > 78 {
		t.Fatalf("viewer hint width = %d, want <= 78 (termCols-2)", got)
	}
	if got := vars["meta.name_short"]; got == "" {
		t.Fatal("viewer hint missing shortened filename")
	}
	if got := vars["meta.name_short"]; got == meta.Name && len([]rune(meta.Name)) > 0 {
		t.Fatal("viewer hint did not shorten an overlong filename")
	}
}

func TestRenderModeZeroValueIsHalfblockAndCyclesToQuad(t *testing.T) {
	rc := renderCfg{}
	if got := rcModeName(rc); got != "halfblock" {
		t.Fatalf("rcModeName(zero) = %q, want halfblock", got)
	}
	next, name := cycleRenderCfg(rc)
	if name != "quad/splithalf" {
		t.Fatalf("cycleRenderCfg(zero) name = %q, want quad/splithalf", name)
	}
	if next.mode != modeQuad || !next.quadOpts.SplitHalf {
		t.Fatalf("cycleRenderCfg(zero) = %#v, want splithalf quad", next)
	}
}

func TestCanonicalRenderCfgMapsStartupConfigsToCycleIDs(t *testing.T) {
	tests := []struct {
		name string
		in   renderCfg
		want string
	}{
		{"zero halfblock", renderCfg{}, "halfblock"},
		{"splithalf quad", renderCfg{mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}, "quad/splithalf"},
		{"edge snap quad", renderCfg{mode: modeQuad, quadOpts: quadblock.Options{EdgeSnap: true}}, "quad/edge-snap"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalRenderCfg(tc.in)
			if gotName := rcModeName(got); gotName != tc.want {
				t.Fatalf("canonical name = %q, want %q (cfg %#v)", gotName, tc.want, got)
			}
		})
	}
}

func TestZoomLevelReportsTerminalCellSourceWidthAcrossModes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 728, 592))
	state := viewState{zoom: 2.0}
	const termCols, termRows = 91, 37
	modes := []struct {
		name string
		rc   renderCfg
		want string
	}{
		{"halfblock", renderCfg{id: 0}, "src px/cell=4"},
		{"quad", renderCfg{id: 1, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}, "src px/cell=4"},
		{"spark", renderCfg{id: 3, mode: modeSpark, sparkMode: sparkline.Quad}, "src px/cell=4"},
		{"spark geom", renderCfg{id: 4, mode: modeSparkGeom, sparkMode: sparkline.Geom}, "src px/cell=4"},
		{"spark best", renderCfg{id: 5, mode: modeSparkBest, sparkMode: sparkline.Best}, "src px/cell=4"},
		{"sextant", renderCfg{id: 6, mode: modeSextant, sextantMode: sextant.ModeSextant}, "src px/cell=5"},
	}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			if got := zoomLevel(state, src, termCols, termRows, tc.rc); got != tc.want {
				t.Fatalf("zoomLevel = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildRefUsesCommonQualityGridAcrossModes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 728, 592))
	state := viewState{zoom: 2.0}
	const termCols, termRows = 91, 37
	modes := []struct {
		name  string
		rc    renderCfg
		wantW int
		wantH int
	}{
		// halfblock and quad use the common GridK×GridK quality grid per terminal cell.
		{"halfblock", renderCfg{id: 0}, termCols * metrics.GridK, termRows * metrics.GridK},
		{"quad", renderCfg{id: 1, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}, termCols * metrics.GridK, termRows * metrics.GridK},
		// spark has CellH=8 so the common quality grid (GridK=4 per cell) would be
		// at viewH/2 — smaller than RenderToImage output.  buildRef falls through to
		// viewW×viewH so that rendered and ref are compared at the same resolution
		// without a 2× downscale that would alias the character fill pattern.
		{"spark", renderCfg{id: 3, mode: modeSpark, sparkMode: sparkline.Quad}, termCols * metrics.GridK, termRows * metrics.GridK * 2},
		{"spark geom", renderCfg{id: 4, mode: modeSparkGeom, sparkMode: sparkline.Geom}, termCols * metrics.GridK, termRows * metrics.GridK * 2},
		{"spark best", renderCfg{id: 5, mode: modeSparkBest, sparkMode: sparkline.Best}, termCols * metrics.GridK, termRows * metrics.GridK * 2},
		{"sextant", renderCfg{id: 6, mode: modeSextant, sextantMode: sextant.ModeSextant}, termCols * metrics.GridK, termRows * metrics.GridK},
	}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			st := state
			_ = buildViewport(src, &st, termCols, termRows, tc.rc)
			ref := buildRef(src, st, termCols, termRows, tc.rc, metrics.GridK, false)
			b := ref.Bounds()
			if b.Dx() != tc.wantW || b.Dy() != tc.wantH {
				t.Fatalf("ref size = %dx%d, want %dx%d", b.Dx(), b.Dy(), tc.wantW, tc.wantH)
			}
		})
	}
}

func TestBuildViewportExpandsSparkSmallFitToDisplaySize(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 32))
	const termCols, termRows = 80, 22

	modes := []struct {
		name string
		rc   renderCfg
		want renderCells
	}{
		{"halfblock", renderCfg{id: 0}, renderCells{Cols: 32, Rows: 16}},
		{"quad", renderCfg{id: 1, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}, renderCells{Cols: 32, Rows: 16}},
		{"spark", renderCfg{id: 3, mode: modeSpark, sparkMode: sparkline.Quad}, renderCells{Cols: 32, Rows: 16}},
		{"spark geom", renderCfg{id: 4, mode: modeSparkGeom, sparkMode: sparkline.Geom}, renderCells{Cols: 32, Rows: 16}},
		{"spark best", renderCfg{id: 5, mode: modeSparkBest, sparkMode: sparkline.Best}, renderCells{Cols: 32, Rows: 16}},
		{"sextant", renderCfg{id: 6, mode: modeSextant, sextantMode: sextant.ModeSextant}, renderCells{Cols: 32, Rows: 16}},
	}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			state := viewState{zoom: 1.0}
			vp := buildViewport(src, &state, termCols, termRows, tc.rc)
			if got := renderedCellSize(vp, tc.rc); got != tc.want {
				t.Fatalf("rendered size = %dx%d, want %dx%d", got.Cols, got.Rows, tc.want.Cols, tc.want.Rows)
			}
			if err := validateRenderSize(src, vp, state, termCols, termRows, tc.rc); err != nil {
				t.Fatalf("validateRenderSize(%s) unexpected error: %v", tc.name, err)
			}
		})
	}
}

func TestValidateRenderSizeCatchesBadSparkViewport(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 32))
	vp := image.NewRGBA(image.Rect(0, 0, 32, 32))
	rc := renderCfg{id: 3, mode: modeSpark, sparkMode: sparkline.Quad}
	state := viewState{zoom: 1.0}
	const termCols, termRows = 80, 22

	err := validateRenderSize(src, vp, state, termCols, termRows, rc)
	if err == nil {
		t.Fatal("validateRenderSize(spark) succeeded, want size mismatch")
	}
}

func TestCycleRenderKeepsRenderedImageSizeAtZoomKThree(t *testing.T) {
	src := horizontalGradientNRGBA(32, 32)
	const termCols, termRows = 95, 35

	rc := renderCfg{id: 1, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}
	state := viewState{
		zoom: rc.mode.viewSpec().ZoomRatioForK(
			maxZoom(src.Bounds().Dx(), src.Bounds().Dy(), termCols, termRows, rc.mode),
			3,
		),
	}

	vp := buildViewport(src, &state, termCols, termRows, rc)
	want := renderedCellSize(vp, rc)
	if want.Cols <= 0 || want.Rows <= 0 {
		t.Fatalf("initial rendered size = %dx%d, want non-empty", want.Cols, want.Rows)
	}

	for i := 0; i < len(renderModes)*2; i++ {
		oldRC := rc
		rc, _ = cycleRenderCfg(rc)
		preserveZoomForMode(&state, src, termCols, termRows, oldRC, rc)
		recenterForMode(&state, src, termCols, termRows, oldRC, rc)

		vp = buildViewport(src, &state, termCols, termRows, rc)
		got := renderedCellSize(vp, rc)
		if got != want {
			t.Fatalf("after cycle %d to %s rendered size = %dx%d, want %dx%d",
				i+1, rcModeName(rc), got.Cols, got.Rows, want.Cols, want.Rows)
		}
	}
}

func TestHorizontalGradientSparkQuadQualityAtZoomKThree(t *testing.T) {
	src := horizontalGradientNRGBA(32, 32)
	const termCols, termRows = 95, 35

	rc := renderCfg{}
	state := viewState{
		zoom: rc.mode.viewSpec().ZoomRatioForK(
			maxZoom(src.Bounds().Dx(), src.Bounds().Dy(), termCols, termRows, rc.mode),
			3,
		),
	}

	scores := map[string]float64{}
	for i := 0; i < len(renderModes); i++ {
		st := state
		vp := buildViewport(src, &st, termCols, termRows, rc)
		ref := buildRef(src, st, termCols, termRows, rc, metrics.GridK, false)
		scores[rcModeName(rc)] = computeQuality(ref, vp, rc).SSIM

		oldRC := rc
		rc, _ = cycleRenderCfg(rc)
		preserveZoomForMode(&state, src, termCols, termRows, oldRC, rc)
		recenterForMode(&state, src, termCols, termRows, oldRC, rc)
	}

	spark := scores["spark/quad"]
	for _, mode := range []string{"quad/splithalf", "quad/edge-snap"} {
		if spark+1e-9 < scores[mode] {
			t.Fatalf("spark/quad SSIM %.6f below %s %.6f (scores: %#v)", spark, mode, scores[mode], scores)
		}
	}
}

func horizontalGradientNRGBA(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(0)
			if w > 1 {
				v = uint8(x * 255 / (w - 1))
			}
			img.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}

// TestMaxZoomOneToOne verifies that at max zoom, each viewport pixel
// corresponds to exactly one source pixel for an image that fills the viewport.
func TestMaxZoomOneToOne(t *testing.T) {
	// 10×8 source = pixCols × pixRows for termCols=10, termRows=4 in halfblock.
	src := image.NewNRGBA(image.Rect(0, 0, 10, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 10; x++ {
			src.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 25),
				G: uint8(y * 30),
				B: uint8(x + y*10),
				A: 255,
			})
		}
	}

	const termCols, termRows = 10, 4
	zoom := maxZoom(10, 8, termCols, termRows, modeHalfblock)
	if zoom != 1.0 {
		t.Fatalf("maxZoom for 10×8 image = %v, want 1.0", zoom)
	}

	state := viewState{zoom: zoom, panX: 0, panY: 0}
	rc := renderCfg{id: 0} // halfblock
	vp := buildViewport(src, &state, termCols, termRows, rc)
	b := vp.Bounds()

	if b.Dx() != 10 || b.Dy() != 8 {
		t.Fatalf("viewport size = %dx%d, want 10×8", b.Dx(), b.Dy())
	}

	for y := 0; y < 8; y++ {
		for x := 0; x < 10; x++ {
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
	}{
		{"maxZoom=1, srcW=5", 1.0, 5, 14},
		{"maxZoom=2, srcW=10", 2.0, 10, 19},
		{"maxZoom=24, srcW=48", 24.0, 48, 37},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := zoomSteps(tc.mz, tc.srcW)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			if got[len(got)-1] != tc.mz/float64(tc.srcW) {
				t.Errorf("steps[last] = %v, want %v", got[len(got)-1], tc.mz/float64(tc.srcW))
			}
			for i := 1; i < len(got); i++ {
				if got[i] >= got[i-1] {
					t.Errorf("steps[%d]=%v ≥ steps[%d]=%v (expected descending)", i, got[i], i-1, got[i-1])
				}
			}
		})
	}
}

func TestZoomStepsAdaptiveTail(t *testing.T) {
	steps := zoomSteps(24.0, 48)
	want := []float64{24.0 / 17.0, 24.0 / 19.0, 24.0 / 21.0, 24.0 / 23.0, 24.0 / 25.0}
	for i, z := range want {
		if got := steps[24+i]; math.Abs(got-z) > 1e-9 {
			t.Fatalf("tail step %d = %v, want %v", 24+i, got, z)
		}
	}
	if got := steps[len(steps)-1]; got != 24.0/48.0 {
		t.Fatalf("last step = %v, want %v", got, 24.0/48.0)
	}
}

func TestStepIdx(t *testing.T) {
	steps := zoomSteps(24.0, 48)
	tests := []struct {
		name string
		zoom float64
		want int
	}{
		{"above max → clamped to 0", steps[0] * 1.01, 0},
		{"exact 1st step → index 0", steps[0], 0},
		{"exact 2nd step → index 1", steps[1], 1},
		{"exact mid step → correct index", steps[len(steps)/2], len(steps) / 2},
		{"exact last step → last index", steps[len(steps)-1], len(steps) - 1},
		{"below min → clamped to last", steps[len(steps)-1] * 0.5, len(steps) - 1},
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

	for i, zoom := range steps {
		rc := renderCfg{id: 0}
		state := viewState{zoom: zoom, panX: 0, panY: 0}
		vp := buildViewport(src, &state, termCols, termRows, rc)
		b := vp.Bounds()

		expectState := viewState{zoom: zoom, panX: 0, panY: 0}
		dims := rc.mode.viewSpec().Dims(20, 10, termCols, termRows, expectState.zoom)
		scaled := resizeRenderedImage(src, dims.ScaledW, dims.ScaledH, rc)
		dims.ClampPan(&expectState.panX, &expectState.panY)
		viewW, viewH := alignViewportSize(dims.ViewW, dims.ViewH, rc)
		wantVP := imgutil.CropImage(scaled, expectState.panX, expectState.panY, viewW, viewH)
		targetCells := expectedCellSize(src, expectState, termCols, termRows, rc)
		targetW, targetH := viewportPixelSizeForCells(targetCells, rc)
		if targetW > 0 && targetH > 0 {
			wb := wantVP.Bounds()
			if wb.Dx() != targetW || wb.Dy() != targetH {
				wantVP = resizeRenderedImage(wantVP, targetW, targetH, rc)
			}
		}

		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				got := vp.At(x, y)
				want := wantVP.At(x, y)
				r1, g1, b1, a1 := got.RGBA()
				r2, g2, b2, a2 := want.RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Errorf("step %d (zoom=%v): vp.At(%d,%d) = (%d,%d,%d,%d), want normalized crop (%d,%d,%d,%d)",
						i, zoom, x, y, r1, g1, b1, a1, r2, g2, b2, a2)
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

func TestSpecZoomKKeyBindings(t *testing.T) {
	inputSpec, _ := input.Load(fs.FS(spec.FS))
	defs := loadButtonKeyDefs(inputSpec)
	keyRows := loadViewKeyRows()
	maps := buildViewKeyMaps(keyRows, defs)

	for _, k := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		action, ok := maps["image_viewer"][k]
		if !ok {
			t.Errorf("image_viewer has no key binding for %q", k)
			continue
		}
		if action != "zoom_k" {
			t.Errorf("key %q bound to action %q, want zoom_k", k, action)
		}
	}
}
