package imgutil

import "image"

// ScaleNN resizes img to w×h using nearest-neighbour sampling anchored at pixel
// centres. This avoids over-representing the first column/row when downscaling
// and works correctly for upscaling too.
func ScaleNN(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 || w < 1 || h < 1 {
		return img
	}
	if srcW == w && srcH == h {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := b.Min.Y + centeredSourceIndex(y, srcH, h)
		for x := 0; x < w; x++ {
			dst.Set(x, y, img.At(b.Min.X+centeredSourceIndex(x, srcW, w), srcY))
		}
	}
	return dst
}

// centeredSourceIndex maps destination pixel dst to a source pixel index, anchoring
// the sample at the pixel centre to avoid over-representing the leading edge.
func centeredSourceIndex(dst, srcN, dstN int) int {
	return ((2*dst+1)*srcN - 1) / (2 * dstN)
}

// AlignCellSize rounds w and h down to the nearest multiple of cellW and cellH
// respectively. Returns at least 1×1.
func AlignCellSize(w, h, cellW, cellH int) (int, int) {
	if cellW > 1 && w > cellW {
		if rem := w % cellW; rem != 0 {
			w -= rem
		}
	}
	if cellH > 1 && h > cellH {
		if rem := h % cellH; rem != 0 {
			h -= rem
		}
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// FillTransparentRows returns a copy of img where every fully-transparent row at
// the bottom is replaced by the last opaque row. This prevents block renderers
// (e.g. sparkline) from treating BG-extension rows as black pixels, which would
// corrupt character selection for the partial char at the image boundary.
// If the image has no transparent tail, the original is returned unchanged.
func FillTransparentRows(img image.Image) image.Image {
	b := img.Bounds()
	lastOpaque := -1
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		_, _, _, a := img.At(b.Min.X, y).RGBA()
		if a != 0 {
			lastOpaque = y
			break
		}
	}
	if lastOpaque < 0 || lastOpaque == b.Max.Y-1 {
		return img
	}
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		srcY := y
		if y > lastOpaque {
			srcY = lastOpaque
		}
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x, y-b.Min.Y, img.At(x, srcY))
		}
	}
	return out
}

// AppendTransparentRows returns a new image with addH fully-transparent rows
// appended at the bottom. The transparent rows signal that the content does not
// fill that terminal char row.
func AppendTransparentRows(img image.Image, addH int) image.Image {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()+addH))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x, y-b.Min.Y, img.At(x, y))
		}
	}
	// Rows [b.Dy()..b.Dy()+addH-1] remain zero-initialized (transparent).
	return out
}

// FitDims computes pixel dimensions for fitting a srcW×srcH source into a
// cols×rows terminal viewport with the given cell geometry, allowing upscale.
//
// Returns:
//   - targetW, targetH: dimensions to resize to (targetH snapped to a half-cell
//     boundary so the last char row is always a representable glyph)
//   - extH: transparent rows to append after resize; either 0 or exactly CellH/2.
//     It is CellH/2 only when the natural height ends on a top half-cell, so the
//     partial char renders as a clean upper-half block (▀) with its transparent
//     bottom half left for the terminal background — never more than half a char.
//
// Resolution-independent geometry: every render mode is built so that
// CellW/(AspectX·CellH) = 1/2, which means the *continuous* display height in
// char rows, srcH·cols/(2·srcW), is identical across modes. The half-cell snap
// below is therefore computed from that continuous height (carried as the exact
// ratio hNum/hDen), NOT from a height already floored to integer pixels. Flooring
// first would lose up to ~half a char at low resolutions (halfblock/quad at
// 2 px/char) but almost nothing at high resolution (spark at 8 px/char), so the
// modes would disagree on the bottom-row geometry for the same source and width.
//
// Either cols or rows may be 0 (unconstrained). Both 0 returns source dimensions.
func FitDims(srcW, srcH, cellW, cellH, aspectX, cols, rows int) (targetW, targetH, extH int) {
	if srcW == 0 || srcH == 0 {
		return max(1, cols*cellW), max(1, rows*cellH), 0
	}

	maxW, maxH := cols*cellW, rows*cellH

	// Compute aspect-preserving dimensions, allowing upscale.
	// Unconstrained dimensions are handled separately to avoid integer overflow
	// when the unconstrained max would be MaxInt. When height is the *derived*
	// dimension we keep its exact continuous value as hNum/hDen (px = hNum/hDen)
	// so the half-cell snap is not corrupted by integer-pixel flooring.
	acSrcW := srcW * aspectX
	var rawW int
	var hNum, hDen int // continuous derived height in px = hNum/hDen
	heightDerived := true
	switch {
	case cols <= 0 && rows <= 0:
		rawW, hNum, hDen = acSrcW, srcH, 1
	case rows <= 0: // width-only constraint
		rawW, hNum, hDen = maxW, srcH*maxW, acSrcW
	case cols <= 0: // height-only constraint — height is fixed (exact cells)
		targetH, rawW, heightDerived = maxH, max(1, acSrcW*maxH/srcH), false
	default: // both constrained — safe to compare (no overflow)
		if acSrcW*maxH >= srcH*maxW { // width-bound, height derived
			rawW, hNum, hDen = maxW, srcH*maxW, acSrcW
		} else { // height-bound — height is fixed (exact cells)
			targetH, rawW, heightDerived = maxH, max(1, acSrcW*maxH/srcH), false
		}
	}

	if heightDerived {
		// Half-cell snap: a terminal char is two stacked half-cells (CellH/2 px
		// each). For the partial last char row to map onto a representable block
		// glyph, its content must align to a half-cell boundary — otherwise no
		// glyph can describe a mid-cell remainder (e.g. 6 content + 2 transparent
		// rows), and the renderer falls back to wrong quadrant/diagonal chars
		// whose colour bleeds into the transparent area (garbled bottom row).
		//
		// Decide from the CONTINUOUS height hNum/hDen. Scale by hDen to stay in
		// integers: cellScaled = CellH·hDen, the fractional part remScaled lives
		// in [0, cellScaled). Round the fractional char to the nearest half-cell:
		//   - below half a cell        → drop it (full content cells only)
		//   - nearer a top half-cell   → keep it + append CellH/2 transparent rows
		//                                (clean ▀); extH = CellH/2
		//   - nearer a full cell       → round up to a full content cell; extH = 0
		// Because hNum/hDen reduces to the same char-height for every mode, the
		// chosen rows and bottom-row fill are identical across modes.
		half := cellH / 2
		if cellH > 1 && half > 0 {
			cellScaled := cellH * hDen
			halfScaled := half * hDen
			full := hNum / cellScaled
			rem := hNum - full*cellScaled // continuous fractional px, ×hDen
			switch {
			case rem < halfScaled: // below half a cell → drop
				targetH = full * cellH
			case 2*rem <= cellScaled+halfScaled: // nearer top half-cell → ▀
				targetH = full*cellH + half
				extH = half
			default: // nearer full cell → round up
				targetH = (full + 1) * cellH
			}
		} else {
			targetH = max(1, hNum/hDen)
		}
		if targetH < 1 {
			targetH = cellH
		}
	}

	// Align width down to a whole cell (height is already cell-aligned above).
	targetW = rawW
	if cellW > 1 && targetW > cellW {
		targetW -= targetW % cellW
	}
	if targetW < 1 {
		targetW = 1
	}

	return targetW, targetH, extH
}
