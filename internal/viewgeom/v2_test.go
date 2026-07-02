package viewgeom

import "testing"

func TestV2FitBalancesHeight(t *testing.T) {
	specs := []struct {
		name string
		spec V2Spec
	}{
		{"halfblock", NewV2CellRatio(1, 2, 1, 1)},
		{"quad", NewV2CellRatio(2, 2, 2, 1)},
		{"spark", NewV2CellRatio(4, 8, 1, 1)},
		{"sextant", NewV2CellRatio(2, 3, 4, 3)},
	}
	sources := []struct {
		name       string
		srcW, srcH int
	}{
		{"vacation", 1042, 1383},
		{"darth", 687, 1168},
		{"soldering", 640, 480},
		{"cross", 20, 20},
	}
	for _, s := range sources {
		for _, cols := range []int{1, 2, 3, 5, 6, 10, 17, 24} {
			for _, rows := range []int{2, 4, 8, 12, 16} {
				for _, tc := range specs {
					plan := tc.spec.Fit(s.srcW, s.srcH, cols, rows, false)
					if plan.RenderW != plan.InnerCols*tc.spec.CellW {
						t.Fatalf("%s %s -w%d -h%d: render width %d != inner cols %d * cellW %d",
							s.name, tc.name, cols, rows, plan.RenderW, plan.InnerCols, tc.spec.CellW)
					}
					if got := plan.DisplayH + plan.FillH - plan.CutH; got != rows*tc.spec.CellH {
						t.Fatalf("%s %s -w%d -h%d: display balance %d != %d",
							s.name, tc.name, cols, rows, got, rows*tc.spec.CellH)
					}
				}
			}
		}
	}
}

func TestV2DerivesWidthFromHeight(t *testing.T) {
	spec := NewV2CellRatio(2, 3, 4, 3)
	plan := spec.Fit(687, 1168, 0, 13, false)
	if plan.DerivedCols <= 0 {
		t.Fatalf("DerivedCols = %d, want > 0", plan.DerivedCols)
	}
	if plan.InnerCols != plan.DerivedCols {
		t.Fatalf("InnerCols = %d, want derived %d", plan.InnerCols, plan.DerivedCols)
	}
	if got := plan.DisplayH + plan.FillH - plan.CutH; got != 13*spec.CellH {
		t.Fatalf("height balance = %d, want %d", got, 13*spec.CellH)
	}
}

func TestV2FitNoConstraintsPreservesAspect(t *testing.T) {
	specs := []struct {
		name string
		spec V2Spec
	}{
		{"six", NewV2CellRatio(2, 3, 4, 3)},
		{"six+half", NewV2CellRatio(2, 6, 2, 3)},
		{"spark+six", NewV2CellRatio(4, 24, 1, 3)},
	}

	for _, tc := range specs {
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.spec.Fit(640, 480, 0, 0, false)
			if plan.RenderW <= 0 || plan.RenderH <= 0 {
				t.Fatalf("render size = %dx%d, want positive", plan.RenderW, plan.RenderH)
			}
			gotNum := plan.RenderW * tc.spec.AspectDen
			gotDen := plan.RenderH * tc.spec.AspectNum
			wantNum, wantDen := 640, 480
			left := gotNum * wantDen
			right := wantNum * gotDen
			tolerance := (tc.spec.CellW*gotDen + tc.spec.CellH*gotNum) * wantDen
			diff := left - right
			if diff < 0 {
				diff = -diff
			}
			if diff > tolerance {
				t.Fatalf("aspect mismatch: render=%dx%d correction=%d:%d gives %d/%d, want %d/%d",
					plan.RenderW, plan.RenderH, tc.spec.AspectNum, tc.spec.AspectDen,
					gotNum, gotDen, wantNum, wantDen)
			}
		})
	}
}

func TestV2FrameAndHalfRows(t *testing.T) {
	spec := NewV2CellRatio(2, 3, 4, 3)

	var halfPlan V2Plan
	foundHalf := false
	for cols := 2; cols <= 24 && !foundHalf; cols++ {
		plan := spec.Fit(640, 480, cols, 12, true)
		if plan.BottomHalf {
			halfPlan = plan
			foundHalf = true
		}
	}
	if !foundHalf {
		t.Fatal("did not find a halfrow case for framed v2 fit")
	}
	if halfPlan.InnerCols < 1 {
		t.Fatalf("framed inner cols = %d, want at least 1", halfPlan.InnerCols)
	}
	if halfPlan.RequestedCols < 2 || halfPlan.InnerCols != max(1, halfPlan.RequestedCols-2) {
		t.Fatalf("framed inner cols = %d, want max(1, requested-2) = %d", halfPlan.InnerCols, max(1, halfPlan.RequestedCols-2))
	}
	if !halfPlan.BottomHalf {
		t.Fatal("framed halfrow plan did not report a bottom halfrow")
	}

	var fullPlan V2Plan
	foundFull := false
	for cols := 2; cols <= 24 && !foundFull; cols++ {
		plan := spec.Fit(640, 480, cols, 12, true)
		if !plan.BottomHalf {
			fullPlan = plan
			foundFull = true
		}
	}
	if !foundFull {
		t.Fatal("did not find a full-row case for framed v2 fit")
	}
	if fullPlan.InnerCols < 1 {
		t.Fatalf("framed inner cols = %d, want at least 1", fullPlan.InnerCols)
	}
	if fullPlan.RequestedCols < 2 || fullPlan.InnerCols != max(1, fullPlan.RequestedCols-2) {
		t.Fatalf("framed inner cols = %d, want max(1, requested-2) = %d", fullPlan.InnerCols, max(1, fullPlan.RequestedCols-2))
	}
	if fullPlan.BottomHalf {
		t.Fatal("framed full-row plan unexpectedly reported a bottom halfrow")
	}
}
