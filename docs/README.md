# Cati Documentation Index

Welcome to the Cati developer documentation. The following resources cover our project guides, code rules, and architecture:

*   [System Documentation](System.md) — Rendering pipeline, design decisions, viewer core consolidation, line-width invariant, offline website asset generation, and licensing.
    *   [Render Pipelines](RenderPipelines.md) — Step-by-step Mermaid diagrams and spatial character art pixel flows for each render mode (half-block, quad-block, sextant, sparkline).
    *   [Video & Audio Pipeline](Video.md) — FPS/dimension probing, rawvideo pipe, one-frame-per-tick loop, play-once vs loop, input resilience, audio via ffplay.
    *   [Interactive Grid Browser](Browser.md) — Page layouts, composite thumbnail grids, mouse tracking, async thumb loading, space-pan, raw terminal swaps.
*   [Terminal Input System](Input.md) — `spec/input.yaml` decision tree, `internal/input` package, SGR 1006 mouse protocol, UTF-8 tokenization, move vs drag, `--input-test` TUI. **Read before touching input handling or `spec/input.yaml`.**
*   [Spec System — Authoritative Reference](Spec.md) — Spec-as-code philosophy, file map, key dispatch pipeline, quality invariants, agent rules, integrity tests, and change checklist. **Read this before touching any `spec/` file or its Go loaders.**
*   [Spec System & Browser Design](Design.md) — The `spec/` YAML-driven config system: template engine (`renderTpl`/`if()`), color system, button/label/view pipeline, full hint-bar variable table (`meta.*`, `ssim`, `last_key`, …), scrollbar, dense-mode grid, split-screen preview.
*   [Quad-Block Pixel Art](QuadPixelArt.md) — Half-block vs. quad-block layout math, the 2× horizontal stretch aspect-ratio correction, neighbour-aware colour quantisation, and the quadrant character lookup table.
*   [Sparkline Pixel Art](SparklinePixelArt.md) — Sparkline layout math, horizontal and vertical orientation, optimal-split character and color selection, and the test helper suite.
*   [Render Experiment Lessons](RenderExperimentLessons.md) — Short notes from the removed sextant search and geomshape render experiments.
*   [Rendering Bug & Golden-Change Playbook](RenderingBugPlaybook.md) — How to diagnose visual/geometry bugs (prove the root cause with numbers first) and change golden images safely. **Read before fixing any rendering bug or touching `testdata/` goldens.**
*   [Go Conventions](Go.md) — Development guidelines for writing Go code, state management, error handling, CLI verbs, and testing.
*   [Make Conventions](Make.md) — Standardized Makefile structures, target phony declarations using the sentinel `⚙️` trick, and self-documenting rules.

---

## Issues

Tracked in [`../issues/`](../issues/README.md) — concrete bugs, design problems, and features with resolution notes.
