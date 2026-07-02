package viewgeom

// V2Spec carries the copied width-first geometry inputs.
//
// FIXME: this is a v2 copy of the render-fit math. Keep it isolated until the
// pipeline settles, then consolidate with the original geometry surface.
type V2Spec struct {
	CellW     int
	CellH     int
	AspectNum int
	AspectDen int
}

// NewV2CellRatio returns a sanitized v2 geometry spec.
func NewV2CellRatio(cellW, cellH, aspectNum, aspectDen int) V2Spec {
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}
	if aspectNum < 1 {
		aspectNum = 1
	}
	if aspectDen < 1 {
		aspectDen = 1
	}
	return V2Spec{CellW: cellW, CellH: cellH, AspectNum: aspectNum, AspectDen: aspectDen}
}

// V2Plan describes the width-first geometry result.
//
// RenderH is the resized image height before any transparent tail is appended.
// DisplayH is RenderH + ExtH. FillH/CutH make DisplayH balance against the
// requested height cap.
type V2Plan struct {
	RequestedCols int
	RequestedRows int
	InnerCols     int
	DerivedCols   int
	RenderW       int
	RenderH       int
	ExtH          int
	DisplayH      int
	FillH         int
	CutH          int
	BottomHalf    bool
	Frame         bool
}

// Fit computes the v2 geometry fitting inside both width and height constraints.
func (s V2Spec) Fit(srcW, srcH, cols, rows int, frame bool) V2Plan {
	plan := V2Plan{
		RequestedCols: cols,
		RequestedRows: rows,
		Frame:         frame,
	}
	if srcW <= 0 || srcH <= 0 {
		return plan
	}

	if cols <= 0 && rows > 0 {
		maxH := rows * s.CellH
		derivedW := max(1, srcW*s.AspectNum*maxH/(srcH*s.AspectDen))
		cols = max(1, derivedW/s.CellW)
		plan.DerivedCols = cols
	}
	if frame {
		cols = max(1, cols-2)
	}
	plan.InnerCols = cols

	renderW, renderH, extH := fitDimsRatio(srcW, srcH, s.CellW, s.CellH, s.AspectNum, s.AspectDen, cols, rows)
	plan.RenderW = renderW
	plan.RenderH = renderH
	plan.ExtH = extH
	plan.DisplayH = renderH + extH
	plan.BottomHalf = extH > 0

	if rows > 0 {
		maxH := rows * s.CellH
		switch {
		case plan.DisplayH < maxH:
			plan.FillH = maxH - plan.DisplayH
		case plan.DisplayH > maxH:
			plan.CutH = plan.DisplayH - maxH
		}
	}

	return plan
}

// FitWidthPrimary computes the v2 geometry using width as the primary constraint.
//
// Width is the primary constraint. If width is zero and height is non-zero,
// the function derives a width naively from the height cap, then reuses that
// width as the primary constraint. Height is treated as an upper bound only,
// and overflow is recorded in CutH.
func (s V2Spec) FitWidthPrimary(srcW, srcH, cols, rows int, frame bool) V2Plan {
	plan := V2Plan{
		RequestedCols: cols,
		RequestedRows: rows,
		Frame:         frame,
	}
	if srcW <= 0 || srcH <= 0 {
		return plan
	}

	if cols <= 0 && rows > 0 {
		maxH := rows * s.CellH
		derivedW := max(1, srcW*s.AspectNum*maxH/(srcH*s.AspectDen))
		cols = max(1, derivedW/s.CellW)
		plan.DerivedCols = cols
	}
	if frame {
		cols = max(1, cols-2)
	}
	plan.InnerCols = cols

	renderW, renderH, extH := fitDimsRatio(srcW, srcH, s.CellW, s.CellH, s.AspectNum, s.AspectDen, cols, 0)
	plan.RenderW = renderW
	plan.RenderH = renderH
	plan.ExtH = extH
	plan.DisplayH = renderH + extH
	plan.BottomHalf = extH > 0

	if rows > 0 {
		maxH := rows * s.CellH
		switch {
		case plan.DisplayH < maxH:
			plan.FillH = maxH - plan.DisplayH
		case plan.DisplayH > maxH:
			plan.CutH = plan.DisplayH - maxH
		}
	}

	return plan
}

// fitDimsRatio is the copied rational-aspect fit math for the v2 pipeline.
//
// FIXME: keep in sync with the v2 render path until the copy is consolidated.
func fitDimsRatio(srcW, srcH, cellW, cellH, aspectNum, aspectDen, cols, rows int) (targetW, targetH, extH int) {
	if srcW == 0 || srcH == 0 {
		return max(1, cols*cellW), max(1, rows*cellH), 0
	}
	if aspectNum < 1 {
		aspectNum = 1
	}
	if aspectDen < 1 {
		aspectDen = 1
	}

	maxW, maxH := cols*cellW, rows*cellH

	acSrcW := srcW * aspectNum
	acSrcH := srcH * aspectDen
	var rawW int
	var hNum, hDen int
	heightDerived := true
	switch {
	case cols <= 0 && rows <= 0:
		rawW, hNum, hDen = max(1, acSrcW/aspectDen), srcH, 1
	case rows <= 0:
		rawW, hNum, hDen = maxW, acSrcH*maxW, acSrcW
	case cols <= 0:
		targetH, rawW, heightDerived = maxH, max(1, acSrcW*maxH/acSrcH), false
	default:
		if acSrcW*maxH >= acSrcH*maxW {
			rawW, hNum, hDen = maxW, acSrcH*maxW, acSrcW
		} else {
			targetH, rawW, heightDerived = maxH, max(1, acSrcW*maxH/acSrcH), false
		}
	}

	if heightDerived {
		half := cellH / 2
		if cellH > 1 && half > 0 {
			cellScaled := cellH * hDen
			halfScaled := half * hDen
			full := hNum / cellScaled
			rem := hNum - full*cellScaled
			switch {
			case rem < halfScaled:
				targetH = full * cellH
			case 2*rem <= cellScaled+halfScaled:
				targetH = full*cellH + half
				extH = half
			default:
				targetH = (full + 1) * cellH
			}
		} else {
			targetH = max(1, hNum/hDen)
		}
		if targetH < 1 {
			targetH = cellH
		}
	}

	targetW = rawW
	if cellW > 1 && targetW > cellW {
		targetW -= targetW % cellW
	}
	if targetW < 1 {
		targetW = 1
	}

	return targetW, targetH, extH
}
