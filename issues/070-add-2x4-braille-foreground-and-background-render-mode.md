# 070 — Add 2x4 braille foreground and background render mode

**Status**: Resolved
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#025](025-spec-driven-render-modes.md), [#009](009-explore-more-sparkline-rendering-modes.md)

---

## 1. Problem & Motivation
Add a render mode that uses a full Unicode braille cell for image pixels: foreground-colored dots and a background fill for the undotted area. This provides full-cell coverage while retaining the 2×4 dot pattern.

## 2. Technical Specification / Findings
Use the 2×4 braille dot geometry. Map dot pixels to the foreground and use the cell background for the complementary area so both foreground and background colors represent the source image.

## 3. Implementation & Verification Plan
**Goal**: Implement and verify a selectable 2×4 braille foreground-and-background render mode, or stop and report if blocked on a user decision or denied permission. The mode is done when it maps source pixels to braille patterns and foreground/background colors, integrates with render-mode configuration and selection, and has focused tests and documented behavior.


## Resolution
Implemented `v1/braille` package and `braille` (`b8`, `2x4`) foreground/background render mode registered in `spec/render_modes.yaml`. Verified with unit tests and benchmarks.