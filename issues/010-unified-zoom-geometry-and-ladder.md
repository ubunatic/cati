# 010 — Unified Zoom Geometry and Ladder

**Status:** ✅ Consolidated (Merged into [008 — Spaghetti Code & Viewport Geometry Refactoring Plan](008-spaghetti-code-analysis.md))  
**Refs:** [System.md](../docs/System.md#k-sequence-zoom-model-june-2026-revised-june-2026), [QuadPixelArt.md](../docs/QuadPixelArt.md), `cmd/interactive.go`

---

## Problem

The current zoom model needs to stay coherent across multiple renderer families. Much of the shared zoom math has now moved into `internal/viewgeom`, but the underlying problem remains: the viewer still needs one geometry model that can survive halfblock, quad, sparkline, and later glyph families without mode-specific drift.

The pain points:

- Halfblock and quad are really the same geometry at different resolutions.
- Future sparkline and curve/triangle glyphs need the same zoom math, but with different cell footprints.
- Small images accumulate too many visually redundant tail states if the ladder is defined as arithmetic `k` increments.
- Subcell offsets are a separate concern and should not be mixed into zoom itself.

## Direction

Use `src px / cell` as the user-visible zoom unit and model every renderer against a shared base cell quantum.

The ladder should:

1. derive candidate footprints from image dimensions and the active renderer mode
2. convert them to `src px / cell`
3. keep only states that change the rendered output after rounding
4. stop at the one-cell state without generating dead tail steps

## Goals

- One geometry model for halfblock, quad, sparkline, and future glyph families
- One zoom ladder implementation that is mode-aware but not mode-specific
- A separate subcell-offset axis for later quadshift testing controls
- A common analysis grid for SSIM and render-quality comparisons

## Notes

This is a refactor target, not a feature request for new zoom controls. The important thing is to remove the hidden assumption that zoom is a scalar `k` sequence and replace it with a footprint-based ladder that can survive new render modes.

The immediate mode-switch regression is tracked separately in [012](012-viewport-geometry-mode-switch-regression.md). Use that issue for the current bug; keep this one as the broader geometry refactor umbrella until the new model is complete and tested end to end.
