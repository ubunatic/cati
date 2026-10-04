---
title: "Study: Terminal Pixels, Measured and Made"
date: 2026-10-04
author: Cati Team
status: complete
---

# Terminal Pixels, Measured and Made

An image in a terminal sounds like a small trick: sample colors, pick a block character, print ANSI escape codes. Cati began with that premise, but the hard part is preserving what matters in an image while obeying the geometry and limits of a text grid. A terminal cell can encode two vertical pixel rows with a half-block glyph and two colors. Cati builds on that idea with quadrant and sextant blocks, sparkline glyphs, and combinations of render modes. It also provides a player and a directory browser, while exposing the renderer as a Go library.

The project is written in Go, with Cobra for the command-line interface and YAML specifications for render modes, controls, themes, and other interface data. It handles raster images and SVG, and its player uses external media tools for video and audio. Its output relies on Unicode glyph support and 24-bit ANSI color in the user's terminal. The website presents sample images, comparisons, installation commands, and a recorded terminal demo.

The recent work shows how much geometry hides behind a simple command. In late September and early October, users could specify widths, heights, aspect behavior, padding, and modes through different paths. Those options had accumulated separate rules in static rendering and playback. Issue 075 drove a pure geometry planner and the integration of that planner into both paths. Issue 077 then extended coverage to 3×3 and composite modes after mismatches showed that explicit dimensions did not always fill the requested grid or align glyph boundaries. The implementation includes dedicated planner tests and CLI regression tests; the follow-up work documented the pipeline so future changes have a shared model.

One concrete episode illustrates the value of that process. Pixel-style aspect handling had to respect the native subcell lattice of each glyph mode while also accounting for the apparent shape of terminal cells. The fixes introduced pixel and aligned aspect modes, bounded pixel-aspect snapping, allowed alignment padding, and scaled row height in proportion to rendered character width. Commits on October 3 record these changes, and issue 078 captures the remaining edge cases around distortion limits and downscaling. The team also added an interactive aspect demo with 24 cases, so people can compare modes and dimensions using real renders instead of reasoning from formulas alone.

Performance work followed a similarly measurable path. A documented optimization to the 2×3/sextant renderer replaced repeated per-mask pixel scoring with closed-form error calculation and lookup tables, and added early exits to sparkline candidate search. The performance note reports a 40% reduction in sextant render latency and a 27.3% reduction for S2 on its benchmark system. Separately, the project added parity checks between fast and reference rendering paths, and benchmarks that report output mismatches as well as speed. That pairing matters: an optimization is useful only if it remains visually identical.

The work is organized as a ticketed engineering loop. The `issues/` tracker contains numbered tasks with goals, findings, milestones, and verification plans. Recent examples include a six-milestone audit of spec ownership (#076), a geometry unification (#075), and closed issues for CLI completion (#074) and spec-driven browser controls. Documentation records architecture, test strategy, rendering pitfalls, and performance measurements. Studies also preserve what went wrong: a September post-mortem describes excessive orchestration polling and overlapping agent work, then turns those failures into explicit operating rules. This is a more instructive use of agents than simply counting parallel tasks: investigate, assign bounded work, test the result, and retain a candid account of the workflow.

As of October 4, the repository records 255 commits in the preceding six weeks, 27 pull-request merge commits in that window, 65 Go test files, 132 Go source files, and 38,202 tracked Go lines. Those numbers do not prove that a solo developer could not build Cati; they do show sustained breadth across renderers, the CLI, media playback, browser UI, YAML specifications, performance work, and regression coverage. Agentic development is most visible here in the volume and traceability: tickets connect decisions to changes, tests protect the tricky boundaries, and studies capture both successful techniques and coordination failures. The result is software that can keep improving without treating each hard bug as a one-off mystery.

## Facts

Git history records 31 commits by `google-labs-jules[bot]` in the six weeks
ending 2026-10-05. The companion
[Jules performance study](google-jules-performance-story.md) describes
representative Cati and Loom Games commits.

| | |
|---|---|
| Stack | Go 1.26, Cobra, YAML specs, Unicode and ANSI terminal output, external video/audio tools |
| Size | 132 Go files; 65 Go test files; 38,202 tracked Go lines (2026-10-04) |
| Activity | 255 commits and 27 PR merge commits in six weeks ending 2026-10-04 |
| Milestones | Geometry planner and integration (#075); spec ownership audit (#076); 2×3/S2 performance improvements (4f60bf2) |
