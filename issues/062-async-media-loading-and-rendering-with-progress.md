# 062 — Async Media Loading and Rendering with Progress

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [Go Library API](../docs/GoLibrary.md), [Video & Audio Pipeline](../docs/Video.md)

---

## 1. Problem & Motivation

Loading preview videos can take 1–2 seconds, and high-resolution images or videos processed by expensive rendering algorithms can take much longer. Callers currently need a way to keep their UI responsive and show users what is happening during this work.

## 2. Technical Specification / Findings

Add an optional API/protocol for asynchronous media loading and rendering that reports progress. It should cover both loading media and rendering it, while preserving the existing synchronous usage for callers that do not need progress.

## 3. Implementation Milestones

### M1 (Progress Types & Async Image/Media Loader API)
- Define typed progress events in `v1/core`: `Progress{ Stage string, Ratio float64, Current int, Total int, Message string }`.
- Add progress-aware image/media loading in `v1/halfblock` (and `v1/core`): `LoadImageContext(ctx context.Context, path string, onProgress func(core.Progress)) (image.Image, error)` and `LoadImageAsync(...)`.
- Add unit tests verifying progress callbacks, cancellation via `context.Context`, and zero-overhead synchronous paths.

### M2 (Progress-Aware Rendering Pipeline)
- [x] Add optional `OnProgress func(core.Progress)` to renderer options across algorithms (`halfblock.Options`, `quadblock.Options`, `sextant.Options`, `sparkline.Options`).
- [x] Emit atomic progress updates during row/cell processing in serial and parallel grid-render loops only when configured; clamp ratios to `[0,1]`.
- [x] Add unit tests covering event stage, totals/current, ratio bounds, and safe concurrent callback use across renderers.

### M3 (Docs, Verification & Preflight)
- [x] Update `docs/GoLibrary.md` and `docs/Video.md` with async loading and rendering progress usage patterns.
- [x] Run full test suite (`go test ./...`), `go vet ./...`, `make preflight`, and `make install` (all pass).
