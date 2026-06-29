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
//   - targetW, targetH: dimensions to resize to (aligned to cell boundaries)
//   - extH: transparent rows to append after resize; non-zero only when the
//     natural height extends into the next cell by at least half a cell — this
//     keeps the partial char row visible while limiting BG transparency to ≤ CellH/2
//     pixels per column (half a char).
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

	// Partial extension: when rawH is not aligned to cellH and the remainder
	// occupies at least half a cell, resize to rawH (keeping the partial row
	// visible) and append only the remaining transparent pixels.
	//
	// The guard rawH >= cellH was intentionally removed: we also extend images
	// shorter than one cell (rawH < cellH) so that block renderers receive a
	// complete input block and the half-char invariant (≤ CellH/2 transparent
	// rows) still holds.
	if cellH > 1 {
		if rem := rawH % cellH; rem > 0 && rem*2 >= cellH {
			targetH = rawH
			extH = cellH - rem
		}
	}

	return targetW, targetH, extH
}
