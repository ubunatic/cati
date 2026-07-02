package cmd

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"ubunatic.com/cati/internal/imgutil"
)

type cropMode int

const (
	cropNone cropMode = iota
	cropFixed
	cropAuto
)

type cropHAlign int

const (
	cropAlignCenter cropHAlign = iota
	cropAlignLeft
	cropAlignRight
)

type cropVAlign int

const (
	cropAlignMiddle cropVAlign = iota
	cropAlignTop
	cropAlignBottom
)

type cropSpec struct {
	mode   cropMode
	cols   int
	rows   int
	x      int
	y      int
	hAlign cropHAlign
	vAlign cropVAlign
}

func parseCropSpec(raw string) (cropSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cropSpec{}, nil
	}
	if isAutoCropAlias(firstCropToken(raw)) || strings.Contains(raw, ",") {
		return parseAutoCropSpec(raw)
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 2 && len(parts) != 4 {
		return cropSpec{}, fmt.Errorf("invalid --crop %q; use W:H, W:H:X:Y, auto, or alignment aliases like l,t", raw)
	}
	vals := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return cropSpec{}, fmt.Errorf("invalid --crop %q: %q is not an integer", raw, part)
		}
		vals[i] = n
	}
	if vals[0] <= 0 || vals[1] <= 0 {
		return cropSpec{}, fmt.Errorf("invalid --crop %q: width and height must be greater than zero", raw)
	}
	if len(vals) == 4 && (vals[2] < 0 || vals[3] < 0) {
		return cropSpec{}, fmt.Errorf("invalid --crop %q: x and y must be zero or greater", raw)
	}
	spec := cropSpec{
		mode:   cropFixed,
		cols:   vals[0],
		rows:   vals[1],
		hAlign: cropAlignCenter,
		vAlign: cropAlignMiddle,
	}
	if len(vals) == 4 {
		spec.x = vals[2]
		spec.y = vals[3]
		spec.hAlign = cropAlignLeft
		spec.vAlign = cropAlignTop
	}
	return spec, nil
}

func parseAutoCropSpec(raw string) (cropSpec, error) {
	parts := strings.Split(raw, ",")
	spec := cropSpec{
		mode:   cropAuto,
		hAlign: cropAlignCenter,
		vAlign: cropAlignMiddle,
	}
	start := 0
	if isAutoCropAlias(strings.ToLower(strings.TrimSpace(parts[0]))) {
		start = 1
	}
	for _, part := range parts[start:] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "", "c", "center":
			spec.hAlign = cropAlignCenter
		case "l", "left":
			spec.hAlign = cropAlignLeft
		case "r", "right":
			spec.hAlign = cropAlignRight
		case "m", "middle":
			spec.vAlign = cropAlignMiddle
		case "t", "top":
			spec.vAlign = cropAlignTop
		case "b", "bottom":
			spec.vAlign = cropAlignBottom
		default:
			return cropSpec{}, fmt.Errorf("invalid --crop %q: unknown alignment %q", raw, part)
		}
	}
	return spec, nil
}

func firstCropToken(raw string) string {
	token, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(raw)), ",")
	return strings.TrimSpace(token)
}

func isAutoCropAlias(token string) bool {
	switch token {
	case "a", "auto", "1", "true":
		return true
	default:
		return false
	}
}

func applyCellCrop(img image.Image, rc renderCfg, spec cropSpec, autoCols, autoRows int) image.Image {
	if spec.mode == cropNone {
		return img
	}
	cells := renderedCellSize(img, rc)
	if cells.Cols <= 0 || cells.Rows <= 0 {
		return img
	}

	cols, rows := spec.cols, spec.rows
	if spec.mode == cropAuto {
		cols = autoCols
		rows = autoRows
	}
	cols = min(max(1, cols), cells.Cols)
	rows = min(max(1, rows), cells.Rows)

	x, y := spec.x, spec.y
	if spec.mode == cropAuto || (spec.mode == cropFixed && spec.hAlign != cropAlignLeft) {
		x = alignedCropHOffset(cells.Cols, cols, spec.hAlign)
	}
	if spec.mode == cropAuto || (spec.mode == cropFixed && spec.vAlign != cropAlignTop) {
		y = alignedCropVOffset(cells.Rows, rows, spec.vAlign)
	}
	x = min(max(0, x), max(0, cells.Cols-cols))
	y = min(max(0, y), max(0, cells.Rows-rows))

	cellW, cellH := rc.mode.renderCellSize()
	return imgutil.CropImage(img, x*cellW, y*cellH, cols*cellW, rows*cellH)
}

func alignedCropHOffset(total, size int, align cropHAlign) int {
	if size >= total {
		return 0
	}
	switch align {
	case cropAlignLeft:
		return 0
	case cropAlignRight:
		return total - size
	default:
		return (total - size) / 2
	}
}

func alignedCropVOffset(total, size int, align cropVAlign) int {
	if size >= total {
		return 0
	}
	switch align {
	case cropAlignTop:
		return 0
	case cropAlignBottom:
		return total - size
	default:
		return (total - size) / 2
	}
}
