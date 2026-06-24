# Cati Documentation Index

Welcome to the Cati developer documentation. The following resources cover our project guides, code rules, and architecture:

*   [System Documentation](System.md) — Comprehensive guide on rendering pipeline, design decisions, offline website asset generation, and licensing.
    *   [Video Probing & Streaming Pipeline](Video.md) — Details on native FPS probing, ffmpeg piping, asynchronous decoding, and queue frame dropping.
    *   [Interactive Grid Browser](Browser.md) — Details on page layouts, compositing thumbnail grids, mouse tracking, and raw terminal swaps.
*   [Quad-Block Pixel Art](QuadPixelArt.md) — Half-block vs. quad-block layout math, the 2× horizontal stretch aspect-ratio correction, neighbour-aware colour quantisation, and the quadrant character lookup table.
*   [Go Conventions](Go.md) — Development guidelines for writing Go code, state management, error handling, CLI verbs, and testing.
*   [Make Conventions](Make.md) — Standardized Makefile structures, target phony declarations using the sentinel `⚙️` trick, and self-documenting rules.
