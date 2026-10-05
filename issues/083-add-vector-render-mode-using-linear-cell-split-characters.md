# 083 — Add vector render mode using linear cell split characters

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#014](014-more-boxdrawing-chars-unicode-v13.md), [#025](025-spec-driven-render-modes.md)

---

## 1. Problem & Motivation
Add a `vector` render mode (with a short alias if appropriate) that represents image coverage with linear and other cell-splitting characters. Include the supplied U+1FB3C–U+1FB6F shapes, U+1FB9A and U+1FB9B (`🮚`, `🮛`), half blocks, and other useful cell splitters. Exclude pixel-like sextant and quadrant glyphs.

## 2. Technical Specification / Findings
The requested glyphs provide slanted and fractional cell boundaries that are distinct from pixel-grid sextant and quadrant modes. The existing Unicode glyph expansion work in #014 covers related source glyph families but does not define this selectable `vector` mode or its exclusions.

## 3. Implementation & Verification Plan
**Goal**: Implement and verify a selectable vector render mode using linear cell-split glyphs, or stop and report if blocked on a user decision or denied permission. Done means the mode uses the requested glyph families and half/cell splitters, excludes pixel-like sextants and quads, and has focused tests and documented behavior.
