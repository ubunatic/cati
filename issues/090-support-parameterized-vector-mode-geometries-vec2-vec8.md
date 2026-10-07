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

## 4. Decision 2026-10-07: `vec` is an alias of a fixed mode
The separate 4x4 `vector` mode and glyph set 50 are removed. `vector`, `v` and `vec`
are aliases of `vec6` (4x6), which joins the default cycle and leaves `experimental`.
Fixed modes `vec2..vec8` keep their geometry; retargeting `vec` is a spec-only edit:
- `TestSpecRenderModesIntegrity` reads each mode's `aliases` and requires them to select
  that mode, and requires a `glyph_union` mode's `cell` to equal its resolved geometry.
- No test names `vec`/`v`/`vector`; the 4x4 straight-bar test calls
  `vectorShapes(spec, 4, 4)` directly.
- Vector shapes moved from Go (`generatedVectorShapes`) to `vector_shapes:` in
  `spec/render_modes.yaml`. Masks stayed identical at 2x2..6x9, except two tie pixels:
  at 3x5 (vec5) and 1x1 the centre pixel of `▚`/`▞` is exactly 50% covered and is now
  on (coverage of the union) where the old OR of quadrant masks left it off.
- Sextants `🬃🬇🬋` fixed in the spec to their Unicode segments (middle third, left/right/
  full); no golden changed (no golden renders a vector mode except `diag`, an exact `🭄`).
- Rect edges snap to pixel boundaries (.5 up). At odd sizes `▖` (3x5) was notched:
  50%-covered edge pixels on, the 25% corner pixel off. Now quadrant pairs tile their
  half exactly at every vec geometry; vec6/vec8 masks unchanged; no golden changed.
- Hand-drawn per-size masks (`masks: { 3x5: [...] }` on a glyph line) override the
  computed mask; first use: tips of 🭬/🭮 at 3x5. More user tuning to follow.
- The `diag` reconstruction probe is now one 4x6 `vec6` cell and reconstructs exactly
  (golden `testdata/reconstruction_probes/diag.png` regenerated from the fixture script).
