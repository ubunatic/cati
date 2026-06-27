# 011 — Sparkline Inversion and Stripe Bugs

**Status:** ✅ Closed  
**Refs:** [System.md](../docs/System.md), `internal/sparkline/sparkline.go`, `internal/sparkline/render.go`

---

## Problem

The sparkline modes other than `spark/lower` exhibited bugs:
1. `spark/left` and `spark/right` produced unwanted comb lines or stripe artifacts.
2. `spark/upper` flipped the fill heights (e.g. 1/8 filled top bar rendered as 7/8 filled top bar).
3. `spark/right` also had similar thick comb lines and flipped fill levels.

## Root Cause

1. **Stripe/Comb Line Bug:** In `readBlock`, the pixels of the cell block were always read in row-major order (horizontal scanning). In vertical modes (`LeftVertical` and `RightVertical`), the bar grows horizontally (left-to-right/right-to-left). Analyzing a 1D pixel array generated from row-major scanning resulted in splits crossing horizontal rows rather than vertical columns, causing complete mismatch between the analyzed block geometry and the rendered vertical block.
2. **Flipped Fill Bug:** In `findOptimalSplit`, when the split index `ci` (0 to 7) was evaluated for `UpperHorizontal` and `RightVertical`, the winning index `bestK` was stored as `ci`. However, because upper/right block elements do not exist in Unicode, they are simulated by swapping the foreground and background of lower/left block elements. Swapping FG/BG on a block of height/width `K` actually represents a fill level of `7 - K`. Thus, a split evaluated at level `ci` must be mapped to the character index `6 - ci` to render at the expected fill height. Without this mapping, the cells were vertically/horizontally flipped.

## Solution

1. **Column-Major Reading:** Modified `readBlock` to read pixel data column-major when the mode is `LeftVertical` or `RightVertical`.
2. **Swapped Index Mapping:** Fixed `findOptimalSplit` and `Char` to map evaluated fill levels `ci` to `6 - ci` when selecting the character index for `UpperHorizontal` and `RightVertical` modes.
3. **Golden Tests & Helper:** Created a new package `internal/sparkline/testhelper` containing utilities to generate gradient test PNGs (20x20 down to 1x1, utilizing Blue and Yellow gradients) and organize them into individual subdirectories in `testdata/`. The helper also wraps and tests the other rendering algorithms (`halfblock` and `quadblock`) by resizing their outputs back to the original source size for visual side-by-side comparison. The Go test verifies all modes, algorithms, and sizes pixel-for-pixel against golden renders.
4. **Removal of Redundant Modes**: Removed the `spark/upper` and `spark/right` modes entirely from the `sparkline` package, command configurations, tests, and test helpers to reduce complexity and redundancy.
