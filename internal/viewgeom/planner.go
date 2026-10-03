package viewgeom

import (
	"ubunatic.com/cati/internal/imgutil"
)

// TargetConstraints holds the requested dimensions from CLI and terminal.
type TargetConstraints struct {
	ExplicitCols int    // From -W (0 = unconstrained)
	ExplicitRows int    // From -H (0 = unconstrained)
	TermCols     int    // From terminal width
	TermRows     int    // From terminal height
	AspectMode   string // "default" or "aligned"
	InitialZoom  string // "0", "1", "w", "h", etc.
}

// Plan describes the resolved terminal canvas, content layout, scaling, and padding.
type Plan struct {
	CanvasCols int // Output terminal canvas columns
	CanvasRows int // Output terminal canvas rows
	RenderW    int // Scaled pixel width of content
	RenderH    int // Scaled pixel height of content
	ExtH       int // Transparent vertical padding rows (to complete cellH)
	PadRight   int // Right padding in pixels
	PadBottom  int // Bottom padding in pixels
}

// PlanRender calculates the complete geometry plan for rendering an image of srcW×srcH.
func PlanRender(srcW, srcH int, c TargetConstraints, spec V2Spec) Plan {
	if srcW <= 0 || srcH <= 0 {
		cols := max(1, c.ExplicitCols)
		rows := max(1, c.ExplicitRows)
		return Plan{CanvasCols: cols, CanvasRows: rows}
	}

	spec = NewV2CellRatio(spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen)

	// Explicit zoom "0" means fit to box/viewport
	if c.InitialZoom != "" && c.InitialZoom != "0" {
		targetW, targetH, ok := explicitZoomPixelTarget(srcW, srcH, c, spec)
		if ok {
			cols := max(1, (targetW+spec.CellW-1)/spec.CellW)
			rows := max(1, (targetH+spec.CellH-1)/spec.CellH)
			return Plan{
				CanvasCols: cols,
				CanvasRows: rows,
				RenderW:    targetW,
				RenderH:    targetH,
			}
		}
	}

	switch {
	case c.ExplicitCols > 0 && c.ExplicitRows > 0:
		// Both dimensions explicit: hard canvas box
		cols, rows := c.ExplicitCols, c.ExplicitRows
		if c.AspectMode == "aligned" {
			targetW := cols * spec.CellW
			targetH := rows * spec.CellH
			if srcW <= targetW && srcH <= targetH {
				return Plan{
					CanvasCols: cols,
					CanvasRows: rows,
					RenderW:    srcW,
					RenderH:    srcH,
					PadRight:   targetW - srcW,
					PadBottom:  targetH - srcH,
				}
			}
		}
		renderW, renderH, extH := fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, cols, rows)
		return Plan{
			CanvasCols: cols,
			CanvasRows: rows,
			RenderW:    renderW,
			RenderH:    renderH,
			ExtH:       extH,
		}

	case c.ExplicitCols > 0 && c.ExplicitRows == 0:
		// Width explicit: rows derived from aspect
		cols := c.ExplicitCols
		if c.AspectMode == "aligned" {
			targetW := cols * spec.CellW
			if srcW <= targetW {
				rows := max(1, (srcH+spec.CellH-1)/spec.CellH)
				targetH := rows * spec.CellH
				return Plan{
					CanvasCols: cols,
					CanvasRows: rows,
					RenderW:    srcW,
					RenderH:    srcH,
					PadRight:   targetW - srcW,
					PadBottom:  targetH - srcH,
				}
			}
		}
		renderW, renderH, extH := fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, cols, 0)
		rows := max(1, (renderH+extH+spec.CellH-1)/spec.CellH)
		return Plan{
			CanvasCols: cols,
			CanvasRows: rows,
			RenderW:    renderW,
			RenderH:    renderH,
			ExtH:       extH,
		}

	case c.ExplicitRows > 0 && c.ExplicitCols == 0:
		// Height explicit: cols derived from aspect
		rows := c.ExplicitRows
		renderW, renderH, extH := fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, 0, rows)
		cols := max(1, (renderW+spec.CellW-1)/spec.CellW)
		return Plan{
			CanvasCols: cols,
			CanvasRows: rows,
			RenderW:    renderW,
			RenderH:    renderH,
			ExtH:       extH,
		}

	default:
		// Neither explicit: fallback to terminal bounding box
		termCols, termRows := c.TermCols, c.TermRows
		if termCols <= 0 && termRows <= 0 {
			termCols, termRows = 80, 24
		}
		renderW, renderH, extH := fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, termCols, termRows)
		cols := max(1, (renderW+spec.CellW-1)/spec.CellW)
		rows := max(1, (renderH+extH+spec.CellH-1)/spec.CellH)
		return Plan{
			CanvasCols: cols,
			CanvasRows: rows,
			RenderW:    renderW,
			RenderH:    renderH,
			ExtH:       extH,
		}
	}
}

func explicitZoomPixelTarget(srcW, srcH int, c TargetConstraints, spec V2Spec) (int, int, bool) {
	if c.InitialZoom == "w" || c.InitialZoom == "h" {
		cols := c.ExplicitCols
		rows := c.ExplicitRows
		if cols <= 0 {
			cols = c.TermCols
		}
		if rows <= 0 {
			rows = c.TermRows
		}
		if cols <= 0 && rows <= 0 {
			cols, rows = 80, 24
		}
		oldSpec := NewCellRatio(spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen)
		zoom := oldSpec.InitialZoomRatio(c.InitialZoom, srcW, srcH, cols, rows, true)
		dims := oldSpec.Dims(srcW, srcH, cols, rows, zoom)
		targetW, targetH := imgutil.AlignCellSize(dims.ScaledW, dims.ScaledH, spec.CellW, spec.CellH)
		return targetW, targetH, true
	}

	k := ParseZoomK(c.InitialZoom)
	if k <= 0 {
		return 0, 0, false
	}
	targetCols := max(1, int(float64(srcW)/k+0.5))
	targetRows := max(1, int(float64(srcH)/(2*k)+0.5))
	return targetCols * spec.CellW, targetRows * spec.CellH, true
}
