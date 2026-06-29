# 015 — Spark/quad garbled bottom row at mid-cell fit heights

**Status:** ✅ Closed  
**Refs:** [SparklinePixelArt.md](../docs/SparklinePixelArt.md), `internal/imgutil/scale.go` (`FitDims`), `internal/sparkline/render.go`

---

## Problem

In `spark/quad`, the bottom terminal-char row rendered garbled for many fit
widths (reported at `-w 3, 6, 10, 24, 28, 31`; `-w 5` looked correct). The
transparent extension half-row appended at the bottom of the last row showed the
wrong colour — the BG of the last opaque row bled into the area that should be
transparent (compare the correct reference `testdata/demo_cross_20x20/render_spark_5ch.png`).

Repro: `cati assets/baby-360p.mp4 -m=sq --range 1s: -w 10`.

## Root Cause

`FitDims` aligned the partial last char row to the *full-cell* remainder. A
terminal char is two stacked half-cells (`CellH/2` px each). When the
aspect-preserving height ended mid-cell with a remainder of at least half a cell
(`rem ∈ {5,6,7}` for spark's `CellH=8`), it kept **all** `rem` content rows and
padded only `CellH − rem` transparent rows — e.g. 6 content + 2 transparent.

No block glyph represents a mid-cell split like 6/2, so `findBestCandidate` fell
back to quadrant/diagonal chars (`▌ ▚ ▘`) whose coloured region spanned the
transparent rows. `RenderOpts` then emitted a BG sequence that the terminal
painted across the whole char, bleeding colour into the transparent half.

Only spark was affected: halfblock/quad use `CellH=2`, so `rem ∈ {0,1}` and the
case never arises. The good cases lined up exactly with the math: `rem=4`
(clean `▀`) and `rem<4` (floored away, no transparency).

## Solution

`FitDims` now snaps the partial last row to the **nearest half-cell boundary**:

- nearer a top half-cell → keep `CellH/2` content rows, append `CellH/2`
  transparent rows (`extH = CellH/2`) → clean `▀`.
- nearer a full cell (`rem=7`) → round up to a full content cell (`extH = 0`),
  resampling ~1px rather than cutting content.

Result: `extH ∈ {0, CellH/2}` always, the last char is always a representable
glyph, and the half-char transparency invariant still holds. The earlier
`render.go` BG-suppression band-aid was reverted — with the geometry corrected
the renderer is naturally right (the `▀` BG region is fully transparent, so no
BG sequence is emitted, identical to the cross golden).

## Golden Impact

Only `sample_summer_vacation/render_spark_24ch.png` and `render_spark_30ch.png`
changed (source 1042×1383 lands at `rem=7`); both now render a full content
bottom row instead of the mid-cell remainder. All square-source goldens scale to
`rawH = 4n` (`rem ∈ {0,4}`) and are unaffected. `TestGoldenTransparentBound`
still holds (`extH ≤ CellH/2`).
