# Two-Agent Lean Sprint Story: Inverting Fastpath to Standard and In-Package Reference Testing

**Date**: 2026-10-24
**Scope**: Invert fastpath architecture across `cati`, standardize direct-slice renderers, introduce in-package reference parity tests, remove CLI/core runtime switches, and eliminate Makefile double-run.
**Sprint Target**: Issue 113 — Invert fastpath to standard and adopt in-package reference testing
**Host Orchestrator**: agy:gemini-4-argon
**Developer Agent**: codex:sol:med
**Architect Refinement**: Pre-refined by `argon:low` (Option B selected)

---

## 1. Header & Context

Historically, `cati` had a dual-path rendering architecture: a "standard path" using generic `image.Image.At(x, y)` sampling and an optimized "fastpath" with direct slice indexing (`*image.RGBA`, `*image.NRGBA`), ANSI byte builders, and integer bitmask solvers.
This split was toggled via `core.Fastpath` boolean switches, exposed to CLI users via `--fastpath` and `CATI_FASTPATH`, and exercised in CI/testing by running the entire test suite twice in `Makefile` via `-tags=fastpath0`.

### Inversion Rationale
The terminology was inverted: the optimized path is what users expect in production, while generic `img.At` is a fallback for less common image types (like `image.Gray` or `image.Paletted`). Furthermore, running every test twice placed an unnecessary CPU and latency burden on development and CI.

---

## 2. Executive Summary

In this lean sprint, `cati` successfully completed the fastpath inversion:
1. **Production Standard**: All renderers (`v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`) and `internal/metrics` now treat direct slice access and bitmask solvers as the standard implementation, retaining generic `img.At` solely as the `default:` type-switch fallback.
2. **In-Package Reference Testing (Option B)**: Naive nested-loop reference implementations were introduced in `reference_test.go` within each package. This avoided exporting ~1,500 lines of internal lookup tables while asserting bit-for-bit or visual parity on synthetic test fixtures.
3. **Latent Bug Fixes**: Corrected non-premultiplied NRGBA alpha handling in `v1/sextant` and `v1/sparkline/solver.go`.
4. **Stripped Knobs & Double Runs**: Deleted `v1/core/fastpath.go`, removed `--fastpath` and `CATI_FASTPATH` from the CLI, updated `spec/cli.yaml`, simplified media benchmarks to a single pass, and eliminated `Makefile` `-tags=fastpath0` targets.
5. **Verification**: `HTO=0 make test-q1` and `make preflight` verified 100% green.

---

## 3. What Worked Well

- **Two-Agent Lean Division of Labor**:
  - `argon:low` architectural refinement proved decisive: choosing Option B (in-package `reference_test.go`) avoided polluting the public Go library API with naive algorithms and prevented massive table duplication.
  - `codex:sol:med` as developer was fast and rigorous, completing both Milestone 1 and Milestone 2 in single shots with zero host code interventions.
- **Strict Invariants**:
  - The Host Orchestrator maintained zero coding and diff-first review, managing the ticket and running tests cleanly.
  - Quota-1 test single-run discipline was respected throughout each turn.
- **Immediate Test Runtime Benefit**:
  - Removing `-tags=fastpath0` eliminated 50% of the redundant test runs across the whole repository without dropping test coverage.

---

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

- **Harnez 60s Ambient Timeout on Preflight**:
  - *Symptom*: Running `make preflight` directly was killed after 60 seconds with `exit status 137` by the default `harnez exec` timeout during the demo-widths rendering verification.
  - *Fix*: Re-ran with `HTO=0 make preflight`, which completed successfully in ~75 seconds.
  - *Learning*: Always prefix long verification gates (`make preflight`, full test suites) with `HTO=0`.
- **Latent NRGBA Premultiplication Bug**:
  - *Symptom*: `v1/sextant` and `v1/sparkline` were copying raw `nrgba.Pix` bytes without premultiplying alpha via `core.NRGBAToRGBA`, unlike `v1/quadblock` (fixed in issue 094).
  - *Fix*: Spotted during ticket refinement and verified by `sol:med` during M1 planning, properly applying `core.NRGBAToRGBA` before pixel/block scoring.
