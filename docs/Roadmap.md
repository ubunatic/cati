# Cati Roadmap

This document outlines the strategic product direction for Cati, using a Now / Next / Later sequencing to clarify our immediate focus, upcoming priorities, and longer-term aspirations.

---

## Now (Priority 1): Real-Time Solver Acceleration & Fast Quality Metrics
*Our immediate focus is on eliminating inner-loop allocations, accelerating glyph selection solvers, and building sub-millisecond perceptual quality scoring.*

*   **Zero-Allocation & LUT Acceleration for Block Renderers** ([Issue #050](../issues/050-zero-allocation-and-lut-acceleration-for-sextant-and-quad-block-renderers.md)):
    *   Direct `[64]rune` array LUTs replacing `map[uint8]rune` hash lookups.
    *   Zero-allocation stack scratches for candidate search in `scoreMask` and `heuristicMasks`.
    *   Hardware `POPCNT` via `math/bits.OnesCount8`.
    *   Single-pass between-cluster variance (Otsu criterion) evaluating candidate masks without 2-pass distance math.
*   **Fast Contiguous Luminance Buffer for SSIM Pipeline** ([Issue #051](../issues/051-fast-contiguous-luminance-buffer-for-ssim-scoring-pipeline.md)):
    *   Linear sequential luminance extraction avoiding `image.Image.At()` virtual method calls.
    *   Contiguous slice/pointer stride window traversals reducing SSIM evaluation to $<0.3\,\text{ms}$.
*   **Unified Composite Quality Metric & Efficiency Index** ([Issue #052](../issues/052-unified-composite-quality-metric-and-efficiency-index.md)):
    *   Standardize $Q_{\text{composite}} = 0.60 \cdot \text{SSIM} + 0.25 \cdot E_{\text{cont}} + 0.15 \cdot B_{\text{score}}$ in `internal/metrics`.
    *   Standardize the Efficiency Index ($\text{Eff} = (1.0 - Q) \times T_{\text{render}}$) for Pareto-optimal sorting in `cati modes --sort eff` and interactive UI.
*   **Recently Completed Analysis Foundations**:
    *   ✅ Custom Image Input Support ([Issue #044](../issues/044-modes-command-custom-image-input-support.md))
    *   ✅ Test Asset Presets & Sample Shortcuts ([Issue #045](../issues/045-modes-command-test-asset-presets-and-sample-shortcuts.md))
    *   ✅ Dataset Benchmark Scorecard ([Issue #046](../issues/046-modes-command-dataset-benchmark-scorecard.md))
    *   ✅ Metadata-Independent Golden Comparisons ([Issue #035](../issues/035-make-png-golden-comparisons-ignore-non-rendering-metadata.md))

---

## Next (Priority 2): Rendering Engine & Golden Test Harness
*Once real-time solvers and metrics operate under sub-millisecond budgets, we expand glyph coverage and automated regression tests.*

*   **Golden PNG `tEXt` Metric Embedding**:
    *   Embed $Q_{\text{composite}}$, $E_{\text{cont}}$, and $B_{\text{score}}$ directly into golden PNG `tEXt` chunks for automated CI regression thresholds.
*   **Composable Glyph Sets & Unicode** ([Issue #014](../issues/014-more-boxdrawing-chars-unicode-v13.md), [Issue #043](../issues/043-compose-named-and-debug-render-modes-from-glyph-set-ids.md)):
    *   Expand coverage for Box Drawing and Block Element characters (Unicode v13).
*   **Smart Render Mode & Width Optimizer** ([Issue #036](../issues/036-add-smart-render-mode-that-selects-the-best-width-by-psnr.md), [Issue #047](../issues/047-smart-render-search-timeout.md)):
    *   Introduce dynamic width probing that optimizes character grid columns against aspect ratio and metric score with search timeout safeguards.
*   **Rendering Edge Cases & Structural Fixes** ([Issue #023](../issues/023-sextant-golden-non-transp.md), [Issue #033](../issues/033-render-unconditionally-emits-erase-line-cr-prefix-breaks-composed-layouts.md)):
    *   Resolve structural bugs like unconditional erase-line emissions and sextant transparency handling.

---

## Later (Priority 3): Library & Ecosystem
*With a fast, high-quality core engine, we will solidify Cati for broad adoption as a developer tool.*

*   **Go Library Stability**: Finalize the `v1/` Go module API for external consumers.
*   **Clean Headless APIs** ([Issue #034](../issues/034-quadblock-and-sextant-have-no-image-loader-must-import-halfblock-loadimage.md)): Refactor image loaders and render targets to eliminate cross-package helper dependencies.
*   **Zero-Dependency Guarantees**: Ensure core render packages operate without external runtime dependencies.
*   **Binary Separation** ([Issue #024](../issues/024-split-cli-player-browser-binaries.md)): Maintain cleanly partitioned CLI, Player (`catiplay`), and Browser (`catibrowse`) binaries.

---

## Later (Priority 4): Interactive Tools & Playback
*Building rich, interactive terminal experiences on top of the optimized Cati engine.*

*   **Interactive Image & Video Grid Browser (`catibrowse`, `imgbrowser`)**:
    *   Fast side-pane inspection views (`#` toggle) showing real-time geometry, scaling factor, and composite quality metrics.
    *   Zoom, pan, and space-pan navigation with aspect-ratio preservation.
*   **Video Playback (`catiplay`)**:
    *   Smooth frame streaming, audio synchronization ([Issue #019](../issues/019-video-audio-drift-on-long-playback.md)), and ffmpeg-side scaling ([Issue #005](../issues/005-video-ffmpeg-scaling.md)).

---
*Note: This roadmap is a living document. Priorities are reviewed and updated as performance milestones are achieved.*
