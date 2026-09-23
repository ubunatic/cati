# 054 — Review of Jules "Bolt" Perf Patches (PR #8, #9)

**Status**: Closed (post-factum: worker cap moved to Makefile env, all fast paths behind shared CATI_FASTPATH)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Performance
**Related**: `v1/sextant/render.go`, `v1/quadblock/render.go`, `v1/core/workers.go`, `Makefile`, commits `e43ad27`, `5466506`

---

## 1. Review findings

- **PR #8 (`e43ad27`, sextant)**: static `allMasks()` slice (safe; only reader is
  `chooseBestCell`), 6-entry LUT for `bitForIndex`/`maskContains` (unguarded
  index in `maskContains`; callers stay in 0–5). Hardcoded 10-worker cap with
  no rationale. No tests or benchmarks.
- **PR #9 (`5466506`, quadblock)**: direct `*image.RGBA` pixel sampling and
  `strconv.AppendUint` ANSI formatting, with parity test. `fastpathEnabled()`
  called `os.Getenv` per pixel and per colour escape, undercutting the
  optimisation. Commit message claims a 10-core cap in `computeQuadCellsJ`
  that was not actually present.
- Both perf docs are dated 2026-03-30, commits 2026-09-23; benchmark numbers
  unverified.

## 2. Changes

- New `core.MaxWorkers()` (`v1/core/workers.go`): reads `CATI_MAX_WORKERS`
  once; unset/invalid/<1 means `runtime.NumCPU()`, otherwise `min(NumCPU, v)`.
- `sextant.RenderToGrid`, `sextant.RenderToImageJ`, and quadblock's worker
  pool use `core.MaxWorkers()` instead of `min(NumCPU, 10)` / `NumCPU`.
- `Makefile` exports `CATI_MAX_WORKERS ?= 10`, so the cap applies to make
  targets and is overridable; plain binaries default to uncapped.
- All fast/simple splits share one flag, `core.Fastpath`
  (`v1/core/fastpath.go`), read once from `CATI_FASTPATH` (`0` = simple).
  Replaces the per-pixel `os.Getenv("QUADBLOCK_FASTPATH")`. Makefile exports
  `CATI_FASTPATH ?= 1`.
- Principle: the simple path is the reference; fast paths only change *how*
  pixels are read or bytes appended and must follow the simple path.
- Gated splits and simplifications:
  - `quadblock.safePixel`: RGBA direct read, now via `PixOffset`.
  - `quadblock.Render`: the duplicated output loop collapsed into one loop;
    only the colour-escape step switches (`appendFgRGB` vs `fgRGB`).
    `fgRGB`/`bgRGB` are plain `Sprintf` again.
  - `sparkline.rgbaAt`: RGBA direct read.
  - `sparkline` solver block sampling: fast/simple now only fill pixel
    arrays; the duplicated accumulation body is one shared loop.
  - `metrics.extractLumaFlat`: typed fast paths gated; `At()` path extracted
    as `extractLumaSimple`. Drift tracked in issue 055.
- Parity tests: `v1/quadblock/render_fastpath_test.go`,
  `v1/sparkline/fastpath_test.go`, `internal/metrics/fastpath_test.go`.
- `docs/Testing.md` gains a "Fast paths" section.

## 3. Verification

`make test` green; full suite also green with `CATI_FASTPATH=0` (goldens unchanged on both paths); `make install` done.