- **Stale `--fastpath` Guidance in `AGENTS.md` and Benchmark Test**:
  - *Symptom*: Removing `--fastpath` caused `cmd/media_benchmark_test.go` to look for lowercase `avg/render` (now `Avg/render`) and surfaced an outdated `--fastpath` exception rule in `AGENTS.md`.
  - *Fix*: `sol:med` caught both during pre-commit diff review and cleaned them up in the M2 commit.

---

## 5. Quality & Invariants Audit

| Dimension | Assessment | Notes |
| :--- | :---: | :--- |
| **Architecture & Separation** | PASS | Naive reference algorithms live strictly in `*_test.go`, keeping production packages lean and avoiding public API leaks. |
| **Idempotency** | PASS | `make preflight` and `make test-short` produce zero git diffs. Re-running tests is deterministic. |
| **Backward Compatibility** | PASS | Public API signatures in `v1/...` remain unchanged; only internal runtime switches and CLI flags were removed. |
| **Spec Integrity** | PASS | `spec/cli.yaml` updated; `cmd/cli_defaults_test.go` passes with zero hardcoded default violations. |
| **Test Verification** | PASS | `HTO=0 make test-q1` passed green in 55s. `HTO=0 make preflight` passed with zero errors. |

---

## 6. Efficiency & Velocity Assessment

- **Milestones Delivered**: 3 (M1 Render Packages, M2 CLI/Makefile, M3 Full Verification & Docs).
- **Turns to Complete**: 2 developer turns (M1 and M2) + host orchestrator verification.
- **Test Time Reduction**: Elimination of `-tags=fastpath0` halved repository test cycles. Package-level short tests now execute in ~11s.
- **Token Efficiency**: Cached context reuse was high across `sol:med` turns, with exact line-bounded reads preventing token bloat.

---

## 7. Key Learnings & Evergreen Upstream

1. **Option B for Reference Implementations**:
   - When optimizing algorithms that rely on large internal lookup tables, prefer in-package `reference_test.go` over dedicated `reference` packages. It prevents unnecessary symbol exportation and avoids duplicating internal tables.
2. **Never Invert Terminology in Production**:
   - Optimized paths should always be the standard production code path; generic fallbacks should be structured as explicit fallback branches, not "non-fastpaths".
3. **Always Set `HTO=0` on `make preflight`**:
   - `make preflight` performs builds, installations, vet passes, and demo width sweeps, which routinely exceed 60 seconds on standard hardware.

---

## 8. File & Diff Summary

### Commits:
- `61a85a8`: `refactor(render): standardize production paths and add in-package reference tests (issue 113 M1)`
- `58e0f1c`: `refactor(cli): strip fastpath flag, core.Fastpath, and Makefile double-run (issue 113 M2)`
- `3220c73`: `docs(issues): close 113, Standardized production paths across core render packages...`
- `ba48324`: `docs(testing): update suite description for single-pass reference testing`

### Key Files Changed:
- `v1/halfblock/`: Standardized `render.go`, added `reference_test.go`.
- `v1/quadblock/`: Standardized `render.go`, converted to `reference_test.go`.
- `v1/sextant/`: Fixed NRGBA premultiplication, standardized `render.go`, converted to `reference_test.go`.
- `v1/sparkline/`: Fixed NRGBA premultiplication in `solver.go`, standardized `render.go`, added `reference_test.go`.
- `internal/metrics/`: Standardized `metrics.go`, converted to `reference_test.go`.
- `cmd/`: Removed `--fastpath` from `root.go`, simplified `media_benchmark.go`, deleted `fastpath_flags_test.go`.
- `v1/core/`: Deleted `fastpath.go`.
- `Makefile`: Removed `-tags=fastpath0` and `CATI_FASTPATH=0` targets.
- `spec/cli.yaml`: Removed `--fastpath` entry.
- `docs/`: Updated `QuadPixelArt.md`, `RenderPipelines.md`, `SparklinePixelArt.md`, `Testing.md`, `Design.md`, `Video.md`.
