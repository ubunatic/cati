# 071 — Respect explicit width and height in static renders

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#028](028-v2-unconstrained-static-fit-aspect.md), [#029](029-v2-default-static-fit-full-terminal-width.md)

---

## 1. Problem & Motivation
When both `--width` and `--height` are explicitly set for a static render, Cati treats them as a fit box and may render fewer columns or rows to preserve the source aspect ratio. For example, `cati -m six assets/doom1.png -w 160 --height 67` currently emits 160 columns by only 50 rows. With both dimensions explicit, the requested cell grid should be the output resolution.

## 2. Technical Specification / Findings
The static path in `cmd/root.go` prepares images through `smartPrepare`, which uses `fitRenderedImageChecked` when smart sizing is off. That fit path preserves aspect ratio, so an explicit height acts as a cap. Mode cell geometry still determines the pixel extent represented by the requested grid. If the source already matches that extent except for an incomplete final cell, preserve its pixels and add transparent padding to complete the cell instead of rescaling it.

## 3. Implementation & Verification Plan
**Goal**: Make static renders honor an explicitly supplied width and height as the target cell grid, preserving source pixels where they already fit and padding incomplete mode cells as needed; stop and report if blocked on a user decision or denied permission.

Verify that `-w 160 --height 67` produces a 160×67 cell render for `six`, and cover both a source that needs no scaling and one that requires scaling to the requested grid. Assert that aspect fitting does not silently reduce either explicitly requested dimension.
