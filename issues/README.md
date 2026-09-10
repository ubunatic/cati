# Issues Index

Concrete current and past issues: bugs, design decisions, open features.

| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [001-quad-quality-algos.md](001-quad-quality-algos.md) | Quad Image Quality: Advanced Rendering Algorithms | 🔄 In Progress |
| 002 | [002-browser-input-handling.md](002-browser-input-handling.md) | Browser Input: q/ESC/^C not working in browser + video viewer | ✅ Closed |
| 003 | [003-style-consistency.md](003-style-consistency.md) | Style: hint bar used hardcoded reverse-video instead of theme palette | ✅ Closed |
| 004 | [004-spec-loose-ends.md](004-spec-loose-ends.md) | Spec System: Loose Ends & Unresolved Stubs | 🔴 Open |
| 005 | [005-video-ffmpeg-scaling.md](005-video-ffmpeg-scaling.md) | Video: ffmpeg-side scaling for rawvideo pipe | 🔴 Open |
| 006 | [006-quality-metrics-and-pixel-art-scaling.md](006-quality-metrics-and-pixel-art-scaling.md) | Quality Metrics: Blockiness, Edge Continuity, and Pixel-Art Scaling | 🔄 In Progress (Phase 5 remaining) |
| 007 | [007-meta-loading-perf.md](007-meta-loading-perf.md) | Media Metadata: ffprobe called for images on hover | 🔴 Open |
| 008 | [008-spaghetti-code-analysis.md](008-spaghetti-code-analysis.md) | Spaghetti Code & Viewport Geometry Refactoring Plan | 🔄 In Progress |
| 009 | [009-explore-more-sparkline-rendering-modes.md](009-explore-more-sparkline-rendering-modes.md) | Explore more sparkline rendering modes | Open |
| 010 | [archive/010-unified-zoom-geometry-and-ladder.md](archive/010-unified-zoom-geometry-and-ladder.md) | Unified Zoom Geometry and Ladder (Archived) | 🗄️ Archived — content merged into [008 — Spaghetti Code & Viewport Geometry Refactoring Plan](../008-spaghetti-code-analysis.md) |
| 011 | [011-sparkline-inversion-and-stripe-bugs.md](011-sparkline-inversion-and-stripe-bugs.md) | Sparkline Inversion and Stripe Bugs | ✅ Closed |
| 012 | [archive/012-viewport-geometry-mode-switch-regression.md](archive/012-viewport-geometry-mode-switch-regression.md) | Viewport Geometry Regression on Render-Mode Switch (Archived) | 🗄️ Archived — content merged into [008 — Spaghetti Code & Viewport Geometry Refactoring Plan](../008-spaghetti-code-analysis.md) |
| 013 | [013-shared-analysis-grid-for-halfblock-and-quadblock.md](013-shared-analysis-grid-for-halfblock-and-quadblock.md) | Shared Analysis Grid for Halfblock and Quadblock Convergence | 🔴 Open |
| 014 | [014-more-boxdrawing-chars-unicode-v13.md](014-more-boxdrawing-chars-unicode-v13.md) | More Unicode goodness | In Progress |
| 015 | [015-spark-bottom-row-halfcell-fit.md](015-spark-bottom-row-halfcell-fit.md) | Spark/quad garbled bottom row at mid-cell fit heights | ✅ Closed |
| 016 | [016-worker-copy-consolidation.md](016-worker-copy-consolidation.md) | Worker-Copy Render Paths: Keep Clones for Tuning, Consolidate Later | 🔴 Open |
| 017 | [017-gpu-glyph-mapping-spark-mode.md](017-gpu-glyph-mapping-spark-mode.md) | GPU Assistance for Glyph Mapping, Starting with Spark Mode | 🔴 Open |
| 018 | [018-sparkline-allocation-reduction.md](018-sparkline-allocation-reduction.md) | Reduce Allocation Pressure in Spark Mode | 🔴 Open |
| 019 | [019-video-audio-drift-on-long-playback.md](019-video-audio-drift-on-long-playback.md) | Video audio drift on longer playback | 🔴 Open |
| 020 | [020-sextant-column-mask-nul-glyph.md](020-sextant-column-mask-nul-glyph.md) | Sextant pure-column masks emit NUL glyph (garbled right edge) | ✅ Closed |
| 021 | [021-golden-storage-resolution-all-algos.md](021-golden-storage-resolution-all-algos.md) | Golden storage resolution that exactly covers all current & future algos | 🟢 Closed |
| 022 | [022-svg-fixed-rasterization-size.md](022-svg-fixed-rasterization-size.md) | SVG: fixed 2048px rasterization ceiling, no target-size scaling | ✅ Closed |
| 023 | [023-sextant-golden-non-transp.md](023-sextant-golden-non-transp.md) | Sextant golden non-transparent coverage | Open |
| 024 | [024-split-cli-player-browser-binaries.md](024-split-cli-player-browser-binaries.md) | Split CLI, Player, and Browser Binaries | In Progress |
| 025 | [025-spec-driven-render-modes.md](025-spec-driven-render-modes.md) | Spec-driven render modes, glyph families, geometry, and colorers | 🔄 In Progress |
| 026 | [026-small-image-aspect-ratio-scale-bugs.md](026-small-image-aspect-ratio-scale-bugs.md) | Small image aspect ratio and scale bugs for spark/sextant modes | ✅ Closed |
| 027 | [027-demo-widths-obsolete-modes.md](027-demo-widths-obsolete-modes.md) | demo_widths.go uses obsolete render mode names | ✅ Closed |
| 028 | [028-v2-unconstrained-static-fit-aspect.md](028-v2-unconstrained-static-fit-aspect.md) | V2 unconstrained static fit aspect mismatch | ✅ Closed |
| 029 | [029-v2-default-static-fit-full-terminal-width.md](029-v2-default-static-fit-full-terminal-width.md) | V2 default static fit uses full terminal width | ✅ Closed |
| 030 | [030-website-asset-references-and-lfs.md](030-website-asset-references-and-lfs.md) | Website assets: out-of-dir references and LFS pointer pitfalls | ✅ Closed (2026-07-04) |
| 031 | [031-remove-ansi-golden-goldens.md](031-remove-ansi-golden-goldens.md) | Remove `.ansi` golden tests (unverifiable byte diffs let a fix go stale) | ✅ Closed |
| 032 | [032-jpeg-golden-toolchain-drift.md](032-jpeg-golden-toolchain-drift.md) | JPEG-sourced golden drift across Go toolchain patch versions | ✅ Closed |
| 033 | [033-render-unconditionally-emits-erase-line-cr-prefix-breaks-composed-layouts.md](033-render-unconditionally-emits-erase-line-cr-prefix-breaks-composed-layouts.md) | Render() unconditionally emits erase-line+CR prefix, breaks composed layouts | Open |
| 034 | [034-quadblock-and-sextant-have-no-image-loader-must-import-halfblock-loadimage.md](034-quadblock-and-sextant-have-no-image-loader-must-import-halfblock-loadimage.md) | quadblock and sextant have no image loader, must import halfblock.LoadImage | Open |
| 035 | [035-make-png-golden-comparisons-ignore-non-rendering-metadata.md](035-make-png-golden-comparisons-ignore-non-rendering-metadata.md) | Make PNG golden comparisons ignore non-rendering metadata | Open |
| 036 | [036-add-smart-render-mode-that-selects-the-best-width-by-psnr.md](036-add-smart-render-mode-that-selects-the-best-width-by-psnr.md) | Add smart render mode that selects the best width by PSNR | Open |
| 037 | [037-extend-smart-rendering-to-algorithm-native-sub-cell-steps.md](037-extend-smart-rendering-to-algorithm-native-sub-cell-steps.md) | Extend smart rendering to algorithm-native sub-cell steps | Closed — resolved in 322cca7 |
| 038 | [038-investigate-blocky-sextant-rendering-at-small-widths.md](038-investigate-blocky-sextant-rendering-at-small-widths.md) | Investigate blocky sextant rendering at small widths | Closed |
| 039 | [039-improve-quad-quality-for-small-pixel-art-renders.md](039-improve-quad-quality-for-small-pixel-art-renders.md) | Improve quad quality for small pixel-art renders | Closed |
| 040 | [040-assess-halfblock-rendering-quality-at-small-widths.md](040-assess-halfblock-rendering-quality-at-small-widths.md) | Assess halfblock rendering quality at small widths | Open |
| 041 | [041-assess-sparkline-and-composite-rendering-quality-at-small-widths.md](041-assess-sparkline-and-composite-rendering-quality-at-small-widths.md) | Assess sparkline and composite rendering quality at small widths | Open |

## Archived

| # | Title | Reason |
|---|-------|--------|
| [010](archive/010-unified-zoom-geometry-and-ladder.md) | Unified zoom geometry and ladder | Consolidated into #008 |
| [012](archive/012-viewport-geometry-mode-switch-regression.md) | Viewport geometry regression on render-mode switch | Consolidated into #008 |

## Status legend
- 🔴 Open
- 🔄 In Progress
- ✅ Closed
- 🚫 Won't fix
- 🗄️ Archived
