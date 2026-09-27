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
- Add optional `Progress func(core.Progress)` / `OnProgress` to renderer options across algorithms (`halfblock.Options`, `quadblock.Options`, `sextant.Options`, `sparkline.Options`).
- Emit progress updates during chunk/row processing in parallel render loops when the callback is set.
- Add unit tests covering progress reporting and bounds clamping (`0.0 <= Ratio <= 1.0`).

### M3 (Docs, Verification & Preflight)
- Update `docs/GoLibrary.md` and `docs/Video.md` with async and progress usage patterns.
- Run full test suite (`go test ./...`), `make preflight`, and `make install`.
