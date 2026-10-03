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
| 017 | [017-gpu-glyph-mapping-spark-mode.md](017-gpu-glyph-mapping-spark-mode.md) | GPU Assistance for Glyph Mapping, Starting with Spark Mode | 🟡 In Progress |
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
| 033 | [033-render-unconditionally-emits-erase-line-cr-prefix-breaks-composed-layouts.md](033-render-unconditionally-emits-erase-line-cr-prefix-breaks-composed-layouts.md) | Render() unconditionally emits erase-line+CR prefix, breaks composed layouts | Closed — opt-out implemented and verified: NoLinePrefix on halfblock/quadblock/sextant Options; go vet/go test/make install green |
| 034 | [034-quadblock-and-sextant-have-no-image-loader-must-import-halfblock-loadimage.md](034-quadblock-and-sextant-have-no-image-loader-must-import-halfblock-loadimage.md) | quadblock and sextant have no image loader, must import halfblock.LoadImage | Open |
| 035 | [035-make-png-golden-comparisons-ignore-non-rendering-metadata.md](035-make-png-golden-comparisons-ignore-non-rendering-metadata.md) | Make PNG golden comparisons ignore non-rendering metadata | Closed (Verified with metadata isolation and negative difference tests in `cmd/golden_render_test.go`) |
| 036 | [036-add-smart-render-mode-that-selects-the-best-width-by-psnr.md](036-add-smart-render-mode-that-selects-the-best-width-by-psnr.md) | Add smart render mode that selects the best width by PSNR | Open |
| 037 | [037-extend-smart-rendering-to-algorithm-native-sub-cell-steps.md](037-extend-smart-rendering-to-algorithm-native-sub-cell-steps.md) | Extend smart rendering to algorithm-native sub-cell steps | Closed — resolved in 322cca7 |
| 038 | [038-investigate-blocky-sextant-rendering-at-small-widths.md](038-investigate-blocky-sextant-rendering-at-small-widths.md) | Investigate blocky sextant rendering at small widths | Closed |
| 039 | [039-improve-quad-quality-for-small-pixel-art-renders.md](039-improve-quad-quality-for-small-pixel-art-renders.md) | Improve quad quality for small pixel-art renders | Closed |
| 040 | [040-assess-halfblock-rendering-quality-at-small-widths.md](040-assess-halfblock-rendering-quality-at-small-widths.md) | Assess halfblock rendering quality at small widths | Closed |
| 041 | [041-assess-sparkline-and-composite-rendering-quality-at-small-widths.md](041-assess-sparkline-and-composite-rendering-quality-at-small-widths.md) | Assess sparkline and composite rendering quality at small widths | Closed |
| 042 | [042-add-info-metadata-to-cati-modes-demos.md](042-add-info-metadata-to-cati-modes-demos.md) | Add `--info` metadata to `cati modes` demos | Closed |
| 043 | [043-compose-named-and-debug-render-modes-from-glyph-set-ids.md](043-compose-named-and-debug-render-modes-from-glyph-set-ids.md) | Compose named and debug render modes from glyph set IDs | Closed |
| 044 | [044-modes-command-custom-image-input-support.md](044-modes-command-custom-image-input-support.md) | Modes Command Custom Image Input Support | Closed (Implemented `-i / --image` and positional path support in `cmd/modes.go`) |
| 045 | [045-modes-command-test-asset-presets-and-sample-shortcuts.md](045-modes-command-test-asset-presets-and-sample-shortcuts.md) | Modes Command Test Asset Presets and Sample Shortcuts | Closed (Implemented in `cmd/modes_presets.go` and `cmd/modes.go`) |
| 046 | [046-modes-command-dataset-benchmark-scorecard.md](046-modes-command-dataset-benchmark-scorecard.md) | Modes Command Dataset Benchmark Scorecard | Closed (Implemented in `cmd/modes_benchmark.go` and `cmd/modes.go`) |
| 047 | [047-smart-render-search-timeout.md](047-smart-render-search-timeout.md) | Abort --smart Search After 1s (--smart-timeout Default) | Open |
| 048 | [048-automatic-natural-aspect-ratio-for-custom-glyph-modes-and-safe-override-api.md](048-automatic-natural-aspect-ratio-for-custom-glyph-modes-and-safe-override-api.md) | Automatic Natural Aspect Ratio for Custom Glyph Modes and Safe Override API | Closed |
| 049 | [049-modes-command-grid-output-layout-and-column-fitting.md](049-modes-command-grid-output-layout-and-column-fitting.md) | Modes Command Grid Output Layout and Column Fitting | Closed |
| 050 | [050-zero-allocation-and-lut-acceleration-for-sextant-and-quad-block-renderers.md](050-zero-allocation-and-lut-acceleration-for-sextant-and-quad-block-renderers.md) | Zero-Allocation and LUT Acceleration for Sextant and Quad Block Renderers | Closed |
| 051 | [051-fast-contiguous-luminance-buffer-for-ssim-scoring-pipeline.md](051-fast-contiguous-luminance-buffer-for-ssim-scoring-pipeline.md) | Fast Contiguous Luminance Buffer for SSIM Scoring Pipeline | Open |
| 052 | [052-unified-composite-quality-metric-and-efficiency-index.md](052-unified-composite-quality-metric-and-efficiency-index.md) | Unified Composite Quality Metric and Efficiency Index | Open |
| 053 | [053-gate-loom-list-panel-search-behind-slash-option.md](053-gate-loom-list-panel-search-behind-slash-option.md) | Gate Loom List Panel Search Behind Option ("Type to Search" vs "[/] Search") | Closed (Implemented GatedSearch in loom Choice and integrated into cati imgbrowser) |
| 054 | [054-review-jules-bolt-perf-patches.md](054-review-jules-bolt-perf-patches.md) | Review of Jules "Bolt" Perf Patches (PR #8, #9) | Closed (post-factum: worker cap moved to Makefile env, all fast paths behind shared CATI_FASTPATH) |
| 055 | [055-metrics-luma-fastpath-drift.md](055-metrics-luma-fastpath-drift.md) | Metrics Luma Fast Paths Diverge From Simple Path | Open |
| 056 | [056-cati-bench-mediafile-per-asset-render-speed-benchmark.md](056-cati-bench-mediafile-per-asset-render-speed-benchmark.md) | `cati --bench <mediafile>`: Per-Asset Render Speed Benchmark | Closed |
| 057 | [057-honour-spec-experimental-render-modes-across-mode-consumers.md](057-honour-spec-experimental-render-modes-across-mode-consumers.md) | Honour spec experimental render modes across mode consumers | Open |
| 058 | [058-cati-bench-reports-fast-simple-output-mismatches.md](058-cati-bench-reports-fast-simple-output-mismatches.md) | cati --bench reports fast/simple output mismatches | Closed |
| 059 | [059-openvideostream-reader-and-cleanup-both-call-cmd-wait-stop-hangs-unless-channel-is-drained.md](059-openvideostream-reader-and-cleanup-both-call-cmd-wait-stop-hangs-unless-channel-is-drained.md) | OpenVideoStream reader and cleanup both call cmd.Wait, stop hangs unless channel is drained | Open |
| 060 | [060-polish-and-test-new-mediabrowse-example.md](060-polish-and-test-new-mediabrowse-example.md) | Polish and test new mediabrowse split-pane example | Closed |
| 061 | [061-loom-view-truncates-ansi-files.md](061-loom-view-truncates-ansi-files.md) | Loom view truncates ANSI files | Closed |
| 062 | [062-async-media-loading-and-rendering-with-progress.md](062-async-media-loading-and-rendering-with-progress.md) | Async Media Loading and Rendering with Progress | Closed |
| 063 | [063-show-only-known-loom-themes-in-mediabrowse.md](063-show-only-known-loom-themes-in-mediabrowse.md) | Show only known Loom themes in mediabrowse | Closed |
| 064 | [064-use-loom-s-standard-navigation-pane-in-mediabrowse.md](064-use-loom-s-standard-navigation-pane-in-mediabrowse.md) | Use Loom's standard navigation pane in mediabrowse | Closed |
| 065 | [065-mediabrowse-video-playback-and-fullscreen-shortcuts-do-not-work.md](065-mediabrowse-video-playback-and-fullscreen-shortcuts-do-not-work.md) | Mediabrowse video playback and fullscreen shortcuts do not work | Closed |
| 066 | [066-mediabrowse-play-pause-should-not-reload-the-video-preview.md](066-mediabrowse-play-pause-should-not-reload-the-video-preview.md) | Mediabrowse play/pause should not reload the video preview | Closed |
| 067 | [067-make-test-fails-makefile-pins-gotoolchain-go1-25-0-but-go-mod-needs-1-26.md](067-make-test-fails-makefile-pins-gotoolchain-go1-25-0-but-go-mod-needs-1-26.md) | make test fails: Makefile pins GOTOOLCHAIN=go1.25.0 but go.mod needs 1.26 | Open |
| 068 | [068-re-init-harnez-with-quota-1-add-test-q1-target-harnez-rules-and-update-docs.md](068-re-init-harnez-with-quota-1-add-test-q1-target-harnez-rules-and-update-docs.md) | Re-init harnez with --quota-1: add test-q1 target, harnez rules and update docs | Open |
| 069 | [069-add-2x4-dot-only-braille-render-mode.md](069-add-2x4-dot-only-braille-render-mode.md) | Add 2x4 dot-only braille render mode | Resolved |
| 070 | [070-add-2x4-braille-foreground-and-background-render-mode.md](070-add-2x4-braille-foreground-and-background-render-mode.md) | Add 2x4 braille foreground and background render mode | Resolved |
| 071 | [071-respect-explicit-width-and-height-in-static-renders.md](071-respect-explicit-width-and-height-in-static-renders.md) | Respect explicit width and height in static renders | Closed — unified in CLI geometry pipeline and pure planner |
| 072 | [072-add-aspect-modes-for-cli-source-mapping.md](072-add-aspect-modes-for-cli-source-mapping.md) | Add aspect modes for CLI source mapping | Closed — unified in CLI geometry pipeline and pure planner |
| 073 | [073-add-pixel-matched-s2-and-h2-render-paths.md](073-add-pixel-matched-s2-and-h2-render-paths.md) | Add pixel-matched s2 and h2 render paths | Open |
| 074 | [074-add-cobra-cli-completion-for-files-and-flags.md](074-add-cobra-cli-completion-for-files-and-flags.md) | Add Cobra CLI completion for files and flags | Closed — implemented Cobra CLI completion for files and flags |
| 075 | [075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md](075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md) | Unify CLI render geometry contracts and validate route-specific options | Closed — unified in CLI geometry pipeline and pure planner |
| 076 | [076-audit-spec-driven-approach-violations-and-classify-candidates.md](076-audit-spec-driven-approach-violations-and-classify-candidates.md) | Spec ownership audit: confirmed gaps and policy candidates | Open |
| 077 | [077-cover-3x3-and-all-composite-modes-in-geometry-planner-and-fix-line-count-padding-mismatches.md](077-cover-3x3-and-all-composite-modes-in-geometry-planner-and-fix-line-count-padding-mismatches.md) | Cover 3x3 and all composite modes in geometry planner and fix line count/padding mismatches | Open |

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
