# 114 — Verify reference render paths directly against golden images

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Testing
**Related**: 113 (fastpath inversion and in-package reference tests), docs/Testing.md, docs/RenderingBugPlaybook.md

---

/goal Verify that naive reference render paths directly match golden PNG images in testdata/, establishing in-package reference golden tests across v1/{halfblock,quadblock,sextant,sparkline} on representative cases while maintaining sub-second package test execution; or stop and report when blocked on a user decision.

## 1. Problem & Motivation

Following Issue 113, `cati` standardized all production renderers (`v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`) to direct slice indexing and precomputed bitmask solvers. Naive reference implementations (`renderReference`) were placed in `v1/*/reference_test.go`.

Currently:
1. `cmd/golden_render_test.go` verifies the **standard production paths** directly against golden PNGs in `testdata/`.
2. Each package's `reference_test.go` verifies the **reference path** against the standard path on small synthetic fixtures (8×8 to 32×32).
3. The reference paths themselves are never run directly against the golden PNG assets.

While parity holds transitively ($\text{Reference} == \text{Standard} == \text{Goldens}$), running the reference implementations directly against golden PNG fixtures provides direct, unyielding verification that naive algorithms (simple nested `for`-loops without bitwise/slice optimizations) independently generate the exact expected golden renders.

## 2. Architectural Analysis & Findings (Refined by argon:low, 2026-10-24)

### Architectural Decisions:
1. **In-Package Golden Checks (`v1/*/reference_test.go`) over `cmd/` Integration**:
   - `v1/*` are public library packages. Exporting `renderReference` or internal cell types (`cellResult`, `quadCell`, `candidate`) for `cmd/` violates the core encapsulation invariant established in Issue 113.
   - Centralizing in `cmd/` would introduce cross-package test dependencies. In-package `reference_test.go` keeps each package completely self-contained and independently testable (`go test ./v1/quadblock/...`).
   - Working directory for `v1/<pkg>` tests is `v1/<pkg>`, resolving repo golden assets at `../../testdata/`.
2. **Latent Test Seam Discovered in `v1/sextant`**:
   - In `v1/sextant/reference_test.go`, `renderReference` currently calls production `chooseCell(pixels, mode)`, which delegates to `chooseBestCell` (the optimized algebraic solver) instead of a naive solver!
   - In Milestone 2, `renderReference` must be updated to call a true naive `chooseCellReference(pixels)` that evaluates all 64 masks using `scoreMask(pixels, mask)` and tie-breaking, ensuring the sextant solver math is independently verified against goldens.
3. **Canvas & Performance Bounding**:
   - Golden PNGs in `testdata/<folder>/render_<algo>_<n>ch.png` are stored on a `12×24` char-resolution canvas using integer replication (`kX, kY`).
   - Naive loops (`img.At`, 64-mask scoring) are slower than production bitmasks. Testing a representative subset of golden resolutions (`n = 2, 4, 5` across `demo_horiz_20x20`, `demo_verti_20x20`, `demo_horiz_4x4`) provides 100% code-path coverage (including edge cases and aspect snapping) while keeping total execution under 200ms.

## 3. Implementation Roadmap

### Milestone 1: In-Package Reference Golden Parity in `v1/halfblock` and `v1/quadblock`
- **`v1/halfblock/reference_test.go`**:
  - Add `renderToImageReference(img image.Image) *image.RGBA` that reconstructs pixels from `renderReference` grid cells.
  - Add `TestReferenceGoldenParity(t *testing.T)` with `t.Parallel()` testing representative golden cases (`../../testdata/<folder>/render_halfblock_<n>ch.png`).
  - Scale source with `imgutil.FitDims`, upscale `12×12` to canvas, and assert pixel equality against decoded golden PNG.
- **`v1/quadblock/reference_test.go`**:
  - Add `renderToImageReference(img image.Image, opts Options) *image.RGBA` that reconstructs `2×2` pixels from `core.Cell` (`'▌'`, `'▐'`, and `charToMask(c.Ch)`).
  - Add `TestReferenceGoldenParity(t *testing.T)` with `t.Parallel()` testing `Options{SplitHalf: true, HalfblockThreshold: 2}` against `../../testdata/<folder>/render_quad_<n>ch.png` (upscaled `6×12`).

### Milestone 2: Naive Solver & Golden Verification in `v1/sextant` and `v1/sparkline`
- **`v1/sextant/reference_test.go`**:
  - Replace `chooseCell` with naive `chooseCellReference` evaluating all 64 masks via `scoreMask` and tie-breaks.
  - Add `renderToImageReference(img image.Image, mode Mode) *image.RGBA` reconstructing `2×3` blocks.
  - Add `TestReferenceGoldenParity(t *testing.T)` testing against `../../testdata/<folder>/render_sextant_<n>ch.png` (upscaled `6×8`).
- **`v1/sparkline/reference_test.go`**:
  - Add `renderToImageReference(img image.Image, outCols, outRows int, mode Mode) *image.RGBA` calling `searchReference` for each cell.
  - Add `TestReferenceGoldenParity(t *testing.T)` asserting against both `render_spark_<n>ch.png` (Quad, cell `4×8`) and `render_spark_best_<n>ch.png` (Best, cell `4×24`), exercising both $\le 64$ and $65-128$ pixel kernels.

### Milestone 3: Documentation & Quota-1 Verification
- Update `docs/Testing.md` to document in-package reference golden testing.
- Run `HTO=0 make test-q1` and `make preflight`.
- Verify package runtimes remain sub-second.
