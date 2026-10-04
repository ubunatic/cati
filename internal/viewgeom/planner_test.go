package viewgeom

import (
	"math"
	"testing"

	appspec "ubunatic.com/cati/spec"
)

func TestPlanRender_DoomHalfblock(t *testing.T) {
	// Doom 1: 320x200 px
	// Halfblock: 1x2 cell, 1:1 aspect
	halfSpec := NewV2CellRatio(1, 2, 1, 1)

	t.Run("width only -W 160", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			AspectMode:   "default",
		}
		plan := PlanRender(320, 200, c, halfSpec)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 50 {
			t.Errorf("CanvasRows = %d, want 50", plan.CanvasRows)
		}
		if plan.RenderW != 160 || plan.RenderH != 100 {
			t.Errorf("Render size = %dx%d, want 160x100", plan.RenderW, plan.RenderH)
		}
	})

	t.Run("width only -W 160 aspect aligned", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			AspectMode:   "aligned",
		}
		plan := PlanRender(320, 200, c, halfSpec)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 50 {
			t.Errorf("CanvasRows = %d, want 50", plan.CanvasRows)
		}
	})

	t.Run("height only -H 50", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitRows: 50,
			AspectMode:   "default",
		}
		plan := PlanRender(320, 200, c, halfSpec)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 50 {
			t.Errorf("CanvasRows = %d, want 50", plan.CanvasRows)
		}
	})
}

func TestPlanRender_DoomSextant(t *testing.T) {
	// Doom 1: 320x200 px
	// Sextant (s2): 2x3 cell, 4:3 aspect
	s2Spec := NewV2CellRatio(2, 3, 4, 3)

	t.Run("both -W 160 -H 67 aspect aligned", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			ExplicitRows: 67,
			AspectMode:   "aligned",
		}
		plan := PlanRender(320, 200, c, s2Spec)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 67 {
			t.Errorf("CanvasRows = %d, want 67", plan.CanvasRows)
		}
		if plan.RenderW != 320 || plan.RenderH != 200 {
			t.Errorf("Render size = %dx%d, want 320x200 (source 1:1)", plan.RenderW, plan.RenderH)
		}
		if plan.PadBottom != 1 || plan.PadRight != 0 {
			t.Errorf("Pad = R:%d B:%d, want R:0 B:1", plan.PadRight, plan.PadBottom)
		}
	})

	t.Run("height only -H 67 aspect aligned", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitRows: 67,
			AspectMode:   "aligned",
		}
		plan := PlanRender(320, 200, c, s2Spec)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 67 {
			t.Errorf("CanvasRows = %d, want 67", plan.CanvasRows)
		}
		if plan.RenderW != 320 || plan.RenderH != 200 {
			t.Errorf("Render size = %dx%d, want 320x200 (source 1:1)", plan.RenderW, plan.RenderH)
		}
		if plan.PadBottom != 1 || plan.PadRight != 0 {
			t.Errorf("Pad = R:%d B:%d, want R:0 B:1", plan.PadRight, plan.PadBottom)
		}
	})

	t.Run("width only -W 160 aspect aligned", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			AspectMode:   "aligned",
		}
		plan := PlanRender(320, 200, c, s2Spec)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 67 {
			t.Errorf("CanvasRows = %d, want 67", plan.CanvasRows)
		}
		if plan.RenderW != 320 || plan.RenderH != 200 {
			t.Errorf("Render size = %dx%d, want 320x200 (source 1:1)", plan.RenderW, plan.RenderH)
		}
		if plan.PadBottom != 1 || plan.PadRight != 0 {
			t.Errorf("Pad = R:%d B:%d, want R:0 B:1", plan.PadRight, plan.PadBottom)
		}
	})
}

