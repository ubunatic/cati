# 093 — Fix octant glyph-to-subpixel mapping

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: [#084](084-add-octant-render-mode-using-unicode-1cd0-block-characters.md), [#086](086-add-vec-and-oct-golden-render-tests-and-investigate-mask-orientation.md), audit commit `e8f5a6c`

---

## 1. Problem & Motivation
`cati assets/samples/sample-003-darth-daughter.jpg -m oct -W 10` shows displaced edges and apparently misaligned blocks. Octant fitting uses masks that do not match the emitted Unicode glyphs, corrupting rendered coverage and color fitting.

## 2. Technical Specification / Findings
`spec/load.go:generatedOctantShapes` reserves 26 masks for older glyphs, then assigns the remaining masks sequentially to U+1CD00–U+1CDE5. Incorrect reservations shift the assignment: an audit of production shapes against Unicode 17.0 found **227 of 230 native octant glyphs have incorrect masks**. U+1CD00 (octant 3), for example, gets mask 0x06 instead of 0x04.

The reserved table also confuses sextants with octants and fractional block heights/orientations. The renderer consumes row-major masks correctly; the inventory is defective. The existing octant test passes because it checks geometry and inventory presence, not semantic mask correctness. Full audit evidence is recorded in #086; that ticket retains the broader vec/oct golden-coverage work.

## 3. Implementation & Verification Plan
**Goal**: Correct every octant glyph-to-subpixel mapping and verify rendering, or stop and report when blocked on a user decision or denied permission. Done means all 256 masks have independently verified Unicode coverage, regression tests detect the current errors, the reported render is checked, and relevant documentation and justified golden coverage are updated under the Rendering Bug Playbook.

Before implementation, check live code and recent commits and re-verify the audit against [Unicode data](https://www.unicode.org/Public/17.0.0/ucd/UnicodeData.txt). Run focused tests, the full suite, and required build/preflight checks; do not regenerate goldens merely to make tests pass.
