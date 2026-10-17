# Fast Reciprocal Division and Symmetric Bit Iteration in Sparkline Solver

## Overview

In the sparkline renderer (`v1/sparkline`), candidate selection in `findBestCandidateFast` evaluates candidate shape masks (e.g., 62-100+ glyph candidates per cell) to find the optimal character and color combination. Profiling revealed two primary CPU bottlenecks:
1. **Division Instruction Latency**: Performing variable integer division (`fgSum / fgN` and `bgSum / bgN`) per candidate invoked up to 600 `idiv` x86_64 CPU instructions per cell evaluation.
2. **Asymmetric Bit Loop Overhead**: Iterating over all set bits in `fgOpaque` regardless of whether `fgN` or `bgN` had fewer set bits, causing unnecessary bit extraction loops when `fgN` comprised most of the cell.

To resolve these bottlenecks:
- Precomputed fixed-point division multipliers (`divMult [129]uint64`) replace `idiv` instructions with 1-cycle `imul` arithmetic (`(sum * divMult[count]) >> 32`), producing 100% exact integer division results.
- Symmetric bit iteration selects `min(fgN, bgN)` to minimize trailing-zero bit extraction loops, computing the complementary color sum via `totalSum - partSum`.

## Hardware Target

- **CPU**: Multi-Core x86_64 / Zen 4+ (AVX-512 capable), 4–10 core execution bounds.
- **Cache & Registers**: Fixed 128-element lookup tables fit entirely in L1 data cache (1 KB table size).

## Topology / Data Flow

```
[Cell RGBA Pixels] ---> [Compute totalSumR/G/B & totalSqSum]
                                   |
                                   v
             +-------------------------------------------+
             | Loop Candidate Bitmasks (bitCands)         |
             |                                           |
             |  1. fgN <= bgN ?                          |
             |     - Sum over fgOpaque (min set bits)    |
             |     - bgSum = totalSum - fgSum            |
             |  2. fgN > bgN ?                           |
             |     - Sum over bgOpaque (min set bits)    |
             |     - fgSum = totalSum - bgSum            |
             |                                           |
             |  3. Fast Division:                        |
             |     avg = (sum * divMult[N]) >> 32        |
             +-------------------------------------------+
                                   |
                                   v
                     [Optimal Cell Result (Ch, FG, BG)]
```

## Benchmark Evidence

Tested on x86_64 Linux with Go 1.26:

```
Benchmark                           | Before (ns/op)  | After (ns/op)   | Delta (%)
--------------------------------------------------------------------------------
BenchmarkFindBestCellBest           | 7026454.2       | 4117107.4       | -41.41%
BenchmarkFindBestCellSixHalf        | 6082642.4       | 3640521.8       | -40.15%
BenchmarkFindBestCellQuad           | 2470228.2       | 1539457.8       | -37.68%
BenchmarkFindBestCellSextant        | 5376071.2       | 3494539.8       | -35.00%
BenchmarkFindBestCellSpark          | 1650875.0       | 1083848.6       | -34.35%
BenchmarkFindBestCellVertical       | 954946.0        | 649051.0        | -32.03%
BenchmarkFindBestCellHalfSplit      | 742838.2        | 588671.2        | -20.75%
BenchmarkRenderCustomShapes         | 6824797.4       | 5368281.2       | -21.34%
BenchmarkRenderToImageSerial        | 1555801.8       | 1242931.4       | -20.11%
BenchmarkRenderSerial               | 1090265.4       | 938631.4        | -13.91%
```

All benchmarks maintain 0 B/op and 0 allocs/op in hot candidate selection routines.
