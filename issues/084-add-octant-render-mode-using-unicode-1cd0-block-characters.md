# 084 — Add octant render mode using Unicode 1CD0 block characters

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#025](025-spec-driven-render-modes.md)

---

## 1. Problem & Motivation
Add a new `octant` render mode with `oct` as its short mode name. It should use the Unicode octant block characters shown in the request, covering U+1CD00–U+1CDE5, to represent image coverage at eight-part cell resolution.

## 2. Technical Specification / Findings
The requested range includes the octant block glyph inventory in Unicode blocks U+1CD0 through U+1CDE. Preserve the intended glyph mapping and use the available glyphs in that range; do not substitute sextant or quadrant render modes.

## 3. Implementation & Verification Plan
**Goal**: Implement and verify selectable `octant`/`oct` rendering with the requested U+1CD00–U+1CDE5 glyph inventory, or stop and report if blocked on a user decision or denied permission. Done means the mode is integrated into mode configuration and selection, with focused tests and documented behavior.
