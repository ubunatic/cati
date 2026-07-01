package viewgeom

import (
	"fmt"
	"image"
	"math"
	"sort"
	"strconv"
	"strings"

	"ubunatic.com/cati/internal/imgutil"
)

// Spec describes the viewport pixel footprint for one terminal cell.
//
// CellW/CellH are the pixel dimensions consumed by the renderer per terminal
// cell after scaling. AspectX is the horizontal source stretch needed before
// fitting to keep terminal output visually square.
type Spec struct {
	CellW   int
	CellH   int
	AspectX int
}

// Dims contains all viewport-space dimensions derived from a source image,
// terminal size, zoom, and renderer geometry.
type Dims struct {
	PixCols int
	PixRows int
	ScaledW int
	ScaledH int
	ViewW   int
	ViewH   int
}

// PanAnchor records the terminal-cell coordinate and viewport-pixel pan where
// an absolute drag gesture began.
type PanAnchor struct {
	Active    bool
	StartCol  int
	StartRow  int
	StartPanX int
	StartPanY int
}

// ZoomStepSpec carries the spec-driven k ladder inputs.
type ZoomStepSpec struct {
	Levels []float64
	Extend string
}

// New returns a sanitized halfblock-like geometry spec for a scalar quantum.
func New(quantum int) Spec {
	if quantum < 1 {
		quantum = 1
	}
	return NewCell(quantum, 2*quantum, quantum)
}

// NewCell returns a sanitized renderer geometry spec.
func NewCell(cellW, cellH, aspectX int) Spec {
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}
	if aspectX < 1 {
		aspectX = 1
	}
	return Spec{CellW: cellW, CellH: cellH, AspectX: aspectX}
}

// PixCols returns the viewport-pixel width for terminal columns.
func (s Spec) PixCols(termCols int) int {
	return termCols * s.CellW
}

// PixRows returns the viewport-pixel height for terminal rows.
func (s Spec) PixRows(termRows int) int {
	return termRows * s.CellH
}

// MaxZoom returns the zoom level at which each cell covers one quantum of the
// source image horizontally and two quanta vertically.
func (s Spec) MaxZoom(srcW, srcH, termCols, termRows int) float64 {
	if srcW <= 0 || srcH <= 0 || termCols <= 0 || termRows <= 0 {
		return 1.0
	}
	_, _, scaledW, scaledH, _, _ := s.ViewportDims(srcW, srcH, termCols, termRows, 1.0)
	if scaledW <= 0 || scaledH <= 0 {
		return 1.0
	}
	zCol := float64(s.AspectX) * float64(srcW) / float64(scaledW)
	zRow := float64(srcH) / float64(scaledH)
	return math.Min(zCol, zRow)
}

// ViewportDims computes derived pixel dimensions for the given zoom state.
func (s Spec) ViewportDims(srcW, srcH, termCols, termRows int, zoom float64) (pixCols, pixRows, scaledW, scaledH, viewW, viewH int) {
	pixCols = s.PixCols(termCols)
	pixRows = s.PixRows(termRows)
	baseFitW, baseFitH := imgutil.FitPixelDims(srcW*s.AspectX, srcH, pixCols, pixRows)
	scaledW = max(1, int(math.Round(float64(baseFitW)*zoom)))
	scaledH = max(1, int(math.Round(float64(baseFitH)*zoom)))
	viewW = min(pixCols, scaledW)
	viewH = min(pixRows, scaledH)
	return
}

// Dims returns ViewportDims as a named value for callers that need to pass the
// same geometry through pan, clamp, crop, and analysis paths.
func (s Spec) Dims(srcW, srcH, termCols, termRows int, zoom float64) Dims {
	pixCols, pixRows, scaledW, scaledH, viewW, viewH := s.ViewportDims(srcW, srcH, termCols, termRows, zoom)
	return Dims{
		PixCols: pixCols,
		PixRows: pixRows,
		ScaledW: scaledW,
		ScaledH: scaledH,
		ViewW:   viewW,
		ViewH:   viewH,
	}
}

// ClampPan clamps pan offsets to the scaled image bounds for this viewport.
func (d Dims) ClampPan(panX, panY *int) {
	*panX = max(0, min(*panX, max(0, d.ScaledW-d.PixCols)))
	*panY = max(0, min(*panY, max(0, d.ScaledH-d.PixRows)))
}

