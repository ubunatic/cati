# ⚡ Bitmask Popcount Derivation and Interleaved Pixel Solvers in Sparkline and Sextant

## Overview

During high-resolution terminal rendering with `v1/sparkline` and `v1/sextant`, candidate glyph selection (`findBestCandidateFast` and `chooseBestCell`) consumed up to 94% of CPU cycles. Profiling revealed two primary bottlenecks:
1. **Redundant Bit Counting:** Evaluating `bgN`, `fgTrans`, and `bgTrans` per candidate candidate mask invoked up to 4 separate `bits.OnesCount64` / `bits.OnesCount8` calls per candidate per cell.
2. **Disjoint Array Cache Line Accesses:** Sampled block RGB components were kept in 3 separate parallel arrays (`pr`, `pg`, `pb`), causing cache line splits when indexing pixels during bit-trailing zero loops.

By deriving `bgN = totalOpaqueN - fgN` and `bgTrans = totalTransN - fgTrans` from precomputed block popcounts, gating transparent bit counts behind `trans0 != 0`, and coalescing RGB components into interleaved contiguous structs (`rgbPixel`), candidate solver execution time was reduced significantly across all sparkline and sextant evaluation modes.

## Hardware Target

- **Core Topology:** 4 to 10 Physical Cores (SIMD / AVX-512 auto-vectorization friendly on AMD Zen 4+).
- **Memory Footprint:** Zero allocations (`0 B/op`, `0 allocs/op`). 384-byte stack buffers fitting entirely within L1 data cache.

## Topology / Data Flow

```
[Raw Pixel Block] ---> [Interleaved rgbPixel Buffer] ---> [Precomputed Block Popcounts (totalOpaqueN, totalTransN)]
                                                                                |
                                                                                v
[Candidate Mask Iteration] <--- [Derive bgN = totalOpaqueN - fgN] <--- [Single fgN Popcount]
            |
            v
 [SSE Error Evaluation] ---> [Lowest-Error Glyph Output]
```

## Benchmark Evidence

### `v1/sparkline` Candidate Solvers (`go test -bench=BenchmarkFindBestCell -benchmem ./v1/sparkline`)

| Benchmark | Baseline (`ns/op`) | Optimized (`ns/op`) | Delta | Allocations |
| :--- | :--- | :--- | :--- | :--- |
| `BenchmarkFindBestCellSpark` | 1,604,172 | 1,512,310 | **-5.7%** | `0 B/op, 0 allocs/op` |
| `BenchmarkFindBestCellQuad` | 2,508,150 | 2,306,974 | **-8.0%** | `0 B/op, 0 allocs/op` |
| `BenchmarkFindBestCellSextant` | 6,510,948 | 5,144,373 | **-21.0%** | `0 B/op, 0 allocs/op` |
| `BenchmarkFindBestCellSixHalf` | 5,842,073 | 5,468,186 | **-6.4%** | `0 B/op, 0 allocs/op` |
| `BenchmarkFindBestCellBest` | 6,656,240 | 6,386,670 | **-4.0%** | `0 B/op, 0 allocs/op` |

All unit tests and fastpath differential parity tests remain 100% green.
