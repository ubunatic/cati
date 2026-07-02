# 026 — Small image aspect ratio and scale bugs for spark/sextant modes

**Status:** ✅ Closed  
**Refs:** `internal/imgutil/scale.go` (`AlignCellSize`), `cmd/interactive.go` (`render`), `cmd/render_pipeline.go` (`prepareRenderedImageChecked`)

---

## Problem

When rendering small images (like `testdata/checkerboard_4x4.png` which is 4x4 pixels) at 1:1 zoom (`-z=1`), modes with larger cell footprints (like `s`, `x`, `sq`, `sx`, `xh`) suffer from aspect ratio distortion, wrong scale, and/or unexpected transparency padding at the bottom of the rendered block.

Reproduction command:
```bash
img=testdata/checkerboard_4x4.png
for a in h hs q s x sq sx xh; do echo "$a $img:"; ./cati $img -m=$a -z=1; done
```

Output:
```
h testdata/checkerboard_4x4.png:
▀▀▀▀
▀▀▀▀ <-- OK (4x2 cells, visual aspect 1:1)

hs testdata/checkerboard_4x4.png:
▀▀▀▀
▀▀▀▀ <-- OK (4x2 cells, visual aspect 1:1)

q testdata/checkerboard_4x4.png:
▀▀▀▀
▀▀▀▀ <-- OK (4x2 cells, visual aspect 1:1)

s testdata/checkerboard_4x4.png:
▀ <-- too small, BG was transparent (renders 1x1 cells, should render 1x1 cells but scaled up to fill the 4x8 cell without bottom transparency)

x testdata/checkerboard_4x4.png:
🬧🬧 <-- too small, aspect OK (renders 2x1 cells, aligned to 4x3)

sq testdata/checkerboard_4x4.png:
▀ <-- too small, BG was transparent (renders 1x1 cells)

sx testdata/checkerboard_4x4.png:
  <-- single colored space, too small wrong aspect (renders 1x1 cells, image scaled to 4x4 but cell expects 4x24)

xh testdata/checkerboard_4x4.png:
▀▀ <-- transparent BG at bottom, wrong aspect (renders 2x1 cells, image scaled to 4x4 but cell expects 2x6)
```

## Root Cause

During 1:1 zoom rendering (`prepareRenderedImageChecked`), the source dimensions are scaled by zoom `zoom` to `dims.ScaledW, dims.ScaledH` and then passed to `imgutil.AlignCellSize(dims.ScaledW, dims.ScaledH, spec.CellW, spec.CellH)` to snap to the cell boundary.

In `internal/imgutil/scale.go`:
```go
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
```

When the scaled dimensions are smaller than a single cell size (i.e. `w <= cellW` or `h <= cellH`), `AlignCellSize` does not align them to the cell boundary, returning the unaligned dimensions (e.g. `w = 4`, `h = 4` for `s` mode where `cellW = 4, cellH = 8`).

This causes:
1. The scaled image size (e.g., 4x4) to not be a multiple of cell size (e.g., 4x8).
2. The cell grid to render fewer cells, and since the image is smaller than the cell, the bottom rows of the cell are treated as transparent, causing color bleeding/wrong glyph selection (like `▀` instead of the full block/pattern).
3. Visual aspect ratio distortion when the transparent padding is included or when the cell is not fully covered.

Similarly, in `imgutil.FitDims`:
```go
	// Align width down to a whole cell (height is already cell-aligned above).
	targetW = rawW
	if cellW > 1 && targetW > cellW {
		targetW -= targetW % cellW
	}
```
If `targetW <= cellW`, it remains unaligned.

## Proposed Fix

Update `AlignCellSize` (and potentially `FitDims`) to ensure that:
1. Width and height are always aligned to a multiple of `cellW` and `cellH`.
2. The minimum aligned dimension is at least `cellW` and `cellH` respectively (i.e. at least 1 cell).

Example update to `AlignCellSize`:
```go
func AlignCellSize(w, h, cellW, cellH int) (int, int) {
	if cellW > 1 {
		if rem := w % cellW; rem != 0 {
			w -= rem
		}
		if w < cellW {
			w = cellW
		}
	}
	if cellH > 1 {
		if rem := h % cellH; rem != 0 {
			h -= rem
		}
		if h < cellH {
			h = cellH
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
```

## Resolution

Fixed by:

- Making `AlignCellSize` promote positive sub-cell dimensions to at least one
  complete render cell while still rounding larger dimensions down to cell
  boundaries.
- Making static explicit zoom (`--zoom=1`, `100%`, `1:1`, positive `k`) derive
  terminal-cell dimensions from source dimensions (`cols = ceil(srcW/k)`,
  `rows = ceil(srcH/(2k))`) before expanding to the active render mode's
  internal pixel grid. This preserves the documented `k=1` semantics across
  all glyph algorithms even when no `--width`/`--height` is supplied.
- Adding `TestAlignCellSizePromotesSubCellDimensions` and
  `TestAllRenderModesZoomOneSmallSquareUseCompleteCells`.
