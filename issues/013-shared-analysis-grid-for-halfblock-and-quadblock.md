# 013 — Shared Analysis Grid for Halfblock and Quadblock Convergence

**Status:** 🔴 Open  
**Refs:** `internal/halfblock/render.go`, `internal/quadblock/render.go`, `internal/quadblock/algos.go`, `internal/sparkline/testhelper/testhelper.go`, [QuadPixelArt.md](../docs/QuadPixelArt.md)

---

## Problem

At small output sizes, halfblock and quadblock can render the same source image very differently even when both algorithms are individually correct. The difference is most visible on the `demo_horiz_20x20` bench at `2x2` terminal cells:

- `halfblock` and `quadblock` do not sample the source through the same footprint
- each mode makes a valid but different approximation of the same image
- the divergence is acceptable at larger sizes, but becomes visually abrupt on tiny outputs

The `5x5` outputs look reasonable in both modes, which suggests the cell compilers are not the primary issue. The mismatch comes from the analysis geometry feeding them.

## Question

Should Cati introduce a shared intermediate analysis grid so that halfblock and quadblock can converge on small images?

Possible directions:

1. **Shared low-res analysis grid** — both renderers consume the same coarse intermediate image before choosing glyphs.
2. **Shared high-res canonical grid** — both renderers consume the same supersampled image before choosing glyphs.
3. **Explicit analysis-mode enum** — `coarse`, `canonical`, `supersampled`, selected as a first-class option instead of implicit renderer behavior.

## Why This Matters

This is not about making the renderers identical in all cases. It is about giving them the same evidence when the output is so small that their current footprints amplify tiny pixel differences into visibly different results.

The current behavior is defensible, but it is not always desirable for bench/demo assets where the goal is consistency across render modes.

## Notes

Do not treat this as a bug in the quad glyph chooser. The vertical `▌`/`▐` work improved quadblock’s local glyph selection, but it did not address the shared-footprint question between the two renderers.
