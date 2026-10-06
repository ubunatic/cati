package viewgeom

import (
	"math"

	"ubunatic.com/cati/internal/imgutil"
)

// TargetConstraints holds the requested dimensions from CLI and terminal.
type TargetConstraints struct {
	ExplicitCols int               // From -W (0 = unconstrained)
	ExplicitRows int               // From -H (0 = unconstrained)
	TermCols     int               // From terminal width
	TermRows     int               // From terminal height
	AspectMode   string            // Source mapping mode, including "aligned" and "pixel"
	InitialZoom  string            // "0", "1", "w", "h", etc.
	PixelPolicy  PixelAspectPolicy // Fractional snapping limits from the spec
}

// PixelAspectPolicy bounds the aspect distortion and constrained-axis padding
// allowed for integer repeats. Zero limits allow only exact integer matches.
type PixelAspectPolicy struct {
	MaxDistortion float64
	MaxPadding    float64
	Formula       FormulaPolicy
}

// FormulaParams defines numerator and denominator for derived aspect scaling.
type FormulaParams struct {
	Num int
	Den int
}

// FormulaPolicy holds mode-specific formula parameters.
type FormulaPolicy struct {
	Default   FormulaParams
	Halfblock FormulaParams
}

func formulaParams(spec V2Spec, policy PixelAspectPolicy) (int, int) {
	if spec.CellW == 1 && spec.CellH == 2 {
		if policy.Formula.Halfblock.Num > 0 && policy.Formula.Halfblock.Den > 0 {
			return policy.Formula.Halfblock.Num, policy.Formula.Halfblock.Den
		}
		return 1, 2
	}
	if policy.Formula.Default.Num > 0 && policy.Formula.Default.Den > 0 {
		return policy.Formula.Default.Num, policy.Formula.Default.Den
	}
	return 2, 3
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
	case IsPixelAspect(c.AspectMode) && c.ExplicitCols > 0 && c.ExplicitRows == 0:
		canvasCols = c.ExplicitCols
		targetW := canvasCols * spec.CellW
		baseRenderW = pixelConstrainedSize(srcW, targetW, c.PixelPolicy.MaxPadding)
		fNum, fDen := formulaParams(spec, c.PixelPolicy)
		idealH := float64(baseRenderW) * float64(spec.CellH) * float64(srcH) * float64(fNum) / (float64(spec.CellW) * float64(srcW) * float64(fDen))
		baseRenderH = pixelDerivedSize(srcH, idealH, spec.CellH, c.PixelPolicy.MaxDistortion)
		canvasRows = max(1, (baseRenderH+spec.CellH-1)/spec.CellH)
		padRight = targetW - baseRenderW
		padBottom = canvasRows*spec.CellH - baseRenderH
		hasAlignedPad = true

	case IsPixelAspect(c.AspectMode) && c.ExplicitRows > 0 && c.ExplicitCols == 0:
		canvasRows = c.ExplicitRows
		targetH := canvasRows * spec.CellH
		baseRenderH = pixelConstrainedSize(srcH, targetH, c.PixelPolicy.MaxPadding)
		fNum, fDen := formulaParams(spec, c.PixelPolicy)
		idealW := float64(baseRenderH) * float64(spec.CellW) * float64(srcW) * float64(fDen) / (float64(spec.CellH) * float64(srcH) * float64(fNum))
		baseRenderW = pixelDerivedSize(srcW, idealW, spec.CellW, c.PixelPolicy.MaxDistortion)
		canvasCols = max(1, (baseRenderW+spec.CellW-1)/spec.CellW)
		padBottom = targetH - baseRenderH
		padRight = canvasCols*spec.CellW - baseRenderW
		hasAlignedPad = true

	case c.ExplicitCols > 0 && c.ExplicitRows > 0:
		// Both dimensions explicit: hard canvas box
		canvasCols = c.ExplicitCols
		canvasRows = c.ExplicitRows
		targetW := canvasCols * spec.CellW
		targetH := canvasRows * spec.CellH
		kx := max(1, int(math.Round(float64(targetW)/float64(srcW))))
		if kx*srcW > targetW && kx > 1 {
			kx = targetW / srcW
		}
		ky := max(1, int(math.Round(float64(targetH)/float64(srcH))))
		if ky*srcH > targetH && ky > 1 {
			ky = targetH / srcH
		}
		if kx >= 1 && ky >= 1 && kx*srcW <= targetW && ky*srcH <= targetH {
			diffW := targetW - kx*srcW
			diffH := targetH - ky*srcH
			if c.AspectMode == "aligned" || (diffW < spec.CellW && diffH < spec.CellH) {
				baseRenderW = kx * srcW
				baseRenderH = ky * srcH
				padRight = diffW
				padBottom = diffH
				hasAlignedPad = true
			}
		}
		if !hasAlignedPad {
			if c.AspectMode == "contain" || c.AspectMode == "fit" {
				baseRenderW, baseRenderH, baseExtH = imgutil.FitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, canvasCols, canvasRows)
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
				padRight = targetW - baseRenderW
				fNum, fDen := formulaParams(spec, c.PixelPolicy)
				floatH := float64(baseRenderW*spec.CellH*srcH*fNum) / float64(spec.CellW*srcW*fDen)
				baseRenderH = max(1, int(math.Round(floatH)))
				canvasRows = max(1, (baseRenderH+spec.CellH-1)/spec.CellH)
				targetH := canvasRows * spec.CellH
				padBottom = targetH - baseRenderH
				hasAlignedPad = true
			}
		}
		if !hasAlignedPad {
			baseRenderW, baseRenderH, baseExtH = imgutil.FitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, canvasCols, 0)
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
				baseRenderH = k * srcH
				padBottom = targetH - baseRenderH
				fNum, fDen := formulaParams(spec, c.PixelPolicy)
				floatW := float64(baseRenderH*spec.CellW*srcW*fDen) / float64(spec.CellH*srcH*fNum)
				baseRenderW = max(1, int(math.Round(floatW)))
				canvasCols = max(1, (baseRenderW+spec.CellW-1)/spec.CellW)
				targetW := canvasCols * spec.CellW
				padRight = targetW - baseRenderW
				hasAlignedPad = true
			}
		}
		if !hasAlignedPad {
			baseRenderW, baseRenderH, baseExtH = imgutil.FitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, 0, canvasRows)
			canvasCols = max(1, (baseRenderW+spec.CellW-1)/spec.CellW)
		}

	default:
		// Neither explicit: fallback to terminal bounding box
		termCols, termRows := c.TermCols, c.TermRows
		if termCols <= 0 && termRows <= 0 {
			termCols, termRows = 80, 24
		}
		baseRenderW, baseRenderH, baseExtH = imgutil.FitDimsRatio(srcW, srcH, spec.CellW, spec.CellH, spec.AspectNum, spec.AspectDen, termCols, termRows)
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

// IsPixelAspect reports whether mode requests pixel-preserving nearest-neighbor sampling.
func IsPixelAspect(mode string) bool {
	return mode == "pixel" || mode == "raw" || mode == "1:1"
}

// Keep a whole-pixel repeat only when its padding fits the fractional budget.
// Otherwise use the requested extent, including when the source must shrink.
func pixelConstrainedSize(source, target int, maxPadding float64) int {
	repeated := (target / source) * source
	if repeated > 0 && float64(target-repeated) <= float64(target)*maxPadding {
		return repeated
	}
	return target
}

// Integer repeats must satisfy both the one-cell and relative distortion bounds.
// Otherwise use the continuous aspect rounded to a pixel, sampled with NN.
func pixelDerivedSize(source int, ideal float64, cell int, maxDistortion float64) int {
	repeated := max(1, int(math.Round(ideal/float64(source)))) * source
	error := math.Abs(float64(repeated) - ideal)
	if error < float64(cell) && error <= ideal*maxDistortion {
		return repeated
	}
	return max(1, int(math.Round(ideal)))
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
