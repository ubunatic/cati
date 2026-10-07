# Sextant Solver: Precomputed Subset Sums, Division LUT, and Uniform Fastpath

## Overview
In `v1/sextant`, evaluating 64 sextant glyph candidate masks per 2x3 block consumed >90% of total rendering time. Profiling identified two primary hot-loop bottlenecks in `chooseBestCell`:
1. **CPU Division Churn (384 IDIVs/cell):** Each mask evaluation computed foreground and background color averages using integer division (`fgSum / fgN` and `bgSum / bgN`), executing 6 division instructions per mask (384 IDIVs per cell).
2. **Bit Indexing Loop Overhead:** Extracting set bit indices per mask via `maskBitIndices[fgOpaque]` added repeated array lookup and loop control overhead in the inner 64-mask candidate search loop.

The fix introduces three optimizations:
- **Precomputed Division Lookup Table (`divTable[7][1531]uint8`):** A small 10.7 KB lookup table populated at `init()` replaces all integer divisions (`divTable[n][sum]`).
- **Cell-Level Subset Sum Precomputation (`sumR[64]`, `sumG[64]`, `sumB[64]`):** Color sums for all 64 possible bitmask combinations are precomputed in 6 iterations prior to entering the candidate loop, eliminating inner set-bit iteration.
- **Uniform Opaque Cell Fastpath & Score-0 Early Exit:** Fully opaque, single-color cells return immediately without executing the 64-mask loop, and the candidate loop exits early when a zero-error match with maximum overlap is found.

## Hardware Target
- **Target OS / Architecture:** Modern Linux (x86_64), AMD Zen 4+ / Zen 5, Intel Core/Xeon.
- **Worker Cap:** Capped at 10 cores (`min(runtime.NumCPU(), 10)`).
- **Cache Topology:** 10.7 KB division LUT fits entirely within CPU L1 data cache (32KB/64KB L1d per core).

## Topology / Data Flow

```
[2x3 Pixel Sample Block]
       |
       v
[Uniform Opaque Check] ----(If Solid)----> [Return Space + BG Cell]
       |
  (If Variable)
       v
[Precompute sumR/G/B[64]]
       |
       v
[64-Candidate Mask Loop]
  ├─> [fgSum = sumR[fgOpaque]]         (Direct Array Lookup)
  ├─> [fgR = divTable[fgN][fgSumR]]    (L1 Cache Table Access - Zero IDIV)
  └─> [Score Evaluation & 0-Exit]
       |
       v
[Best Cell Result]
```

## Benchmark Evidence

### BenchmarkRenderSextant (256x128 image)
```
benchmark                       old ns/op     new ns/op     delta
BenchmarkRenderSextant-1        32934551      10739527      -67.39%
BenchmarkRenderSextant-2        33304412      10761272      -67.69%
BenchmarkRenderSextant-4        33054294      10607710      -67.91%
BenchmarkRenderSextant-8        32885188      10620230      -67.70%
BenchmarkRenderSextant-10       32770918      10726794      -67.27%
```

### BenchmarkChooseCell (Individual Cell Selection)
```
benchmark                       old ns/op     new ns/op     delta
BenchmarkChooseCell             6361 ns/op    1717 ns/op    -73.01%
```

Both allocations (60 allocs/op) and memory usage (114.2 KB/op) remain zero/minimal in the hot path.
