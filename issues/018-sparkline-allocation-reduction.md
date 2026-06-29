# 018 — Reduce Allocation Pressure in Spark Mode

**Status:** 🔴 Open  
**Refs:** `internal/sparkline/render.go`, `internal/sparkline/render_bench_test.go`, `internal/sparkline/render_jobs_test.go`, `cmd/interactive.go`, `cmd/metrics.go`, `cmd/ssim.go`

---

## Context

Spark mode is the heaviest renderer in the current bench set and it allocates aggressively on the hot path. The current worker-copy path is useful for tuning, but it still leaves a lot of allocation pressure in place:

- per-cell glyph selection and reconstruction allocate more than the block renderers
- `RenderToImage` and its worker copy are expensive enough that throughput is dominated by allocator churn
- the current parallel work should not hide the underlying allocation profile

## Goal

Reduce allocations in spark mode without changing output semantics.

The first pass should focus on the hottest paths only:

1. identify the dominant allocation sites in `internal/sparkline`
2. remove avoidable per-cell and per-row heap churn
3. keep the serial renderer as the reference while tuning the worker copy
4. re-bench after each change so allocation wins are measured, not assumed

## Scope for the First Pass

- reuse scratch buffers where it is safe to do so
- avoid temporary slices or strings in the glyph mapping path
- prefer stack-local or pooled working state for row/block iteration
- preserve the current output bit-for-bit unless a later issue explicitly changes rendering behavior

## Notes

Do not collapse the worker copy back into the serial path yet. This issue is about reducing allocation cost first, then consolidating once the hot path is stable.

## Current Baseline

The first allocation-reduction pass landed in `internal/sparkline/render.go` and the direct cell benchmark now shows the hot path is allocation-free:

- `BenchmarkFindBestCellVertical`: `0 allocs/op` from `524,288`
- `BenchmarkFindBestCellQuad`: `0 allocs/op` from `1,572,864`

The current local bench run for the package is:

- `BenchmarkFindBestCellVertical`: `2.97 ms/op`, `0 B/op`, `0 allocs/op`
- `BenchmarkFindBestCellQuad`: `11.68 ms/op`, `0 B/op`, `0 allocs/op`
- `BenchmarkRenderSerial`: `3.24 ms/op`, `227,647 B/op`, `2,240 allocs/op`
- `BenchmarkRenderToImageSerial`: `3.27 ms/op`, `131,136 B/op`, `2 allocs/op`

Remaining allocations now live higher up in the render path, mostly around row assembly and full-image reconstruction. That makes the next tuning steps more targeted: keep the cell kernel lean, then squeeze the ANSI string building and `RenderToImage` paths separately.