// VisibleSize returns the viewport pixel size available from the current pan.
func (d Dims) VisibleSize(panX, panY int) (int, int) {
	return min(d.ViewW, d.ScaledW-panX), min(d.ViewH, d.ScaledH-panY)
}

// SrcCrop maps viewport pixel coords back to source image coords.
func SrcCrop(srcW, srcH, panX, panY, scaledW, scaledH, viewW, viewH int) (x0, y0, x1, y1 int) {
	x0 = panX * srcW / scaledW
	y0 = panY * srcH / scaledH
	x1 = min((panX+viewW)*srcW/scaledW, srcW)
	y1 = min((panY+viewH)*srcH/scaledH, srcH)
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	return
}

// SrcCrop maps the visible viewport rectangle back to source image coordinates.
func (d Dims) SrcCrop(srcW, srcH, panX, panY int) image.Rectangle {
	viewW, viewH := d.VisibleSize(panX, panY)
	if viewW <= 0 || viewH <= 0 {
		return image.Rectangle{}
	}
	x0, y0, x1, y1 := SrcCrop(srcW, srcH, panX, panY, d.ScaledW, d.ScaledH, viewW, viewH)
	return image.Rect(x0, y0, x1, y1)
}

// VisibleCrop returns the visible source rectangle size for the current state.
func (s Spec) VisibleCrop(srcW, srcH int, zoom float64, panX, panY, termCols, termRows int) (int, int) {
	if srcW <= 0 || srcH <= 0 {
		return 0, 0
	}
	dims := s.Dims(srcW, srcH, termCols, termRows, zoom)
	dims.ClampPan(&panX, &panY)
	vw, vh := dims.VisibleSize(panX, panY)
	if vw <= 0 || vh <= 0 {
		return 0, 0
	}
	crop := dims.SrcCrop(srcW, srcH, panX, panY)
	return max(1, crop.Dx()), max(1, crop.Dy())
}

// ZoomAtCursor adjusts pan so the pixel under the cursor stays fixed.
func (s Spec) ZoomAtCursor(zoom *float64, panX, panY *int, newZoom float64, col, row int) {
	f := newZoom / *zoom
	cursorX := col * s.CellW
	cursorY := row * s.CellH
	*panX = int(math.Round(float64(*panX+cursorX)*f)) - cursorX
	*panY = int(math.Round(float64(*panY+cursorY)*f)) - cursorY
	*zoom = newZoom
}

// NewPanAnchor creates an anchor for absolute grab-and-pull panning.
func NewPanAnchor(col, row, panX, panY int) PanAnchor {
	return PanAnchor{
		Active:    true,
		StartCol:  col,
		StartRow:  row,
		StartPanX: panX,
		StartPanY: panY,
	}
}

// PanFromAnchor returns the pan offsets for a grab-and-pull gesture at col,row.
func (s Spec) PanFromAnchor(anchor PanAnchor, col, row int) (int, int) {
	return anchor.StartPanX - (col-anchor.StartCol)*s.CellW,
		anchor.StartPanY - (row-anchor.StartRow)*s.CellH
}

// PanByCells applies a relative terminal-cell pan delta to viewport-pixel pan.
func (s Spec) PanByCells(panX, panY *int, cols, rows int) {
	*panX += cols * s.CellW
	*panY += rows * s.CellH
}

// Recenter adjusts pan after a mode switch to keep the same source region visible.
func (s Spec) Recenter(srcW, srcH, termCols, termRows int, zoom float64, oldQ, newQ Spec, panX, panY int) (int, int) {
	if srcW <= 0 || srcH <= 0 {
		return 0, 0
	}
	oldDims := oldQ.Dims(srcW, srcH, termCols, termRows, zoom)
	oldDims.ClampPan(&panX, &panY)
	centerX := (float64(panX) + float64(oldDims.ViewW)/2) * float64(srcW) / float64(oldDims.ScaledW)
	centerY := (float64(panY) + float64(oldDims.ViewH)/2) * float64(srcH) / float64(oldDims.ScaledH)

	newDims := newQ.Dims(srcW, srcH, termCols, termRows, zoom)
	panX2 := int(math.Round(centerX*float64(newDims.ScaledW)/float64(srcW) - float64(newDims.ViewW)/2))
	panY2 := int(math.Round(centerY*float64(newDims.ScaledH)/float64(srcH) - float64(newDims.ViewH)/2))
	newDims.ClampPan(&panX2, &panY2)
	return panX2, panY2
}

