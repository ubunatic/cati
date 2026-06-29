# 016 — Worker-Copy Render Paths: Keep Clones for Tuning, Consolidate Later

**Status:** 🔴 Open  
**Refs:** `internal/halfblock/render.go`, `internal/quadblock/render.go`, `internal/sparkline/render.go`, `cmd/interactive.go`, `cmd/metrics.go`, `cmd/ssim.go`

---

## Context

Worker-aware render entrypoints were added as cloned paths so we can tune them independently from the serial baselines:

- `internal/halfblock.RenderJ` / `RenderToImageJ`
- `internal/quadblock.RenderJ` / `RenderToImageJ`
- `internal/sparkline.RenderJ` / `RenderToImageJ`

The serial functions remain the authoritative baseline. The clones are called when `-j/--jobs > 1` so we can measure worker behavior without mutating the original loops or sub-functions underneath them.

## Why This Exists

The current clone strategy is intentional, not ideal:

- it keeps the original algorithms stable while parallel work is explored
- it makes benchmarking serial vs worker paths straightforward
- it avoids accidental behavior drift from shared helper refactors too early

The downside is duplication. Once the worker implementations settle, the shared pieces should be merged back into a single implementation per renderer.

## Cleanup Targets

1. Merge the cloned render loops back into shared helpers once worker behavior stabilizes.
2. Remove the `FIXME` markers from the worker copies by consolidating the duplicated path.
3. Re-bench serial vs worker paths after each consolidation step to make sure the tuned behavior does not regress.

## Notes

Do not refactor the original serial loops yet. The cloned versions are intentionally kept separate for tuning, comparison, and incremental parallelization.
