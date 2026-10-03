package viewgeom

import (
	"testing"
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

	t.Run("height only -H 50 aspect aligned", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitRows: 50,
			AspectMode:   "aligned",
		}
		plan := PlanRender(320, 200, c, s2Spec)
		if plan.CanvasCols != 160 {
			t.Errorf("CanvasCols = %d, want 160", plan.CanvasCols)
		}
		if plan.CanvasRows != 50 {
			t.Errorf("CanvasRows = %d, want 50", plan.CanvasRows)
		}
		if plan.RenderW != 320 || plan.RenderH != 150 {
			t.Errorf("Render size = %dx%d, want 320x150", plan.RenderW, plan.RenderH)
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
		if plan.CanvasRows != 50 {
			t.Errorf("CanvasRows = %d, want 50", plan.CanvasRows)
		}
		if plan.RenderW != 320 || plan.RenderH != 150 {
			t.Errorf("Render size = %dx%d, want 320x150", plan.RenderW, plan.RenderH)
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
		if plan.CanvasRows != 50 {
			t.Errorf("CanvasRows = %d, want 50", plan.CanvasRows)
		}
		if plan.RenderW != 960 || plan.RenderH != 150 {
			t.Errorf("Render size = %dx%d, want 960x150 (3x integer scale with 4:1 aspect correction)", plan.RenderW, plan.RenderH)
		}
		if plan.PadRight != 0 {
			t.Errorf("PadRight = %d, want 0", plan.PadRight)
		}
	})

	t.Run("both -W 160 -H 67 default stretch", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 160,
			ExplicitRows: 67,
			AspectMode:   "default",
		}
		plan := PlanRender(320, 200, c, spec3x3)
		if plan.CanvasCols != 160 || plan.CanvasRows != 67 {
			t.Errorf("Canvas = %dx%d, want 160x67", plan.CanvasCols, plan.CanvasRows)
		}
		if plan.RenderW != 160*6 || plan.RenderH != 67*3 {
			t.Errorf("Render size = %dx%d, want %dx%d", plan.RenderW, plan.RenderH, 160*6, 67*3)
		}
	})
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
