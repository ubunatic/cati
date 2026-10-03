package viewgeom

import (
	"math"
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

	// Step 1: Resolve the base canvas (CanvasCols, CanvasRows) and base content fit.
	var canvasCols, canvasRows int
	var baseRenderW, baseRenderH, baseExtH int
	var padRight, padBottom int
	hasAlignedPad := false

	switch {
	case c.ExplicitCols > 0 && c.ExplicitRows > 0:
		// Both dimensions explicit: hard canvas box
		canvasCols = c.ExplicitCols
		canvasRows = c.ExplicitRows
		targetW := canvasCols * spec.CellW
		targetH := canvasRows * spec.CellH
		k := max(1, int(math.Round(float64(targetW)/float64(srcW))))
		if k*srcW > targetW && k > 1 {
			k = targetW / srcW
		}
		if k >= 1 && k*srcW <= targetW && k*srcH <= targetH {
			diffW := targetW - k*srcW
			diffH := targetH - k*srcH
			if c.AspectMode == "aligned" || (diffW < spec.CellW && diffH < spec.CellH) {
				baseRenderW = k * srcW
				baseRenderH = k * srcH
				padRight = diffW
				padBottom = diffH
				hasAlignedPad = true
			}
		}
		if !hasAlignedPad {
			if c.AspectMode == "contain" || c.AspectMode == "fit" {
				baseRenderW, baseRenderH, baseExtH = fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, canvasCols, canvasRows)
			} else {
				baseRenderW = targetW
				baseRenderH = targetH
			}
		}

	case c.ExplicitCols > 0 && c.ExplicitRows == 0:
		// Width explicit: canvas rows derived from aspect
		canvasCols = c.ExplicitCols
		if c.AspectMode == "aligned" {
			targetW := canvasCols * spec.CellW
			k := max(1, int(math.Round(float64(targetW)/float64(srcW))))
			if k*srcW > targetW && k > 1 {
				k = targetW / srcW
			}
			if k >= 1 && k*srcW <= targetW {
				baseRenderW = k * srcW
				baseRenderH = k * srcH
				padRight = targetW - baseRenderW
				canvasRows = max(1, (baseRenderH+spec.CellH-1)/spec.CellH)
				targetH := canvasRows * spec.CellH
				padBottom = targetH - baseRenderH
				hasAlignedPad = true
			}
		}
		if !hasAlignedPad {
			baseRenderW, baseRenderH, baseExtH = fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, canvasCols, 0)
			canvasRows = max(1, (baseRenderH+baseExtH+spec.CellH-1)/spec.CellH)
		}

	case c.ExplicitRows > 0 && c.ExplicitCols == 0:
		// Height explicit: canvas cols derived from aspect
		canvasRows = c.ExplicitRows
		if c.AspectMode == "aligned" {
			targetH := canvasRows * spec.CellH
			k := max(1, int(math.Round(float64(targetH)/float64(srcH))))
			if k*srcH > targetH && k > 1 {
				k = targetH / srcH
			}
			if k >= 1 && k*srcH <= targetH {
				baseRenderW = k * srcW
				baseRenderH = k * srcH
				padBottom = targetH - baseRenderH
				canvasCols = max(1, (baseRenderW+spec.CellW-1)/spec.CellW)
				targetW := canvasCols * spec.CellW
				padRight = targetW - baseRenderW
				hasAlignedPad = true
			}
		}
		if !hasAlignedPad {
			baseRenderW, baseRenderH, baseExtH = fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, 0, canvasRows)
			canvasCols = max(1, (baseRenderW+spec.CellW-1)/spec.CellW)
		}

	default:
		// Neither explicit: fallback to terminal bounding box
		termCols, termRows := c.TermCols, c.TermRows
		if termCols <= 0 && termRows <= 0 {
			termCols, termRows = 80, 24
		}
		baseRenderW, baseRenderH, baseExtH = fitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, termCols, termRows)
		canvasCols = max(1, (baseRenderW+spec.CellW-1)/spec.CellW)
		canvasRows = max(1, (baseRenderH+baseExtH+spec.CellH-1)/spec.CellH)
	}

	// Step 2: If no explicit zoom (or zoom is "0"), return the fitted canvas and content.
	if c.InitialZoom == "" || c.InitialZoom == "0" || hasAlignedPad {
		return Plan{
			CanvasCols: canvasCols,
			CanvasRows: canvasRows,
			RenderW:    baseRenderW,
			RenderH:    baseRenderH,
			ExtH:       baseExtH,
			PadRight:   padRight,
			PadBottom:  padBottom,
		}
	}

	// Step 3: Explicit zoom ("w", "h", or numeric k).
	// CanvasCols and CanvasRows remain the resolved outer bounds.
	zoomW, zoomH, ok := computeZoomContentSize(srcW, srcH, canvasCols, canvasRows, c.InitialZoom, spec)
	if !ok {
		return Plan{
			CanvasCols: canvasCols,
			CanvasRows: canvasRows,
			RenderW:    baseRenderW,
			RenderH:    baseRenderH,
			ExtH:       baseExtH,
		}
	}

	// If canvas was not explicit in either axis and no terminal was given, expand canvas to fit zoom
	if c.ExplicitCols == 0 && c.ExplicitRows == 0 && c.TermCols <= 0 && c.TermRows <= 0 {
		canvasCols = max(1, (zoomW+spec.CellW-1)/spec.CellW)
		canvasRows = max(1, (zoomH+spec.CellH-1)/spec.CellH)
	}

	return Plan{
		CanvasCols: canvasCols,
		CanvasRows: canvasRows,
		RenderW:    zoomW,
		RenderH:    zoomH,
	}
}

func computeZoomContentSize(srcW, srcH, canvasCols, canvasRows int, initialZoom string, spec V2Spec) (int, int, bool) {
	switch initialZoom {
	case "w":
		renderW := canvasCols * spec.CellW
		renderH := max(1, int(math.Round(float64(renderW*srcH*spec.AspectDen)/float64(srcW*spec.AspectNum))))
		return renderW, renderH, true
	case "h":
		renderH := canvasRows * spec.CellH
		renderW := max(1, int(math.Round(float64(renderH*srcW*spec.AspectNum)/float64(srcH*spec.AspectDen))))
		return renderW, renderH, true
	}

	k := ParseZoomK(initialZoom)
	if k <= 0 {
		return 0, 0, false
	}

	colsFloat := float64(srcW) / k
	rowsFloat := float64(srcH) / (2.0 * k)

	targetCols := safeScaleInt(colsFloat)
	targetRows := safeScaleInt(rowsFloat)

	return targetCols * spec.CellW, targetRows * spec.CellH, true
}

func safeScaleInt(val float64) int {
	if math.IsNaN(val) || val <= 0 {
		return 1
	}
	if val > 1_000_000 {
		return 1_000_000
	}
	return max(1, int(math.Ceil(val)))
}
