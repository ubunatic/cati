# 012 — Viewport Geometry Regression on Render-Mode Switch

**Status:** ✅ Consolidated (Merged into [008 — Spaghetti Code & Viewport Geometry Refactoring Plan](008-spaghetti-code-analysis.md))  
**Refs:** `cmd/interactive.go`, `cmd/ssim.go`, `internal/viewgeom/viewgeom.go`, `internal/quadblock/render.go`, [System.md](../docs/System.md#cell-quantum-zoom-model-june-2026-revised-june-2026)

---

## Repro

Run:

```bash
cati -i assets/samples/sample-001-soldering-practice-2025.jpg -q=0 --zoom 1:1
```

Observed:

1. Halfblock mode starts in a solid-looking 1:1 state.
2. The hint bar reports `src px/cell=1`.
3. Panning works initially.
4. Pressing `r` / `R` to switch render modes breaks the viewport geometry:
   - the image loses its natural aspect
   - width appears halved or height doubled
   - the zoom center shifts
   - panning can overflow at the borders

## What This Means

The viewer is still mixing two different geometry concepts:

1. the terminal-cell footprint used for rendering and hit-testing
2. the source-pixel footprint used for zoom / crop / SSIM math

The current refactor has started moving the shared math into `internal/viewgeom`, but the mode-switch path still needs a full pass so every consumer uses the same footprint model.

## Working Direction

The geometry model should be explicit about the cell footprint per mode:

- halfblock: `1x2`
- quad: `2x2`
- sparkline: `4x8`

That footprint must drive:

1. `viewportDims`
2. pan clamping
3. `ZoomAtCursor`
4. mode-switch recentering
5. SSIM reference generation

## Remaining Work

- finish the `internal/viewgeom` refactor so the app stops carrying geometry assumptions in `cmd/`
- make `r` / `R` recenter through the same source-center math as the other zoom transitions
- verify `buildViewport` and `buildRef` clamp against the same footprint used by rendering
- add regression tests for mode switching, not just isolated geometry helpers

## Notes

This is a follow-up to the zoom-ladder work. The ladder itself is not the main failure here; the failure is that mode switching still changes the effective geometry without a matching recenter and clamp pass.
