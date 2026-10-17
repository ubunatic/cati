# 104 — Update tui_integration_test.go for post-split CLI flags and spec-driven mode names

**Status**: Closed — Fixed in 8078566; full fast/reference PTY suites and container CI pass
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [#024](024-split-cli-player-browser-binaries.md), [#025](025-spec-driven-render-modes.md), [#103](103-integration-tag-root-tests-use-package-main-instead-of-cati.md), [Testing](../docs/Testing.md)

---

## 1. Problem & Motivation
Now that issue #103 moved `tui_integration_test.go` to `package cati_test` so the package builds and vets under `-tags=integration`, runtime execution of `make test-integration` reveals that several assertions in `tui_integration_test.go` still reference pre-split CLI flags (such as `-w` instead of `-W`) and pre-spec render mode names (such as `halfblock`, `quad/splithalf`, `spark/quad`).

## 2. Technical Specification / Findings
1. `TestInteractiveAndStaticHalfblockRenderMatch` passes `-w 50`, but static `cati` uses `-W` for width, causing `Error: unknown shorthand flag: 'w' in -w`.
2. `TestInteractiveModeHalfStartsHalfblockAndCyclesToQuad`, `TestInteractiveGradientConvergentModesShowPerfectSSIM`, `TestInteractiveSolidRedAllModesShowPerfectSSIM`, and `TestInteractiveVideoFramesDoNotWrapAcrossRenderModes` assert strings like `halfblock`, `quad/splithalf`, `spark/quad`, whereas spec-driven mode names and status headers now display `half`, `quad`, `bars`, `six`, etc.
3. The cycle sequence triggered by pressing `r` in interactive mode now follows `spec/render_modes.yaml` (`2x4`, `full`, `half`, `quad`, ...), diverging from the historic hardcoded 3-mode sequence.

## 3. Implementation & Verification Plan
1. Update `tui_integration_test.go` flag invocations from `-w` to `-W`.
2. Update expected mode strings and status bar tokens to reflect the spec-driven mode names declared in `spec/render_modes.yaml`.
3. Preserve terminal geometry assertions and verify with `make test-integration`.
