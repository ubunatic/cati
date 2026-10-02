# 069 — Add 2x4 dot-only braille render mode

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#025](025-spec-driven-render-modes.md), [#009](009-explore-more-sparkline-rendering-modes.md)

---

## 1. Problem & Motivation
Add a render mode that uses the eight dot positions in each Unicode braille cell as image pixels. Only foreground dots represent pixels; the undotted area does not carry image color.

## 2. Technical Specification / Findings
Use the 2×4 braille dot geometry and braille pattern glyphs. Preserve the distinction between this dot-only mode and a full-cell mode that colors the background.

## 3. Implementation & Verification Plan
**Goal**: Implement and verify a selectable 2×4 dot-only braille render mode, or stop and report if blocked on a user decision or denied permission. The mode is done when it maps source pixels to braille dot patterns, integrates with the render-mode configuration and selection flow, and has focused tests and documented behavior.
