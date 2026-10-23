# 114 — Verify reference render paths directly against golden images

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Testing
**Related**: 113 (fastpath inversion and in-package reference tests), docs/Testing.md, docs/RenderingBugPlaybook.md

---

/goal Verify that naive reference render paths directly match golden PNG images, either via in-package golden verification or shared test fixtures, ensuring baseline naive implementations produce identical results on real golden assets; or stop and report when blocked on a user decision.

## 1. Problem & Motivation

Following Issue 113, `cati` standardized all production renderers (`v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`) to direct slice indexing and precomputed bitmask solvers. Naive reference implementations (`renderReference`) were placed in `v1/*/reference_test.go`.

Currently:
1. `cmd/golden_render_test.go` verifies the **standard production paths** directly against golden PNGs in `cmd/testdata/`.
2. Each package's `reference_test.go` verifies the **reference path** against the standard path on small synthetic fixtures (8×8 to 32×32).
3. The reference paths themselves are never run directly against the golden PNG assets.

While parity holds transitively ($\text{Reference} == \text{Standard} == \text{Goldens}$), running the reference implementations directly against golden PNG fixtures provides an unyielding, direct verification that our naive algorithms (simple nested `for`-loops without bitwise/slice optimizations) independently generate the exact expected golden renders.

## 2. Technical Considerations & Discovery Questions

1. **Test Location & Package Encapsulation**:
   - `renderReference` functions in `v1/*/reference_test.go` are currently unexported to avoid public API leakage (Option B in Issue 113).
   - *Option A (In-Package Golden Checks)*: Add a golden verification test directly within each package's `reference_test.go` that reads the relevant subset of golden fixtures (e.g. `cmd/testdata/demo_horiz_20x20/` etc.).
   - *Option B (Internal Test Helper)*: Export an internal/testhelper entrypoint (e.g. `export_test.go` or `internal/referencetest`) allowing `cmd/golden_render_test.go` to optionally run golden sweeps in reference mode.
2. **Performance & Test Bounding**:
   - Reference implementations are naive and unoptimized (`img.At` calls, slow nested searches).
   - Running every golden resolution (1ch to 20ch across multiple aspect ratios) through naive loops might introduce unwanted test latency.
   - Which representative golden subset should be tested against reference implementations to keep execution sub-second while maintaining high confidence?

## 3. Implementation Plan Skeleton

- [ ] Investigate fixture sharing: access `cmd/testdata` or representative golden fixtures from `v1/*` packages.
- [ ] Determine whether to test reference loops in-package (`v1/*/reference_test.go`) or via `cmd/golden_render_test.go`.
- [ ] Implement direct golden comparison assertions for `halfblock`, `quadblock`, `sextant`, and `sparkline` reference loops.
- [ ] Verify execution time remains within sub-second bounds.
