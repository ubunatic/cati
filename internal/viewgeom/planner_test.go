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
}

func TestPlanRender_QuadblockAndSparkline(t *testing.T) {
	// 32x20 test image
	quadSpec := NewV2CellRatio(2, 2, 2, 1)
	sparkSpec := NewV2CellRatio(4, 8, 1, 1)

	t.Run("quadblock width only -W 8", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitCols: 8,
			AspectMode:   "default",
		}
		plan := PlanRender(32, 20, c, quadSpec)
		if plan.CanvasCols != 8 {
			t.Errorf("CanvasCols = %d, want 8", plan.CanvasCols)
		}
		if plan.CanvasRows <= 0 {
			t.Errorf("CanvasRows must be > 0, got %d", plan.CanvasRows)
		}
	})

	t.Run("sparkline height only -H 10", func(t *testing.T) {
		c := TargetConstraints{
			ExplicitRows: 10,
			AspectMode:   "default",
		}
		plan := PlanRender(32, 20, c, sparkSpec)
		if plan.CanvasRows != 10 {
			t.Errorf("CanvasRows = %d, want 10", plan.CanvasRows)
		}
		if plan.CanvasCols <= 0 {
			t.Errorf("CanvasCols must be > 0, got %d", plan.CanvasCols)
		}
	})
}
