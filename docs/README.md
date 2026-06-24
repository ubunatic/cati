# Cati Documentation Index

Welcome to the Cati developer documentation. The following resources cover our project guides, code rules, and architecture:

*   [System Documentation](System.md) — Comprehensive guide on rendering pipeline, design decisions, offline website asset generation, and licensing.
    *   [Video Probing & Streaming Pipeline](Video.md) — Details on native FPS probing, ffmpeg piping, asynchronous decoding, and queue frame dropping.
    *   [Interactive Grid Browser](Browser.md) — Page layouts, composite thumbnail grids, mouse tracking, async thumb loading, space-pan, raw terminal swaps.
*   [Spec System & Browser Design](Design.md) — The `spec/` YAML-driven config system: template engine (`renderTpl`/`if()`), color system (`dark`/`light`/named/hex), button/label/view pipeline, hint bar vars, scrollbar, dense-mode grid, split-screen preview.
*   [Quad-Block Pixel Art](QuadPixelArt.md) — Half-block vs. quad-block layout math, the 2× horizontal stretch aspect-ratio correction, neighbour-aware colour quantisation, and the quadrant character lookup table.
*   [Go Conventions](Go.md) — Development guidelines for writing Go code, state management, error handling, CLI verbs, and testing.
*   [Make Conventions](Make.md) — Standardized Makefile structures, target phony declarations using the sentinel `⚙️` trick, and self-documenting rules.

---

## Issues

Tracked in [`../issues/`](../issues/README.md) — concrete bugs, design problems, and features with resolution notes.
