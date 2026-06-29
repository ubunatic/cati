# 017 — GPU Assistance for Glyph Mapping, Starting with Spark Mode

**Status:** 🔴 Open  
**Refs:** `internal/sparkline`, `cmd/interactive.go`, `cmd/metrics.go`, `cmd/ssim.go`, `internal/quadblock`, `internal/halfblock`

---

## Context

The current glyph-mapping path is CPU-only and does per-cell analysis in Go. That is fine for the baseline renderer, but spark mode is the most compute-heavy path and the best candidate if we want to explore GPU assistance.

Spark mode is a good first target because it already operates on fixed-size analysis blocks and has a clearly bounded reconstruction step:

- source pixels are grouped into `4×8` blocks
- each block is mapped to a glyph plus fg/bg colours
- the same mapping feeds ANSI output, `RenderToImage`, and SSIM/quality scoring

## Goal

Explore a GPU-assisted path for glyph selection and reconstruction in spark mode first, with the option to extend to other renderers later.

The intended use is acceleration of the glyph mapping work itself, not a rewrite of the terminal I/O or viewer UI.

## Scope for the First Pass

1. Identify the minimum data that must be uploaded to the GPU for a spark block analysis pass.
2. Measure whether a GPU pass can beat the existing CPU implementation once upload/download overhead is included.
3. Keep the serial CPU path as the reference implementation.
4. Preserve output equivalence first; performance tuning comes after correctness.

## Open Questions

- Which API is the best fit on this codebase: OpenCL, CUDA, Vulkan compute, or a Go binding around an existing GPU compute layer?
- Is the useful unit of work a single cell, a row of cells, or a whole viewport tile?
- Does the GPU path help enough on typical terminal-sized images to justify the extra dependency and complexity?
- Should the GPU path produce a candidate glyph map only, or also reconstruct the rendered image for quality metrics?

## Notes

Do not start by parallelizing every renderer. Spark mode should be the first benchmarked target because it has the largest per-cell analysis cost and the cleanest block geometry for experimentation.

## First Pass

1. Benchmark `FindBestCell` directly for `spark/quad` on a representative `4×8` block grid.
2. Keep the current Go implementation as the reference path and measure any candidate backend against it.
3. Treat upload/download and reconstruction costs as part of the baseline, not as an optional later detail.
4. If a GPU path is added later, start as a candidate glyph mapper for spark mode only and leave the terminal/CLI code untouched.

## Baseline Numbers

The first direct kernel benchmark set is in `internal/sparkline/find_best_cell_bench_test.go`.
The current CPU results show the shape of the work:

- `BenchmarkFindBestCellVertical`: `2.97 ms/op`, `0 B/op`, `0 allocs/op`
- `BenchmarkFindBestCellQuad`: `11.68 ms/op`, `0 B/op`, `0 allocs/op`

The quad path is materially more expensive than the vertical baseline, so any future GPU path needs to batch many cells at once. Per-cell dispatch would likely lose to overhead before it reaches the renderer core.
