# 102 — Deduplicate halfblock and sextant fastpath branches

**Status**: Closed — Fixed in cd68067; fast/reference suites, renderer race checks, preflight and benchmark comparison pass without golden changes
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: PRs #54, #55, [perf notes](../docs/perf/2026-10-12-halfblock-direct-sampling.md), [sextant perf notes](../docs/perf/2026-10-12-sextant-fast-solver.md)

---

## 1. Problem & Motivation
Review of PRs #54/#55 found copy-pasted fastpath branches. Each copy must stay
bit-identical to the reference path, so every duplicate is a future parity bug.

## 2. Technical Specification / Findings
1. `v1/halfblock/render.go` `RenderToGrid` row func: RGBA and NRGBA branches
   differ only in how one pixel is read; the `pairToCell` → `core.Cell` block is
   repeated three times (RGBA, NRGBA, generic).
2. `v1/halfblock/render.go` `RenderToImageJ` fastpath: three branches (RGBA,
   NRGBA, `safePixel`) repeat the same top/bot opaque-colour and `dst.Pix`
   write code (~130 lines). The fastpath also skips the grid and `pairToCell`
   entirely; it relies on the unstated invariant "grid reconstruction ==
   source pixel with A forced to 255, transparent → zero". Document it.
3. `v1/sextant/render.go` `RenderToImageJ`: the `core.Fastpath` branch copies
   the whole coverage loop only to replace `dst.SetRGBA` with direct `Pix`
   writes. Extract the loop; keep only the store in the branch.
4. `sextant.divTable` is used by the reference path too. It is exact for
   sums ≤ 1530, but per `core.Fastpath` doc the reference path should stay
   plain; either gate it or note why it is exempt.

## 3. Implementation & Verification Plan
**Goal**: Each fastpath has one pixel reader per image type and one shared
writer; behaviour is unchanged.

- Introduce a small per-row reader (e.g. `func(x int) color.RGBA`) chosen once
  per row per image type; share cell building and `dst.Pix` writes.
- No golden may change. `go test ./v1/...` (incl. `TestFastpathParity`,
  `TestFastpathDifferentialParity`), `go test -race ./v1/...`, and
  `make preflight` must pass.
- Benchmarks (`render_bench_test.go`) must not regress by more than noise;
  record before/after in the perf doc.

## Resolution

Extracted shared byte readers, grid-cell conversion, and opaque reconstruction
stores in halfblock while retaining concrete type selection per row. These
small helpers avoid introducing indirect reader calls in the hot loops.
Sextant now shares coverage/color traversal and branches only for the store;
its exact division-table arithmetic is documented and exhaustively tested.

Added odd-height subimage parity and independent source-color reconstruction
checks. The full fast/reference suites, `go test -race ./v1/...` (Go 1.26.0),
and `make preflight` pass; no goldens were regenerated. Pinned, interleaved
reconstruction benchmarks show no regression; measurements and reproduction
instructions are in the linked performance notes.
