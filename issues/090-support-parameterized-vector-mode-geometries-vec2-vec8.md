# 090 — Support parameterized vector mode geometries (vec2..vec8)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: #089, #091

---

## 1. Problem & Motivation
`vector` uses a fixed 4x4 subpixel lattice. On a 1:2 terminal cell this gives
1:2 subpixels, and the Teletext G3 endpoints (thirds vertically) do not land on
grid lines. We want experimental modes to compare lattices.

## 2. Technical Specification / Findings
Proposed modes (cell w x h):

| Mode | Geometry | Note |
|------|----------|------|
| vec2 | 2x2 | coarse baseline |
| vec3 | 2x3 | native Teletext sextant lattice |
| vec4 | 3x4 | |
| vec5 | 3x5 | odd center row/column |
| vec6 | 4x6 | all 10 G3 endpoints are exact integer grid points |
| vec8 | 4x8 | square subpixels on 1:2 cells |

Masks must be generated from geometry (perimeter endpoints + named corner,
supersampled area >= 50%, complement pairs forced exact), not hand tables.
A Python prototype of the generator ran in-session for all six geometries.

## 3. Implementation & Verification Plan
1. Replace the 4x4 `vectorRuneMasks` table in `spec/load.go` with a
   `generatedVectorShapes(w, h)` generator; keep 4x4 output identical.
2. Add `vec2..vec8` glyph sets and modes in `spec/render_modes.yaml` (mark experimental).
3. Spec tests: per geometry, mask length = w*h, complement pairs XOR to all-ones.
4. Do after #091 so straight-line glyphs are included in every geometry.
