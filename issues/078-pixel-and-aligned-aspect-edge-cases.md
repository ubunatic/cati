# 078 — Pixel/aligned aspect edge cases: distortion cap and downscale fallback

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Design
**Related**: [#077](077-cover-3x3-and-all-composite-modes-in-geometry-planner-and-fix-line-count-padding-mismatches.md)

---

## 1. Problem

`internal/viewgeom.PlanRender` single-dimension `aligned`/`pixel` paths (doom1.png 320×200):

| Mode | `-W` | aligned | pixel | Note |
|---|---|---|---|---|
| quad | 160 | 67 | **100** | pixel: ideal 0.67× vertical rounds to 1× → +50% stretch |
| 2x3/six/quad/half | 107 | 33–34 | 33–34 | `W·CellW < srcW` → no integer k ≥ 1, silently falls back to square-pixel `fitDimsRatio` (blends) |

## 2. Original decisions

1. **Distortion cap for `pixel`**: fall back to `aligned` when the integer row/col repeat deviates more than N% (e.g. 25%) from the aligned height?
2. **Downscale case**: when no integer upscale fits, should `aligned`/`pixel` use integer *downscale* (1/2, 1/3 nearest-neighbour), warn, or keep the current square-pixel fallback?

## 3. Resume point

Logic lives in the `ExplicitCols>0 && ExplicitRows==0` and mirrored `ExplicitRows` branches of `PlanRender`. Add cases to `TestPlanRender_Doom3x3` / `cmd/root_test.go` `TestCLIDoom1S2Render`.

## 4. Pixel decision (2026-10-03)

User reproduction: Doom at `-W 54 -m 3x3 --aspect pixel` emitted 67 rows.
The 6×3 cell grid fits the source once horizontally (320 pixels plus 4 padding),
so the continuous vertical extent is `320·3·200·2/(6·320·3) = 66.67` pixels.
Clamping vertical repeats to at least 1 forced 200 pixels: 3× the intended height.
Width 107 similarly forced 200 instead of 133.33 pixels; width 160 correctly uses 200.

Pixel now accepts whole-pixel repeats only within less than one mode cell of the
intended extent, rather than imposing a percentage policy. Otherwise it uses
nearest-neighbor scaling. The same rule limits blank padding on the constrained
axis. Too-small widths/heights use nearest-neighbor downsampling with the same
aspect formula, and pixel aliases override pyramid sampling to prevent blending.
Expected 3x3 rows: width 53 → 22, 54 → 23, 107 → 45, 160 → 67.
Quad at width 160 → 67 rows; six at width 160 remains 67 rows.

Existing goldens are unaffected: they use renderer fit geometry or explicit
native targets, not this single-dimension pixel path. Add a dedicated pixel
sampling golden with algorithm and sizing metadata.

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
