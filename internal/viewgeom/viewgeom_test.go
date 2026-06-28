package viewgeom

import "testing"

func TestMaxZoom(t *testing.T) {
	tests := []struct {
		name               string
		spec               Spec
		srcW, srcH         int
		termCols, termRows int
		want               float64
	}{
		{"halfblock image larger than viewport", NewCell(1, 2, 1), 1920, 1080, 80, 40, 24.0},
		{"quad image larger than viewport", NewCell(2, 2, 2), 1920, 1080, 80, 40, 24.0},
		{"halfblock image fits exactly", NewCell(1, 2, 1), 80, 40, 80, 40, 1.0},
		{"zero srcW", NewCell(1, 2, 1), 0, 100, 80, 40, 1.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.spec.MaxZoom(tc.srcW, tc.srcH, tc.termCols, tc.termRows)
			if got != tc.want {
				t.Fatalf("MaxZoom(...) = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestViewportDims(t *testing.T) {
	g := NewCell(1, 2, 1)
	pixCols, pixRows, scaledW, scaledH, viewW, viewH := g.ViewportDims(10, 8, 10, 4, 1.0)
	if pixCols != 10 || pixRows != 8 || scaledW != 10 || scaledH != 8 || viewW != 10 || viewH != 8 {
		t.Fatalf("ViewportDims = %d,%d,%d,%d,%d,%d", pixCols, pixRows, scaledW, scaledH, viewW, viewH)
	}
}

func TestViewportDimsQuadMatchesHorizontalStretch(t *testing.T) {
	g := NewCell(2, 2, 2)
	pixCols, pixRows, scaledW, scaledH, viewW, viewH := g.ViewportDims(10, 8, 10, 4, 1.0)
	if pixCols != 20 || pixRows != 8 || scaledW != 20 || scaledH != 8 || viewW != 20 || viewH != 8 {
		t.Fatalf("ViewportDims quad = %d,%d,%d,%d,%d,%d", pixCols, pixRows, scaledW, scaledH, viewW, viewH)
	}
}

func TestZoomAtCursor(t *testing.T) {
	g := NewCell(2, 2, 2)
	zoom := 1.0
	panX, panY := 0, 0
	g.ZoomAtCursor(&zoom, &panX, &panY, 2.0, 40, 0)
	if panX != 80 || zoom != 2.0 {
		t.Fatalf("ZoomAtCursor = panX %d zoom %v", panX, zoom)
	}
}

func TestVisibleCrop(t *testing.T) {
	g := NewCell(1, 2, 1)
	if w, h := g.VisibleCrop(0, 10, 1.0, 0, 0, 80, 40); w != 0 || h != 0 {
		t.Fatalf("VisibleCrop zero image = %d,%d", w, h)
	}
}

func TestRecenter(t *testing.T) {
	oldQ := NewCell(1, 2, 1)
	newQ := NewCell(2, 2, 2)
	panX, panY := oldQ.Recenter(100, 50, 80, 40, 1.0, oldQ, newQ, 20, 10)
	if panX < 0 || panY < 0 {
		t.Fatalf("Recenter returned negative pan: %d,%d", panX, panY)
	}
}

func TestRecenterPreservesSourceCenter(t *testing.T) {
	oldQ := NewCell(1, 2, 1)
	newQ := NewCell(2, 2, 2)
	const srcW, srcH, termCols, termRows = 400, 200, 80, 40
	const zoom = 4.0
	const panX, panY = 30, 20

	_, _, oldScaledW, oldScaledH, oldViewW, oldViewH := oldQ.ViewportDims(srcW, srcH, termCols, termRows, zoom)
	oldCX := (float64(panX) + float64(oldViewW)/2) * float64(srcW) / float64(oldScaledW)
	oldCY := (float64(panY) + float64(oldViewH)/2) * float64(srcH) / float64(oldScaledH)

	newPanX, newPanY := oldQ.Recenter(srcW, srcH, termCols, termRows, zoom, oldQ, newQ, panX, panY)
	_, _, newScaledW, newScaledH, newViewW, newViewH := newQ.ViewportDims(srcW, srcH, termCols, termRows, zoom)
	newCX := (float64(newPanX) + float64(newViewW)/2) * float64(srcW) / float64(newScaledW)
	newCY := (float64(newPanY) + float64(newViewH)/2) * float64(srcH) / float64(newScaledH)

	if diff := oldCX - newCX; diff < -0.6 || diff > 0.6 {
		t.Fatalf("center x drift = %v", diff)
	}
	if diff := oldCY - newCY; diff < -0.6 || diff > 0.6 {
		t.Fatalf("center y drift = %v", diff)
	}
}

func TestPanByCellHasConsistentSourceDeltaAcrossModes(t *testing.T) {
	specs := []struct {
		name string
		spec Spec
	}{
		{"halfblock", NewCell(1, 2, 1)},
		{"quad", NewCell(2, 2, 2)},
		{"spark", NewCell(4, 8, 1)},
	}

	const srcW, srcH = 1920, 1080
	const termCols, termRows = 80, 38
	const zoom = 4.0

	var wantX, wantY float64
	for i, tc := range specs {
		t.Run(tc.name, func(t *testing.T) {
			dims := tc.spec.Dims(srcW, srcH, termCols, termRows, zoom)
			panX, panY := 0, 0
			tc.spec.PanByCells(&panX, &panY, 1, 1)
			gotX := float64(panX) * float64(srcW) / float64(dims.ScaledW)
			gotY := float64(panY) * float64(srcH) / float64(dims.ScaledH)
			if i == 0 {
				wantX, wantY = gotX, gotY
				return
			}
			if gotX != wantX || gotY != wantY {
				t.Fatalf("source delta = %.6g,%.6g, want %.6g,%.6g", gotX, gotY, wantX, wantY)
			}
		})
	}
}

func TestZoomSteps(t *testing.T) {
	tests := []struct {
		name    string
		mz      float64
		srcW    int
		wantLen int
	}{
		{"maxZoom=1, srcW=5", 1.0, 5, 15},
		{"maxZoom=2, srcW=10", 2.0, 10, 20},
		{"maxZoom=24, srcW=48", 24.0, 48, 38},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ZoomSteps(tc.mz, tc.srcW, ZoomStepSpec{
				Levels: []float64{0.125, 0.25, 0.5, 0.75, 1.25},
				Extend: "adaptive",
			})
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			if got[len(got)-1] != tc.mz/float64(tc.srcW) {
				t.Fatalf("steps[last] = %v, want %v", got[len(got)-1], tc.mz/float64(tc.srcW))
			}
		})
	}
}

func TestStepIdx(t *testing.T) {
	steps := ZoomSteps(24.0, 48, ZoomStepSpec{
		Levels: []float64{0.125, 0.25, 0.5, 0.75, 1.25},
		Extend: "adaptive",
	})
	tests := []struct {
		zoom float64
		want int
	}{
		{steps[0] * 1.01, 0},
		{steps[0], 0},
		{steps[len(steps)-1], len(steps) - 1},
	}
	for _, tc := range tests {
		if got := StepIdx(tc.zoom, steps); got != tc.want {
			t.Fatalf("StepIdx(%v) = %d, want %d", tc.zoom, got, tc.want)
		}
	}
}

func TestZoomSequenceRoundTrip(t *testing.T) {
	steps := ZoomSteps(24.0, 48, ZoomStepSpec{
		Levels: []float64{0.125, 0.25, 0.5, 0.75, 1.25},
		Extend: "adaptive",
	})
	for i, z := range steps {
		if got := StepIdx(z, steps); got != i {
			t.Fatalf("step %d: StepIdx returned %d", i, got)
		}
	}
}

func TestZoomLevel(t *testing.T) {
	got := NewCell(1, 2, 1).ZoomLevel(1.0, 10, 8, 10, 4)
	if got != "src px/cell=1" {
		t.Fatalf("ZoomLevel = %q, want src px/cell=1", got)
	}
}

func TestParseZoomK(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"0", 0},
		{"1", 1},
		{"100%", 1},
		{"1:1", 1},
	}
	for _, tc := range tests {
		if got := ParseZoomK(tc.in); got != tc.want {
			t.Fatalf("ParseZoomK(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCropMath(t *testing.T) {
	x0, y0, x1, y1 := SrcCrop(10, 10, 4, 4, 8, 8, 4, 4)
	if x1 <= x0 || y1 <= y0 {
		t.Fatalf("SrcCrop invalid: %d,%d,%d,%d", x0, y0, x1, y1)
	}
}

func TestInitialZoomRatio(t *testing.T) {
	s := NewCell(1, 2, 1)
	// Image: 100x50. Term: 80x10.
	// pixCols = 80, pixRows = 20.
	// baseFitW, baseFitH = imgutil.FitPixelDims(100, 50, 80, 20) = 40, 20.
	// For "w": zoom = 80 / 40 = 2.0.
	// For "h": zoom = 20 / 20 = 1.0.
	gotW := s.InitialZoomRatio("w", 100, 50, 80, 10)
	if gotW != 2.0 {
		t.Errorf("InitialZoomRatio(\"w\") = %v, want 2.0", gotW)
	}
	gotH := s.InitialZoomRatio("h", 100, 50, 80, 10)
	if gotH != 1.0 {
		t.Errorf("InitialZoomRatio(\"h\") = %v, want 1.0", gotH)
	}
	got0 := s.InitialZoomRatio("0", 100, 50, 80, 10)
	if got0 != 1.0 {
		t.Errorf("InitialZoomRatio(\"0\") = %v, want 1.0", got0)
	}
}
