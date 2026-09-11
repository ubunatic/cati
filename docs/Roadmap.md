# Cati Roadmap

This document outlines the strategic product direction for Cati, using a Now / Next / Later sequencing to clarify our immediate focus, upcoming priorities, and longer-term aspirations.

## Now (Priority 1): "cati modes" & Quality Metrics
*Our immediate focus is on the Analysis Suite and establishing a rigorous foundation for measuring visual quality.*

*   **Custom Image Input Support** (Issue #044): ✅ Completed — `-i / --image` and positional input support for single and pair analysis.
*   **Test Asset Presets & Sample Shortcuts** (Issue #045): ✅ Completed — `-p / --preset / --sample` shortcuts and listing for geometric and photographic suites.
*   **Dataset Benchmark Scorecard** (Issue #046): ✅ Completed — `--benchmark / --suite` multi-image evaluation scorecard with category SSIM, latencies, and efficiency index.
*   **Golden Comparison & Metadata** (Issue #035): ✅ Completed — verified metadata-independent comparison with pixel difference detection tests.
*   **Related items:** Foundational two-phase isolated benchmarking and quality metrics.

## Next (Priority 2): Rendering Engine & Glyph Quality
*Once we can measure quality accurately, we will refine the core rendering capabilities and explore advanced optimization.*

*   **Composable Glyph Sets & Unicode** (Issue #014): Expand coverage for Box Drawing and Block Element characters (e.g., Unicode v13 features).
*   **Smart Render Mode by PSNR** (Issue #036): Introduce dynamic rendering algorithms that auto-select the best dimensions and glyphs based on objective quality metrics.
*   **Combinatorial Solver Search & Optimization:** Investigate fast solver search mechanisms for optimal glyph selection and potential SIMD/GPU accelerations (e.g., legacy issue #017).
*   **Rendering Edge Cases** (Issue #023, #033): Resolve structural bugs like unconditional erase-line emissions and sextant transparency handling.

## Later (Priority 3): Library & Ecosystem
*With a fast, high-quality core engine, we will solidify Cati for broad adoption as a developer tool.*

*   **Go Library Stability:** Finalize the `v1/` Go module API for external consumers.
*   **Clean Headless APIs:** Refactor image loaders and render targets (e.g., Issue #034: unify image loaders across `quadblock` and `sextant`).
*   **Zero-Dependency Guarantees:** Ensure the core library operates seamlessly without bloated dependencies, providing standard interfaces.
*   **Binary Separation** (Issue #024): Split CLI, Player, and Browser into dedicated binaries for cleaner distributions.

## Later (Priority 4): Interactive Tools
*Our ultimate goal is to build powerful, interactive terminal experiences on top of the Cati engine.*

*   **`catiplay` & `catibrowse`:** Develop the interactive grid browser and video player components.
*   **Advanced UX Features:** Implement zooming, panning, space-pan navigation, and smooth video scrubbing inside standard terminal emulators.

---
*Note: This roadmap is a living document. Priorities may shift based on user feedback and technical discoveries.*
