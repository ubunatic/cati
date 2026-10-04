package viewgeom

import "ubunatic.com/cati/internal/imgutil"

// V2Spec carries width-first geometry inputs.
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

	renderW, renderH, extH := imgutil.FitDimsRatio(srcW, srcH, s.CellW, s.CellH, s.AspectNum, s.AspectDen, cols, rows)
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

	renderW, renderH, extH := imgutil.FitDimsRatio(srcW, srcH, s.CellW, s.CellH, s.AspectNum, s.AspectDen, cols, 0)
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

