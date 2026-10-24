# Two-Agent Lean Sprint Study: Issue 114 In-Package Reference Golden Parity Testing

**Date**: 2026-10-24
**Sprint Target**: Issue 114 — Verify reference render paths directly against golden images
**Host Orchestrator**: agy:gemini-4-argon
**Developer Agent**: codex:sol:med
**Reviewer Agent**: agy:argon:low
**Architect Refinement**: Pre-refined by `argon:low` on 2026-10-24

---

## 1. Context & Objectives

Following Issue 113's standardization of production renderers to direct slice access and bitmask solvers, naive reference implementations were introduced in `v1/*/reference_test.go`. While transitively verified via parity against standard paths and standard paths against golden PNGs, Issue 114 establishes direct in-package golden image assertions for the reference implementations.

### Goals:
1. Maintain encapsulation: do not export reference implementations across package boundaries.
2. Direct golden verification in `v1/halfblock` and `v1/quadblock` (M1).
3. Naive `scoreMask` solver and golden verification in `v1/sextant` and `v1/sparkline` (M2).
4. Sub-second test execution across all packages by bounding to representative golden cases.
5. Review each milestone commit with `argon:low` reviewer.

---

## 2. Searcher / Refinement Phase

- **Searcher**: Skipped per lean sprint protocol §1; Issue 114 was recently refined against live code with exact file:line citations across `v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`, and `testdata/`.
- **Architect Refinement**: Pre-refined by `argon:low`. Established in-package `reference_test.go` golden tests (relative path `../../testdata`), identified that `v1/sextant` reference test called production `chooseCell` instead of naive solver, and established bounded representative golden cases (`n = 2, 4, 5`).

---

## 3. Milestones & Progress Log

- **Milestone 1**: In-Package Reference Golden Parity in `v1/halfblock` and `v1/quadblock`.
  - *Status*: Delivered in commit `935934d`.
  - *Verification*: `go test ./v1/halfblock/... ./v1/quadblock/...` and `make test-short` passed cleanly.
  - *Summary*: Implemented private `renderToImageReference` in `v1/halfblock/reference_test.go` and `v1/quadblock/reference_test.go`. Added parallel `TestReferenceGoldenParity` in both packages testing 8 representative golden cases (`demo_horiz_20x20`, `demo_verti_20x20`, `demo_horiz_4x4` at n=2, 4, 5). Used exact golden canvas replication (`12×12` for halfblock, `6×12` for quadblock) and asserted pixel-for-pixel equality against golden PNGs. Package runtimes remained sub-second (`halfblock` ~0.5s, `quadblock` ~0.03s). Reviewed by Host Orchestrator directly due to `argon:low` quota exhaustion.

- **Milestone 2**: Naive Solver & Golden Verification in `v1/sextant` and `v1/sparkline`.
  - *Status*: Delivered in commit `d5f9fcb`.
  - *Verification*: `go test ./v1/sextant/... ./v1/sparkline/...` and `make test-short` passed cleanly.
  - *Summary*: Replaced sextant production solver delegation with `chooseCellReference` evaluating all 64 masks using `scoreMask` and tie breaks. Added private `renderToImageReference` and parallel `TestReferenceGoldenParity` in both packages. Sparkline verifies both Quad (`render_spark`) and Best (`render_spark_best`) modes against goldens. Package runtimes remain sub-second (`sextant` ~0.02s, `sparkline` ~0.35s). Reviewed by Host Orchestrator diff review.

- **Milestone 3**: Documentation & Final Quota-1 Verification.
  - *Status*: In progress.
