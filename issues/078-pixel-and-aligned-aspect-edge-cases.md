# 078 — Pixel/aligned aspect edge cases: distortion cap and downscale fallback

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Design
**Related**: [#077](077-cover-3x3-and-all-composite-modes-in-geometry-planner-and-fix-line-count-padding-mismatches.md)

---

## Current state

Pixel sizing is resolved: spec-owned 10% distortion/padding limits, one-cell
derived-axis limit, and nearest-neighbor resizing protect small images and
permit uniform repeats with padding. The interactive comparison demo is ready
for visual inspection. This issue remains open only for the `aligned` downscale
fallback decision. Glyph fitting and terminal calibration were deferred.

## 1. Original problem

`internal/viewgeom.PlanRender` single-dimension `aligned`/`pixel` paths (doom1.png 320×200):

| Mode | `-W` | aligned | pixel | Note |
|---|---|---|---|---|
| quad | 160 | 67 | **100** | pixel: ideal 0.67× vertical rounds to 1× → +50% stretch |
| 2x3/six/quad/half | 107 | 33–34 | 33–34 | `W·CellW < srcW` → no integer k ≥ 1, silently falls back to square-pixel `fitDimsRatio` |

The fallback changes the aspect convention; blending depends on the selected
prescaler. Default nearest-neighbor sampling itself does not blend colors.

## 2. Original decisions

1. **Distortion cap for `pixel`**: fall back to `aligned` when the integer row/col repeat deviates more than N% (e.g. 25%) from the aligned height?
2. **Downscale case**: when no integer upscale fits, should `aligned`/`pixel` use integer *downscale* (1/2, 1/3 nearest-neighbour), warn, or keep the current square-pixel fallback?

## 3. Resume point

Decide whether `aligned` should retain square-pixel fit for inputs too narrow
for an integer repeat, or use the same reference-lattice aspect as `pixel`.
The pixel paths, regression tests and policy loader are already implemented.

## 4. Pixel decision (2026-10-03)

User reproduction: Doom at `-W 54 -m 3x3 --aspect pixel` emitted 67 rows.
The 6×3 cell grid fits the source once horizontally (320 pixels plus 4 padding),
so the continuous vertical extent is `320·3·200·2/(6·320·3) = 66.67` pixels.
Clamping vertical repeats to at least 1 forced 200 pixels: 3× the intended height.
Width 107 similarly forced 200 instead of 133.33 pixels; width 160 correctly uses 200.

The first fix accepted whole-pixel repeats only within less than one mode cell
of the intended extent, before the percentage refinement in §5. Otherwise it used
nearest-neighbor scaling. The same rule limits blank padding on the constrained
axis. Too-small widths/heights use nearest-neighbor downsampling with the same
aspect formula, and pixel aliases override pyramid sampling to prevent blending.
Expected 3x3 rows: width 53 → 22, 54 → 23, 107 → 45, 160 → 67.
Quad at width 160 → 67 rows; six at width 160 remains 67 rows.

Existing goldens were unaffected: they use renderer fit geometry or explicit
native targets, not this single-dimension pixel path. A dedicated pixel sampling
golden was added with algorithm and sizing metadata.

Remaining open decision: `aligned` still uses the existing square-pixel fit fallback
when a source cannot fit at an integer upscale. Its downscale policy is unchanged.

Coverage: planner invariants across both axes, five cell geometries, all pixel
aliases and extents 1–321; CLI regression/control cases for widths 53/54/107/160,
quad and height-only input, plus playback preview. The new
`testdata/aspect-pixel/render_3x3_3x2.png` golden carries PNG tEXt algorithm,
aspect, source, canvas, sampling and snap-tolerance metadata. Its synthetic
12×12 RGB fixture also asserts every prepared pixel is the expected source color,
with nonzero source bounds and both nearest-neighbor and pyramid settings.

Validation: `HTO=0 make test` passed with both fast and reference paths, including
all existing goldens unchanged. `make preflight` passed and installed the updated
CLI/player/browser binaries. Live CLI and forwarded playback preview both emit
23 rows for the width-54 reproduction.

## 5. Small-image protection and padding preference

User accepted two follow-ups: protect small pixel extents against excessive
relative stretch and allow more padding for uniform integer pixel repeats.
The policy now lives in `spec/aspect.yaml`: `pixel.max_distortion: 0.10` and
`pixel.max_padding: 0.10`, with a matching schema and strict loader.
The CLI passes policy into the pure planner; missing spec disables optional
snapping, while malformed existing spec produces an error. There are no Go
copies of the defaults.

Before/after proofs:

- 12×12 source, 3x3 width 5: ideal 30×10; old repeat forced 30×12 (20% stretch).
  New content is 30×10 plus two transparent bottom subcells, still a 5×4 canvas.
- Doom width 110: old content 660×138 (uneven column repeats), canvas 110×46.
  New content 640×133 (uniform 2× columns), right padding 20/660 = 3.03%,
  bottom padding 2, canvas 110×45.
- Doom height 70: 210 available subcell rows; 200 source rows plus 10 padding
  use 4.76% padding. New content 960×200, canvas 160×70.
- Doom width 54 stays 320×67 plus right 4/bottom 2, canvas 54×23.

Existing goldens remained unchanged: the old 12×12 width-3 pixel fixture
needs 33.33% padding to use an integer horizontal repeat, so it still uses 18×6
content. Separate annotated small/padded pixel goldens were added; no renderer or glyph
selection algorithm changes. The aligned downscale decision remains open.

Validation: `HTO=0 make test` passed in both fast and reference modes; all existing
goldens remained unchanged. New small/padded goldens include algorithm, source,
canvas/content dimensions, sampling and policy limits in PNG tEXt metadata.
Planner tests cover both axes, aliases, padding/distortion limits, inclusive
percentage boundaries and zero-policy behavior. Spec tests verify schema bounds,
loader fidelity, missing-file behavior, and invalid/unknown/non-finite input.
`make preflight` passed, installed updated binaries, and verified demo renders.
Live checks: widths 54/107/110/119/160 emit 23/45/45/50/67 rows; height 70 emits
70 rows with 160 columns.

## 6. Interactive visual inspection

`scripts/demo_aspect.sh` calls the installed `cati` and compares current
`default`/`aligned`/`pixel` behavior on Doom at widths 12/54/107/110/160 and
the vacation photo at widths 12/54/110 (24 cases). It does not recreate old
binaries. Captions and exact commands appear below each render; dashed lines
separate cases, and all output remains in scrollback. Up/Down or `k`/`j` move
between cases, Enter advances, and `q`/EOF exits. Bounds stop navigation without
exiting, so the final case remains available for backward inspection.

All real renders, key sequence variants, boundaries and actual child-PTY
navigation passed. Each script revision passed preflight. The user will report
visible gaps by case number/command; automated success does not establish visual
acceptance. See `docs/Testing.md` for test scope and the Harnez PTY workaround.

The doc review also corrected an overstatement about physical aspect:
`aligned`/`pixel` use the 2:3 reference convention when applicable. Doom at width
160 produces 67 lattice rows; physically 1:2 terminal cells with square source
pixels would require 50. Prescaling preserves sampled colors, while later glyph
fitting may still approximate them. Those separate improvements remain deferred.
