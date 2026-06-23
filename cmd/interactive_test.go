package cmd

import (
	"image"
	"image/color"
	"testing"
)

// ── interactive (error paths) ─────────────────────────────────────────────────

func TestInteractive_MissingFile(t *testing.T) {
	err := interactive("nonexistent.png", 0, 0)
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
			gotW, gotH := fitPixelDims(tc.srcW, tc.srcH, tc.maxW, tc.maxH)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("fitPixelDims(%d,%d,%d,%d) = %dx%d, want %dx%d",
					tc.srcW, tc.srcH, tc.maxW, tc.maxH, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

// ── parseSGRMouse ─────────────────────────────────────────────────────────────

func TestParseSGRMouse(t *testing.T) {
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
			btn, col, row, release, ok := parseSGRMouse(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if btn != tc.wantBtn || col != tc.wantCol || row != tc.wantRow || release != tc.wantRelease {
				t.Errorf("got btn=%d col=%d row=%d release=%v, want btn=%d col=%d row=%d release=%v",
					btn, col, row, release, tc.wantBtn, tc.wantCol, tc.wantRow, tc.wantRelease)
			}
		})
	}
}

// ── SGR button predicates ─────────────────────────────────────────────────────

func TestSGRPredicates(t *testing.T) {
	type row struct {
		btn      int
		isScroll bool
		isDrag   bool
		button   int
	}
	tests := []row{
		{0, false, false, 0},   // left click
		{1, false, false, 1},   // middle click
		{2, false, false, 2},   // right click
		{32, false, true, 0},   // left drag
		{33, false, true, 1},   // middle drag
		{34, false, true, 2},   // right drag
		{64, true, false, 0},   // scroll up
		{65, true, false, 1},   // scroll down
		{68, true, false, 0},   // scroll up + shift
		{36, false, true, 0},   // left drag + shift (32+4)
	}
	for _, tc := range tests {
		if got := sgrIsScroll(tc.btn); got != tc.isScroll {
			t.Errorf("sgrIsScroll(%d) = %v, want %v", tc.btn, got, tc.isScroll)
		}
		if got := sgrIsDrag(tc.btn); got != tc.isDrag {
			t.Errorf("sgrIsDrag(%d) = %v, want %v", tc.btn, got, tc.isDrag)
		}
		if got := sgrButton(tc.btn); got != tc.button {
			t.Errorf("sgrButton(%d) = %v, want %v", tc.btn, got, tc.button)
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

	crop := cropImage(img, 4, 4, 4, 4)
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
