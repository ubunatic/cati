---
title: Render Experiment Lessons
weight: 70
---

# Render Experiment Lessons


June 2026 cleanup: `xs` / `sextant/2x3` is the only new shipped render
algorithm. The experimental sextant search aliases (`xg`, `xb`) and geomshape
aliases (`sh`, `shg`, `shb`) were removed from code, CLI parsing, cycling,
metrics, and tests.

A later pass also removed `sg` / `spark/geom` (the `chooseGeomCandidates`
heuristic that switched between quad and sextant candidate sets per block). It
added no quality over `spark/best`, which already scores the union of both
candidate sets, so it was dead weight in the cycle. Its `render_spark_geom_*` /
`cli_spark_geom_*` goldens were dropped with it.

Key rules retained from the failed experiments:

- Every renderer must declare its terminal-cell source footprint and aspect
  correction in the shared pipeline.
- Static and playback paths must fail with `render aspect mismatch` before ANSI
  output when the rendered viewport no longer matches the source aspect.
- Opaque source regions must not emit terminal-default background holes.
- Experimental aliases should not be public CLI modes until they pass the same
  aspect, gap, and golden-image invariants as shipped renderers.
