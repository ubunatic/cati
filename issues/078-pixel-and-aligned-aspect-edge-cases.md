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

## 2. Open decisions

1. **Distortion cap for `pixel`**: fall back to `aligned` when the integer row/col repeat deviates more than N% (e.g. 25%) from the aligned height?
2. **Downscale case**: when no integer upscale fits, should `aligned`/`pixel` use integer *downscale* (1/2, 1/3 nearest-neighbour), warn, or keep the current square-pixel fallback?

## 3. Resume point

Logic lives in the `ExplicitCols>0 && ExplicitRows==0` and mirrored `ExplicitRows` branches of `PlanRender`. Add cases to `TestPlanRender_Doom3x3` / `cmd/root_test.go` `TestCLIDoom1S2Render`.
