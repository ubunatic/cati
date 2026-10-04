# 077 — Cover 3x3 and all composite modes in geometry planner and fix line count/padding mismatches

**Status**: Closed — resolved by updating planner to fill explicit grid and aligning glyph grid bounds
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#071](071-respect-explicit-width-and-height-in-static-renders.md), [#072](072-add-aspect-modes-for-cli-source-mapping.md), [#075](075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md)

---

## 1. Problem & Motivation
When invoking CLI renders with specialized or composite modes (such as `3x3` and `six`), geometry derivation and line count validation can diverge:
1. `cati ~/projects/cati/assets/doom1.webm --play preview -W 107 -m 3x3` fails with:
   `Error: render line count mismatch for 3x3: got 33 rows, want 34`
2. `cati ~/projects/cati/assets/doom1.webm --play preview -W 160 -H 67 -m six` renders the image with 7 trailing empty rows instead of filling the full 67-row grid (due to `six` having 2×6 cell geometry vs `2x3` having 2×3 geometry).

## 2. Technical Specification / Findings
- **Mode Geometry Divergence**:
  - `3x3` has 3×3 cell geometry with 1:2 font aspect correction. Intermediate integer rounding between `fitDimsRatio` / `PlanRender` and `sparkline.RenderToImage` causes a 1-row mismatch (33 vs 34 rows).
  - Mode `six` uses 2×6 cell geometry ($67 \times 6 = 402\text{px}$), whereas the Doom 1 160×67 target was designed for 2×3 cell geometry (`2x3` / `s2`, $67 \times 3 = 201\text{px}$). In `six` mode, explicit canvas padding pads 7 blank rows to satisfy the requested 67-row box.
- **Goal**:
  - `/goal Ensure 3x3, 2x3, 2x6, and all spec render modes have aligned rasterization and validation bounds without line-count mismatches, and document mode-specific geometry distinctions, or stop and report when blocked on a user decision or denied permission.`

## 3. Implementation & Verification Plan
**Goal**: Harmonize row count derivation and renderer rasterization across all registered modes in `spec/modes.yaml` (including `3x3`, `2x3`, `six`, `quad`, `spark`), ensuring line-count validation passes without runtime errors.

1. **Row Count Alignment**:
   - Align `internal/viewgeom/planner.go` row rounding with `sparkline.RenderToImageWithOptions` and `validateRenderSize` across all aspect-corrected cell geometries (specifically $3\times3$, $2\times3$, $2\times6$, $4\times8$).
2. **Test Matrix**:
   - Add test cases in `cmd/root_test.go` for `3x3` preview with `-W 107` and verify 0 line-count mismatch errors.
   - Verify explicit grid behavior across both `2x3` (s2) and `2x6` (six) modes.
