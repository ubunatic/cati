# 086 — Add vec and oct golden render tests and investigate mask orientation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#083](083-add-vector-render-mode-using-linear-cell-split-characters.md), [#084](084-add-octant-render-mode-using-unicode-1cd0-block-characters.md), [Rendering Bug & Golden-Change Playbook](../docs/RenderingBugPlaybook.md)

---

## 1. Problem & Motivation
Some vector (`vec`) masks appear to be applied in the wrong direction, as shown in the reported screenshot. The new `vec` and `oct` modes also need inspectable golden renders so mask-orientation bugs can be diagnosed and their expected output stabilized as the modes evolve.

## 2. Technical Specification / Findings
The specific glyph masks and orientations that are wrong are not yet identified. Establish a small, representative golden set for both modes that makes directional and asymmetric coverage easy to inspect; include descriptive algorithm and parameter metadata in the PNGs.

## 3. Implementation & Verification Plan
**Goal**: Add inspectable golden render tests for `vec` and `oct`, use them to identify and correct the reported vector mask-direction error, or stop and report if blocked on a user decision or denied permission. Done means the goldens cover representative orientations for both modes, mask application has focused regression coverage, and every golden change is justified by the rendering behavior it records.

## 4. Octant mapping audit (2026-10-06)

Reported input: `cati assets/samples/sample-003-darth-daughter.jpg -m oct -W 10`.
The current `spec/load.go:generatedOctantShapes` maps 256 row-major 2x4 masks
to glyphs, reserving 26 supposed pre-existing shapes and assigning the remaining
230 sequentially from U+1CD00. The reserved masks are incorrect, so the
sequential assignment is also incorrect.

Comparing the actual `ResolveGlyphSetExpression("octant")` output against
[Unicode 17.0 UnicodeData](https://www.unicode.org/Public/17.0.0/ucd/UnicodeData.txt)
finds **227 of 230 native octant glyphs have incorrect masks** (only 3 agree).
For each `BLOCK OCTANT-<digits>` name, the authoritative mask is the bitwise
union of `1 << (digit - 1)`; compare that with the production shape's boolean mask.
Examples: U+1CD00 (octant 3) gets 0x06 instead of 0x04; U+1CD01
(octants 2,3) gets 0x07 instead of 0x06; U+1CD02 (octants 1,2,3)
gets 0x09 instead of 0x07.

Independent errors in the reserved table include U+1FB00 (a sextant, height
1/3 rather than 1/4), U+1FB02 (sextants 1,2 rather than a single octant),
U+2594 (upper 1/8 rather than upper 1/4), and U+2587 (lower 7/8 rather than
upper 3/4). Mask 0xE0 is an asymmetric three-octant shape, not the horizontal
lower-three-eighths block U+2583.

The custom-shape renderer consumes masks in row-major order, so these findings
identify a glyph inventory defect rather than a transposed indexing convention.
Its fitting calculations use the incorrect masks, while the terminal displays
the actual Unicode glyph coverage. This can move edges and create apparent
alignment defects independently of font behavior.

`go test ./spec -run TestOctantAndVectorGlyphSets -count=1` passes: it checks
geometry, inventory size, and presence of native octant glyphs, but not their
semantic correspondence to masks. No rendering code or golden images changed
during this investigation. Fix the inventory and add independent full-table
mapping assertions before treating octant goldens as trusted output.
