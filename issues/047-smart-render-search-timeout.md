# 047 — Abort --smart Search After 1s (--smart-timeout Default)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [`cmd/smart_render.go`](../cmd/smart_render.go), [`spec/render_modes.yaml`](../spec/render_modes.yaml), [`docs/System.md`](../docs/System.md)

---

## 1. Problem & Motivation

When `--smart` mode is enabled, Cati searches candidate widths (and native sub-cell steps) to select the dimension maximizing PSNR/SSIM. For complex images, high target resolutions, or large combinatorial glyph mode unions (e.g. `z` / `z+`), evaluating all candidate reconstructions can take several seconds and stall interactive viewing and CLI rendering.

## 2. Technical Specification

- Add `--smart-timeout` flag (default: `1s`) to CLI and interactive render configs.
- Update `spec/render_modes.yaml` smart policy schema to include default timeout configuration (`timeout: 1s`).
- In `smartPrepareSelected` and candidate scoring loops:
  - Track elapsed evaluation time against the configured timeout deadline.
  - If the timeout expires during candidate search, gracefully stop evaluating further candidates.
  - Select the best candidate found prior to the timeout (or fall back to the base aspect dimensions if no better candidate was evaluated).
- Ensure candidate evaluation runs deterministically (e.g. evaluating closest/widest candidates first so high-value candidates are checked early).

## 3. Implementation Plan

1. Update `spec/render_modes.yaml` and `spec.SmartRenderPolicy` to support timeout duration.
2. Add `--smart-timeout` flag to `cmd/smart_render.go` / root and modes commands.
3. Integrate deadline/context or elapsed-time check into candidate scoring loops in `cmd/smart_render.go`.
4. Add unit tests in `cmd/smart_render_test.go` verifying timeout cancellation and fallback behavior.
