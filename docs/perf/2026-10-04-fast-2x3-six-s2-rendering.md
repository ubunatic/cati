# Performance Optimization: Closed-Form Mask Scoring and Early Exit in 2x3, Six, and S2 Renderers

## Overview

Component: `v1/sextant`, `v1/sparkline`, `cmd` ("2x3", "six", and "s2" render modes)

### Bottleneck
Profiling revealed two primary bottlenecks when rendering "2x3", "six", and "s2" modes:
1. **Sextant Mask Evaluation in `v1/sextant`**: Over 90% of CPU time in `v1/sextant` was spent in `chooseBestCell`/`scoreMask`. For every 2x3 cell, `scoreMask` was called for all 64 masks, re-iterating over pixels to sum colors, performing divisions, and calling `rgbaDist2` distance functions per pixel per mask. Furthermore, `sampleBlock` invoked `splitAxis` division logic and per-pixel interface assertions.
2. **Candidate Search in `v1/sparkline`**: `findBestCandidateFast` spent significant time iterating candidate sets for cells that were completely transparent or already matched candidate #1 with zero error. `*image.NRGBA` images lacked a fastpath slice sampling loop in `findBestCandidateFast`.

### Fix
1. **Closed-Form RGB Squared Error & Mask Index LUT in `v1/sextant`**:
   - Precomputed `maskBitIndices[64]` lookup table for 6-bit sextant masks.
   - Summed total pixel colors, squared sums, opaque mask, and transparent mask ONCE per cell block.
   - Refactored `chooseBestCell` under `core.Fastpath` to compute closed-form RGB squared error:
     $\text{err} = \text{totalSqSum} - 2(\text{fg.R} \cdot \text{fgSumR} + \dots) + \text{fgN} \cdot (\text{fg.R}^2 + \dots)$.
   - Added direct 2x3 pixel slice sampling in `sampleBlock` for `*image.RGBA` and `*image.NRGBA` images.
2. **Early Exits & NRGBA Fastpath in `v1/sparkline`**:
   - Added an immediate early exit for all-transparent cell blocks (`opaque0 == 0 && opaque1 == 0`).
   - Added an early loop break when a candidate achieves `errInt == 0 && fgTrans == 0 && splitPenalty == 0` (perfect reconstruction).
   - Added an `*image.NRGBA` direct slice sampling fastpath in `findBestCandidateFast`.

---

## Hardware Target

- Target System: Multi-core x86_64 / AMD Zen 4+ (4–10 Workers).
- Execution Strategy: Eliminate per-mask pixel loops and unnecessary candidate iterations in block solvers.

---

## Benchmark Evidence

Measured with `go test -bench=. -benchmem ./v1/sextant` and `go test -bench=BenchmarkFastpathS2vsSix -run=^$ ./cmd`:

```
pkg: ubunatic.com/cati/v1/sextant
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz

                                Baseline (Before)      Optimized (After)      Improvement
BenchmarkChooseCell-4                  7,833 ns/op            5,955 ns/op     24.0% faster
BenchmarkRenderSextant-4             55.05 ms/op            32.98 ms/op     40.0% faster
BenchmarkRenderToImageSextant-4      48.29 ms/op            33.52 ms/op     30.6% faster

pkg: ubunatic.com/cati/cmd
BenchmarkFastpathS2vsSix/six_std-4    57.09 ms/op            51.56 ms/op      9.6% faster
BenchmarkFastpathS2vsSix/s2_fastpath 95.79 ms/op            69.63 ms/op     27.3% faster
```

### Summary of Wins
- **Native 2x3 Sextant Rendering (`BenchmarkRenderSextant`)**: Render latency reduced from **55.0 ms/op down to 33.0 ms/op (40% faster)**.
- **S2 Fastpath (`s2_fastpath`)**: Render latency reduced from **95.8 ms/op down to 69.6 ms/op (27.3% faster)**.
- **Six Mode (`six_std`)**: Render latency reduced from **57.1 ms/op down to 51.6 ms/op (9.6% faster)**.
