# 071 — Respect explicit width and height in static renders

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#028](028-v2-unconstrained-static-fit-aspect.md), [#029](029-v2-default-static-fit-full-terminal-width.md)

---

## 1. Problem & Motivation
When both `--width` and `--height` are explicitly set for a static render, Cati treats them as a fit box and may render fewer columns or rows to preserve the source aspect ratio. For example, `cati -m six assets/doom1.png --width 160 --height 67` currently emits 160 columns by only 50 rows. With both dimensions explicit, the requested cell grid should be the output resolution.

## 2. Technical Specification / Findings
The static path in `cmd/root.go` prepares images through `smartPrepare`, which uses `fitRenderedImageChecked` when smart sizing is off. That fit path preserves aspect ratio, so an explicit height acts as a cap. Mode cell geometry still determines the pixel extent represented by the requested grid. If the source already matches that extent except for an incomplete final cell, preserve its pixels and add transparent padding to complete the cell instead of rescaling it.

### Addendum: source padding and CLI dimension flags

Add `--pad <cols>,<rows>` to transparently pad the source image in pixels before rendering. This lets the Doom 1 six-mode golden use `--pad 0,1 --width 160 --height 67`: the extra transparent source row completes the final 2×3 cell row while the explicit dimensions select the target grid. This padding option is part of #071 and must work independently of the later `--aspect` mapping modes in #072.

Rename the short dimension flags to `--width|-W` and `--height|-H`. This frees `-w` for future use; `-h` remains reserved for help. Update CLI help and usage examples accordingly.

## 3. Implementation & Verification Plan
**Goal**: Make static renders honor an explicitly supplied width and height as the target cell grid, support transparent source padding via `--pad <cols>,<rows>`, preserve source pixels where they already fit, and rename the short dimension flags to `-W` and `-H`; stop and report if blocked on a user decision or denied permission.

Use `assets/doom1.png` and the custom Doom goldens as test assets. Verify that `-m six --pad 0,1 --width 160 --height 67` produces the expected 160×67 cell golden, and cover both a source that needs no scaling and one that requires scaling to the requested grid. Assert that aspect fitting does not silently reduce either explicitly requested dimension, padding is transparent and applied before rendering, and `-W`/`-H` work while `-h` still displays help.
