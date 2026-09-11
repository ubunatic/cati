# 046 — Modes Command Dataset Benchmark Scorecard

**Status**: Closed (Implemented in `cmd/modes_benchmark.go` and `cmd/modes.go`)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [`issues/044-modes-command-custom-image-input-support.md`](file:///home/uwe/projects/cati/issues/044-modes-command-custom-image-input-support.md), [`issues/045-modes-command-test-asset-presets-and-sample-shortcuts.md`](file:///home/uwe/projects/cati/issues/045-modes-command-test-asset-presets-and-sample-shortcuts.md), [`cmd/modes.go`](file:///home/uwe/projects/cati/cmd/modes.go)

---

## 1. Problem & Motivation
Evaluating algorithm quality and speed across multiple render modes currently requires running commands one-by-one or checking test outputs. A benchmark scorecard mode in `cati modes` can evaluate all modes across the full test corpus and summarize performance metrics in a structured table.

## 2. Technical Specification
- Add `--benchmark` / `--suite` flag to `cati modes`.
- Evaluate all active modes across synthetic geometric patterns and photographic samples.
- Compute average latency, category SSIM scores (geometric vs photographic), and overall efficiency index.
- Display a clear ANSI summary table with rank and metrics.

## 3. Implementation Plan
1. Define test corpus groupings in `cmd/modes.go` or `cmd/benchmark.go`.
2. Implement aggregated multi-image evaluation loop utilizing the Phase 1 (isolated timing) and Phase 2 (parallel SSIM) architecture.
3. Add formatted ASCII/ANSI summary scorecard renderer.
4. Add unit and golden tests for scorecard generation.