func TestPlanRender_Doom3x3(t *testing.T) {
	// Doom 1: 320x200 px
	// Mode 3x3: 6x3 cell, aspect 4:1 (acSrcW = srcW*4, acSrcH = srcH*1)
	spec3x3 := NewV2CellRatio(6, 3, 4, 1)

	t.Run("width only -W 160 aspect aligned integer 3x upscale", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			AspectMode:   "aligned",
		}
		plan := PlanRender(320, 200, c, spec3x3)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 67 {
			t.Errorf("CanvasRows = %d, want 67", plan.CanvasRows)
		}
		if plan.RenderW != 960 || plan.RenderH != 200 {
			t.Errorf("Render size = %dx%d, want 960x200 (3x integer scale)", plan.RenderW, plan.RenderH)
		}
		if plan.PadRight != 0 || plan.PadBottom != 1 {
			t.Errorf("Pad = R:%d B:%d, want R:0 B:1", plan.PadRight, plan.PadBottom)
		}
	})

	t.Run("width only -W 107 aspect aligned integer 2x upscale", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 107,
			AspectMode:   "aligned",
		}
		plan := PlanRender(320, 200, c, spec3x3)
		if plan.CanvasCols != 107 {
			t.Errorf("CanvasCols = %d, want 107", plan.CanvasCols)
		}
		if plan.CanvasRows != 45 {
			t.Errorf("CanvasRows = %d, want 45", plan.CanvasRows)
		}
		if plan.RenderW != 640 || plan.RenderH != 133 {
			t.Errorf("Render size = %dx%d, want 640x133 (2x integer scale)", plan.RenderW, plan.RenderH)
		}
		if plan.PadRight != 2 || plan.PadBottom != 2 {
			t.Errorf("Pad = R:%d B:%d, want R:2 B:2", plan.PadRight, plan.PadBottom)
		}
	})

	t.Run("width only -W 107 aspect pixel limits height distortion", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 107,
			AspectMode:   "pixel",
			PixelPolicy:  pixelPolicy(t),
		}
		plan := PlanRender(320, 200, c, spec3x3)
		if plan.CanvasCols != 107 {
			t.Errorf("CanvasCols = %d, want 107", plan.CanvasCols)
		}
		if plan.CanvasRows != 45 {
			t.Errorf("CanvasRows = %d, want 45", plan.CanvasRows)
		}
		if plan.RenderW != 640 || plan.RenderH != 133 {
			t.Errorf("Render size = %dx%d, want 640x133", plan.RenderW, plan.RenderH)
		}
		if plan.PadRight != 2 || plan.PadBottom != 2 {
			t.Errorf("Pad = R:%d B:%d, want R:2 B:2", plan.PadRight, plan.PadBottom)
		}
	})

	t.Run("six -W 160 aspect pixel duplicates rows 2x", func(t *testing.T) {
		spec6 := NewV2CellRatio(2, 6, 2, 3)
		plan := PlanRender(320, 200, TargetConstraints{ExplicitCols: 160, AspectMode: "pixel"}, spec6)
		if plan.CanvasCols != 160 || plan.CanvasRows != 67 {
			t.Errorf("Canvas = %dx%d, want 160x67", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderW != 320 || plan.RenderH != 400 {
			t.Errorf("Render size = %dx%d, want 320x400 (1x cols, 2x rows)", plan.RenderW, plan.RenderH)
		}
	})

	t.Run("both -W 160 -H 67 subcell snapping", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			ExplicitRows: 67,
			AspectMode:   "default",
		}
		plan := PlanRender(320, 200, c, spec3x3)
		if plan.CanvasCols != 160 || plan.CanvasRows != 67 {
			t.Errorf("Canvas = %dx%d, want 160x67", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderW != 960 || plan.RenderH != 200 {
			t.Errorf("Render size = %dx%d, want 960x200 (subcell snap)", plan.RenderW, plan.RenderH)
		}
		if plan.PadBottom != 1 {
			t.Errorf("PadBottom = %d, want 1", plan.PadBottom)
		}
	})

	t.Run("both -W 160 -H 80 default stretch", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			ExplicitRows: 80,
			AspectMode:   "default",
		}
		plan := PlanRender(320, 200, c, spec3x3)
		if plan.CanvasCols != 160 || plan.CanvasRows != 80 {
			t.Errorf("Canvas = %dx%d, want 160x80", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderW != 160*6 || plan.RenderH != 80*3 {
			t.Errorf("Render size = %dx%d, want %dx%d", plan.RenderW, plan.RenderH, 160*6, 80*3)
		}
	})
}

func TestPlanRender_PixelSnapBounds(t *testing.T) {
	policy := pixelPolicy(t)
	for _, spec := range []V2Spec{
		NewV2CellRatio(1, 2, 1, 1),
		NewV2CellRatio(2, 2, 2, 1),
		NewV2CellRatio(2, 3, 4, 3),
		NewV2CellRatio(6, 3, 4, 1),
		NewV2CellRatio(2, 6, 2, 3),
	} {
		for _, aspect := range []string{"pixel", "raw", "1:1"} {
			for extent := 1; extent <= 321; extent++ {
				width := PlanRender(320, 200, TargetConstraints{ExplicitCols: extent, AspectMode: aspect, PixelPolicy: policy}, spec)
				idealH := float64(width.RenderW*spec.CellH*200*2) / float64(spec.CellW*320*3)
				if math.Abs(float64(width.RenderH)-idealH) >= float64(spec.CellH) {
					t.Fatalf("cell %dx%d %s W=%d: height %d differs by a cell from ideal %.3f", spec.CellW, spec.CellH, aspect, extent, width.RenderH, idealH)
				}
				if math.Abs(float64(width.RenderH)-idealH) > math.Max(1, idealH*policy.MaxDistortion) {
					t.Fatalf("%s W=%d: derived height exceeds relative snapping or raster rounding bound", aspect, extent)
				}
				if width.CanvasCols != extent || width.PadRight < 0 || float64(width.PadRight) > float64(extent*spec.CellW)*policy.MaxPadding || width.RenderW+width.PadRight != extent*spec.CellW || width.RenderH+width.PadBottom != width.CanvasRows*spec.CellH || width.PadBottom < 0 || width.PadBottom >= spec.CellH {
					t.Fatalf("cell %dx%d %s W=%d: invalid canvas or padding %+v", spec.CellW, spec.CellH, aspect, extent, width)
				}
				height := PlanRender(320, 200, TargetConstraints{ExplicitRows: extent, AspectMode: aspect, PixelPolicy: policy}, spec)
				idealW := float64(height.RenderH*spec.CellW*320*3) / float64(spec.CellH*200*2)
				if math.Abs(float64(height.RenderW)-idealW) >= float64(spec.CellW) {
					t.Fatalf("cell %dx%d %s H=%d: width %d differs by a cell from ideal %.3f", spec.CellW, spec.CellH, aspect, extent, height.RenderW, idealW)
				}
				if math.Abs(float64(height.RenderW)-idealW) > math.Max(1, idealW*policy.MaxDistortion) {
					t.Fatalf("%s H=%d: derived width exceeds relative snapping or raster rounding bound", aspect, extent)
				}
				if height.CanvasRows != extent || height.PadBottom < 0 || float64(height.PadBottom) > float64(extent*spec.CellH)*policy.MaxPadding || height.RenderH+height.PadBottom != extent*spec.CellH || height.RenderW+height.PadRight != height.CanvasCols*spec.CellW || height.PadRight < 0 || height.PadRight >= spec.CellW {
					t.Fatalf("cell %dx%d %s H=%d: invalid canvas or padding %+v", spec.CellW, spec.CellH, aspect, extent, height)
				}
			}
		}
	}
}

func TestPixelDerivedSize_RepeatTolerance(t *testing.T) {
	for _, tc := range []struct {
		ideal float64
		want  int
	}{
		{197.5, 200}, // Less than a 3px cell: allow an integer repeat.
		{197, 197},   // Exactly one cell: retain the continuous size.
		{66.67, 67},  // Never force a 1x repeat on a much smaller target.
	} {
		if got := pixelDerivedSize(200, tc.ideal, 3, pixelPolicy(t).MaxDistortion); got != tc.want {
			t.Errorf("ideal %.2f: got %d, want %d", tc.ideal, got, tc.want)
		}
	}
}

func pixelPolicy(t *testing.T) PixelAspectPolicy {
	t.Helper()
	policy, err := appspec.LoadPixelAspectPolicy()
	if err != nil {
		t.Fatal(err)
	}
	return PixelAspectPolicy{MaxDistortion: policy.MaxDistortion, MaxPadding: policy.MaxPadding}
}

func TestPlanRender_PixelSmallAndPadded(t *testing.T) {
	grid := NewV2CellRatio(6, 3, 4, 1)
	policy := pixelPolicy(t)
	for _, tc := range []struct {
		name                   string
		srcW, srcH, cols, rows int
		want                   Plan
	}{
		{"small 20 percent stretch rejected", 12, 12, 5, 0, Plan{CanvasCols: 5, CanvasRows: 4, RenderW: 30, RenderH: 10, PadBottom: 2}},
		{"Doom uniform 2x columns with padding", 320, 200, 110, 0, Plan{CanvasCols: 110, CanvasRows: 45, RenderW: 640, RenderH: 133, PadRight: 20, PadBottom: 2}},
		{"height allows padding for uniform rows", 320, 200, 0, 70, Plan{CanvasCols: 160, CanvasRows: 70, RenderW: 960, RenderH: 200, PadBottom: 10}},
		{"width beyond padding budget", 320, 200, 119, 0, Plan{CanvasCols: 119, CanvasRows: 50, RenderW: 714, RenderH: 149, PadBottom: 1}},
	} {
		for _, aspect := range []string{"pixel", "raw", "1:1"} {
			t.Run(tc.name+"/"+aspect, func(t *testing.T) {
				got := PlanRender(tc.srcW, tc.srcH, TargetConstraints{ExplicitCols: tc.cols, ExplicitRows: tc.rows, AspectMode: aspect, PixelPolicy: policy}, grid)
				if got != tc.want {
					t.Fatalf("plan = %+v, want %+v", got, tc.want)
				}
			})
		}
	}
	// Policy is passed into the pure planner: no hidden defaults or spec copies.
	noSnap := PlanRender(320, 200, TargetConstraints{ExplicitCols: 110, AspectMode: "pixel"}, grid)
	if noSnap.RenderW != 660 || noSnap.PadRight != 0 {
		t.Fatalf("zero policy should disable optional padding: %+v", noSnap)
	}
}

func TestPixelRepeatPolicyBoundaries(t *testing.T) {
	if got := pixelDerivedSize(12, 10, 3, 0.10); got != 10 {
		t.Fatalf("20%% stretch accepted: %d", got)
	}
	if got := pixelDerivedSize(12, 10, 3, 0.20); got != 12 {
		t.Fatalf("configured 20%% boundary rejected: %d", got)
	}
	if got := pixelConstrainedSize(9, 10, 0.10); got != 9 {
		t.Fatalf("10%% padding boundary rejected: %d", got)
	}
	if got := pixelConstrainedSize(9, 11, 0.10); got != 11 {
		t.Fatalf("padding beyond 10%% accepted: %d", got)
	}
}

func TestPlanRender_ZoomPreservesHardCanvas(t *testing.T) {
	// 32x20 halfblock source
	halfSpec := NewV2CellRatio(1, 2, 1, 1)

	t.Run("-W 8 -H 3 --zoom 1 preserves 8x3 canvas", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 8,
			ExplicitRows: 3,
			InitialZoom:  "1",
		}
		plan := PlanRender(32, 20, c, halfSpec)
		if plan.CanvasCols != 8 || plan.CanvasRows != 3 {
			t.Errorf("Canvas = %dx%d, want 8x3", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderW != 32 || plan.RenderH != 20 {
			t.Errorf("Render = %dx%d, want 32x20", plan.RenderW, plan.RenderH)
		}
	})

	t.Run("-W 8 -H 3 --zoom w preserves 8x3 canvas", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 8,
			ExplicitRows: 3,
			InitialZoom:  "w",
		}
		plan := PlanRender(32, 20, c, halfSpec)
		if plan.CanvasCols != 8 || plan.CanvasRows != 3 {
			t.Errorf("Canvas = %dx%d, want 8x3", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderW != 8 {
			t.Errorf("RenderW = %d, want 8", plan.RenderW)
		}
	})

	t.Run("-W 8 -H 3 --zoom h preserves 8x3 canvas", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 8,
			ExplicitRows: 3,
			InitialZoom:  "h",
		}
		plan := PlanRender(32, 20, c, halfSpec)
		if plan.CanvasCols != 8 || plan.CanvasRows != 3 {
			t.Errorf("Canvas = %dx%d, want 8x3", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderH != 6 {
			t.Errorf("RenderH = %d, want 6 (3 rows * 2 cellH)", plan.RenderH)
		}
	})

	t.Run("extreme small zoom 1e-300 does not overflow", func(t *testing.T) {
		c := TargetConstraints{
			InitialZoom: "1e-300",
		}
		plan := PlanRender(32, 20, c, halfSpec)
		if plan.CanvasCols <= 0 || plan.CanvasRows <= 0 {
			t.Errorf("Canvas dimensions must be positive, got %dx%d", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderW <= 0 || plan.RenderH <= 0 {
			t.Errorf("Render dimensions must be positive, got %dx%d", plan.RenderW, plan.RenderH)
		}
	})
}

func TestPlanRender_EdgeCases(t *testing.T) {
	halfSpec := NewV2CellRatio(1, 2, 1, 1)

	t.Run("zero source size", func(t *testing.T) {
		c := TargetConstraints{ExplicitCols: 80, ExplicitRows: 24}
		plan := PlanRender(0, 0, c, halfSpec)
		if plan.CanvasCols != 80 || plan.CanvasRows != 24 {
			t.Errorf("Canvas = %dx%d, want 80x24", plan.CanvasCols, plan.CanvasRows)
		}
	})

	t.Run("negative source size", func(t *testing.T) {
		c := TargetConstraints{ExplicitCols: 80, ExplicitRows: 24}
		plan := PlanRender(-10, -5, c, halfSpec)
		if plan.CanvasCols != 80 || plan.CanvasRows != 24 {
			t.Errorf("Canvas = %dx%d, want 80x24", plan.CanvasCols, plan.CanvasRows)
		}
	})

	t.Run("fallback terminal bounds", func(t *testing.T) {
		c := TargetConstraints{TermCols: 80, TermRows: 24}
		plan := PlanRender(160, 100, c, halfSpec)
		if plan.CanvasCols > 80 || plan.CanvasRows > 24 {
			t.Errorf("Plan dimensions (%dx%d) exceed terminal fallback 80x24", plan.CanvasCols, plan.CanvasRows)
		}
	})
}
