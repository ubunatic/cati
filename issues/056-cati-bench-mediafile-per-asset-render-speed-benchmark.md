# 056 — `cati --bench <mediafile>`: Per-Asset Render Speed Benchmark

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `cmd/modes_benchmark.go` (synthetic-corpus scorecard), `cmd/play.go`, `v1/halfblock/video.go`, `CATI_FASTPATH` / `CATI_MAX_WORKERS` (issue 054)

---

## /goal

`cati --bench <mediafile>` measures and prints processing speed for a real
asset so render modes (and `CATI_FASTPATH=0/1`, `CATI_MAX_WORKERS`) can be
compared on user media:

- **Image**: render the image (repeatedly, for stable timing) per mode and
  report time per render.
- **Video**: decode and render **every frame** of the whole video as fast as
  possible — no playback pacing, no terminal output, **video only, no audio** —
  and report total time, frames, and fps per mode.

Done when the flag is specced (`spec/` + schema + test, per docs/Spec.md),
works for image and video, and its output makes a fast-vs-simple or
mode-vs-mode comparison readable at a glance.

## Notes / open questions

- `cmd/modes_benchmark.go` already benchmarks modes on a fixed built-in
  corpus with SSIM; decide whether to reuse its table/entries or keep
  `--bench` purely about speed.
- Where time goes (decode vs render) would be useful to report separately.
- Re-check current CLI/spec layout before starting; this ticket may be stale.
