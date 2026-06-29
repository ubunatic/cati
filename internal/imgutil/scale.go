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
// Either cols or rows may be 0 (unconstrained). Both 0 returns source dimensions.
func FitDims(srcW, srcH, cellW, cellH, aspectX, cols, rows int) (targetW, targetH, extH int) {
	if srcW == 0 || srcH == 0 {
		return max(1, cols*cellW), max(1, rows*cellH), 0
	}

	maxW, maxH := cols*cellW, rows*cellH

	// Compute aspect-preserving dimensions, allowing upscale.
	// Unconstrained dimensions are handled separately to avoid integer overflow
	// when the unconstrained max would be MaxInt.
	acSrcW := srcW * aspectX
	var rawW, rawH int
	switch {
	case cols <= 0 && rows <= 0:
		rawW, rawH = srcW, srcH
	case rows <= 0: // width-only constraint
		rawW = maxW
		rawH = max(1, srcH*maxW/acSrcW)
	case cols <= 0: // height-only constraint
		rawH = maxH
		rawW = max(1, acSrcW*maxH/srcH)
	default: // both constrained — safe to compare (no overflow)
		if acSrcW*maxH >= srcH*maxW { // width-bound
			rawW = maxW
			rawH = max(1, srcH*maxW/acSrcW)
		} else { // height-bound
			rawH = maxH
			rawW = max(1, acSrcW*maxH/srcH)
		}
	}

	targetW, targetH = AlignCellSize(rawW, rawH, cellW, cellH)

	// Half-cell snap: a terminal char is two stacked half-cells (CellH/2 px
	// each). For the partial last char row to map onto a representable block
	// glyph, its content must align to a half-cell boundary — otherwise no
	// glyph can describe a mid-cell remainder (e.g. 6 content + 2 transparent
	// rows), and the renderer falls back to wrong quadrant/diagonal chars whose
	// colour bleeds into the transparent area (garbled bottom row).
	//
	// When the natural height ends mid-cell with a remainder of at least half a
	// cell, snap it to the NEAREST half-cell boundary:
	//   - closer to one half-cell of content → keep a top half-cell and append
	//     CellH/2 transparent rows; the char renders as a clean ▀.
	//   - closer to a full cell → round up to the full cell with no transparent
	//     padding; the char renders as a full content row.
	// Either way the half-char transparency invariant (≤ CellH/2 transparent
	// rows) holds and the last char is always a representable glyph. Remainders
	// below half a cell are dropped by AlignCellSize above (unchanged).
	if cellH > 1 {
		half := cellH / 2
		if rem := rawH % cellH; rem >= half {
			lowH := rawH - rem
			if rem-half <= cellH-rem { // nearer to lowH+half than to lowH+cellH
				targetH = lowH + half
				extH = half
			} else {
				targetH = lowH + cellH
				extH = 0
			}
		}
	}

	return targetW, targetH, extH
}