// ZoomLevel formats the current source-pixels-per-cell value.
func (s Spec) ZoomLevel(zoom float64, srcW, srcH, termCols, termRows int) string {
	mz := s.MaxZoom(srcW, srcH, termCols, termRows)
	srcPxPerCell := mz / zoom
	return fmt.Sprintf("src px/cell=%.3g", srcPxPerCell)
}

// ZoomRatioForK converts a source-pixels-per-cell value into the internal
// zoom multiplier. A non-positive k means fit-to-viewport.
func (s Spec) ZoomRatioForK(mz, k float64) float64 {
	if k <= 0 {
		return 1.0
	}
	return mz / k
}

// ZoomSteps returns a descending sequence of zoom values using the supplied spec.
func ZoomSteps(mz float64, srcW int, spec ZoomStepSpec) []float64 {
	seen := map[float64]bool{}

	for _, k := range spec.Levels {
		k = math.Round(k*10000) / 10000
		if k >= 0.125 && k <= float64(srcW) && !seen[k] {
			seen[k] = true
		}
	}

	for k := 1.0; k <= float64(srcW); {
		k = math.Round(k*10000) / 10000
		if !seen[k] {
			seen[k] = true
		}
		switch spec.Extend {
		case "quarters":
			switch {
			case k < 2:
				k += 0.25
			case k < 5:
				k += 0.5
			default:
				k += 1.0
			}
		case "adaptive":
			switch {
			case k < 2:
				k += 0.25
			case k < 5:
				k += 0.5
			case k < 15:
				k += 1.0
			case k < 32:
				k += 2.0
			case k < 64:
				k += 4.0
			default:
				k += 8.0
			}
		default:
			switch {
			case k < 5:
				k += 0.5
			default:
				k += 1.0
			}
		}
	}

	if srcW > 0 {
		seen[float64(srcW)] = true
	}

	var ks []float64
	for k := range seen {
		ks = append(ks, k)
	}
	sort.Float64Slice(ks).Sort()

	steps := make([]float64, len(ks))
	for i, k := range ks {
		steps[i] = mz / k
	}
	return steps
}

// StepIdx returns the index of the nearest zoom step <= zoom.
func StepIdx(zoom float64, steps []float64) int {
	for i, z := range steps {
		if z <= zoom {
			return i
		}
	}
	return len(steps) - 1
}

// InitialZoomRatio parses a zoom flag and returns the corresponding ratio.
func (s Spec) InitialZoomRatio(flag string, srcW, srcH, termCols, termRows int) float64 {
	if flag == "w" {
		pixCols := s.PixCols(termCols)
		pixRows := s.PixRows(termRows)
		baseFitW, _ := imgutil.FitPixelDims(srcW*s.AspectX, srcH, pixCols, pixRows)
		if baseFitW > 0 {
			return float64(pixCols) / float64(baseFitW)
		}
		return 1.0
	}
	if flag == "h" {
		pixCols := s.PixCols(termCols)
		pixRows := s.PixRows(termRows)
		_, baseFitH := imgutil.FitPixelDims(srcW*s.AspectX, srcH, pixCols, pixRows)
		if baseFitH > 0 {
			return float64(pixRows) / float64(baseFitH)
		}
		return 1.0
	}

	k := ParseZoomK(flag)
	if k <= 0 {
		return 1.0
	}
	mz := s.MaxZoom(srcW, srcH, termCols, termRows)
	if srcW > 0 {
		k = math.Max(k, 1.0/float64(srcW))
		k = math.Min(k, float64(srcW))
	}
	return mz / k
}

// ParseZoomK parses the --zoom value and returns the number of source columns
// per terminal cell (k).
func ParseZoomK(s string) float64 {
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(s)

	var k float64 = -1
	switch {
	case strings.HasSuffix(s, "%"):
		pct, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err == nil && pct >= 0 {
			if pct == 0 {
				k = 0
			} else {
				k = 100.0 / pct
			}
		}
	case strings.Contains(s, ":"):
		parts := strings.SplitN(s, ":", 2)
		a, errA := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		b, errB := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if errA == nil && errB == nil && a >= 0 && b > 0 {
			k = a / b
		}
	default:
		v, err := strconv.ParseFloat(s, 64)
		if err == nil && v >= 0 {
			k = v
		}
	}
	return k
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
