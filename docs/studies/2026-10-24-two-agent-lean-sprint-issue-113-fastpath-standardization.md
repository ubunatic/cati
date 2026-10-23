# Two-Agent Lean Sprint Study: Issue 113 Fastpath Standardization & In-Package Reference Testing

**Date**: 2026-10-24
**Sprint Target**: Issue 113 — Invert fastpath to standard and adopt in-package reference testing
**Host Orchestrator**: agy:gemini-4-argon
**Developer Agent**: codex:sol:med
**Architect Refinement**: Pre-refined by `argon:low` on 2026-10-24 (Option B selected)

---

## 1. Context & Objectives

Issue 113 addresses the inverted terminology and runtime branching of "fastpath" across `cati`.
Historically, `cati` had a "standard path" using generic `image.Image.At` and an optimized "fastpath" with direct slice indexing, ANSI builders, and bitmask solvers, toggled via `core.Fastpath` and `--fastpath` flags, and doubled in `Makefile` via `-tags=fastpath0`.

### Goals:
1. Make optimized direct-slice rendering the sole production standard path. Generic `img.At` remains only as the fallback branch for non-RGBA/NRGBA images.
2. Adopt in-package `reference_test.go` (Option B) containing naive nested-loop implementations for parity assertions on small fixtures.
3. Eliminate `core.Fastpath` runtime flags, CLI `--fastpath`, and `Makefile` double-runs (`-tags=fastpath0`).
4. Fix latent bug uncovered in `v1/sextant`: NRGBA partial alpha handling missing `core.NRGBAToRGBA`.

---

## 2. Searcher / Refinement Phase

- **Searcher**: Skipped per lean sprint protocol §1; Issue 113 was recently refined against live code with exact file:line citations across `v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`, `internal/metrics`, `cmd/`, and `Makefile`.
- **Architect Refinement**: Performed by `argon:low` prior to sprint start. Validated Option B (in-package `reference_test.go`) over Option A (monolithic `v1/reference`) to preserve encapsulation and avoid exposing unexported lookup tables or duplicating ~1,500 lines of internals.

---

## 3. Milestones & Progress Log

- **Milestone 1**: Standardize Core Render Packages & In-Package Reference Tests (`v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`, `internal/metrics`).
  - *Status*: Delivered in commit `61a85a8`.
  - *Verification*: `go test ./v1/... ./internal/metrics/...` and `make test-short` passed cleanly.
  - *Summary*: Removed `core.Fastpath` checks; direct slice access standardized as production standard; generic `img.At` fallback retained for non-RGBA/NRGBA images. Fixed latent NRGBA partial-alpha premultiplication in `v1/sextant` and `v1/sparkline/solver.go`. Replaced fastpath toggle tests with in-package `reference_test.go` parity suites. Updated rendering docs.

- **Milestone 2**: Strip CLI Knobs, `core.Fastpath`, and `Makefile` Double-Run.
  - *Status*: Dispatched to `sol:med`.
