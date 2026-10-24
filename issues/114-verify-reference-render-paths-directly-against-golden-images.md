# 114 — Verify reference render paths directly against golden images

**Status**: Closed — Added in-package TestReferenceGoldenParity across v1/{halfblock,quadblock,sextant,sparkline}; replaced sextant solver delegation with naive scoreMask loop; verified directly against golden PNGs with sub-second execution; make test-q1 green
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

### Milestone 1: In-Package Reference Golden Parity in `v1/halfblock` and `v1/quadblock` (Delivered: commit 935934d)
- **Delivered Summary**:
  - Implemented private `renderToImageReference` in `v1/halfblock/reference_test.go` and `v1/quadblock/reference_test.go`.
  - Added parallel `TestReferenceGoldenParity` in both packages testing 8 representative golden cases (`demo_horiz_20x20`, `demo_verti_20x20`, `demo_horiz_4x4` at n=2, 4, 5).
  - Used exact golden canvas replication (`12×12` for halfblock, `6×12` for quadblock) and asserted pixel-for-pixel equality against golden PNGs.
  - Test runtimes remain sub-second (`halfblock` ~0.5s, `quadblock` ~0.03s). Verified by Host Orchestrator diff review.

### Milestone 2: Naive Solver & Golden Verification in `v1/sextant` and `v1/sparkline` (Delivered: commit d5f9fcb)
- **Delivered Summary**:
  - In `v1/sextant/reference_test.go`, replaced production solver delegation with `chooseCellReference` evaluating all 64 masks using `scoreMask` and tie breaks. Added private `renderToImageReference` and parallel `TestReferenceGoldenParity` testing against golden sextant renders replicated `6×8`.
  - In `v1/sparkline/reference_test.go`, added private `renderToImageReference` and parallel `TestReferenceGoldenParity` testing both Quad (`render_spark_<n>ch.png`, replicated `3×3`) and Best (`render_spark_best_<n>ch.png`, replicated `3×1`), covering both $\le 64$ and $65-128$ pixel kernels.
  - Package runtimes remain sub-second (`sextant` ~0.02s, `sparkline` ~0.35s). Verified by Host Orchestrator diff review.

### Milestone 3: Documentation & Quota-1 Verification
- Update `docs/Testing.md` to document in-package reference golden testing.
- Run `HTO=0 make test-q1` and `make preflight`.
- Verify package runtimes remain sub-second.
- Close Issue 114 with decisive evidence.
