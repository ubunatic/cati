# Fast Division Reciprocal Multipliers and Complementary Bit Counts in Sparkline Candidate Solvers

## Overview
In `v1/sparkline`, `findBestCandidateFast` evaluates candidate Unicode block glyph masks against pixel blocks (up to 128 subpixels per cell). Across up to 256 candidates per cell (in `Best` candidate mode), each candidate evaluation previously performed variable 32-bit integer divisions (`sum / fgN` and `sum / bgN`) to compute average foreground and background RGB color components. Variable integer division on x86_64 executes via `idivl` with 10–15 CPU cycle latency per division.

In addition, each candidate evaluation recomputed bitwise complement mask counts (`bgN` and `bgTrans`) using `bits.OnesCount64((^candM0) & opaque0)` and `bits.OnesCount64((^candM0) & trans0)`, invoking unnecessary POPCNT, bitwise NOT, and bitwise AND instructions.

To resolve these bottlenecks:
1. Precomputed 64-bit reciprocal multiplication factors (`divInv [129]uint64`) enable integer division replacement via single-cycle multiplication: `(sum * divInv[n]) >> 32 == sum / n`.
2. Total cell opaque (`opaqueCount0`) and transparent (`transCount0`) bit counts are computed once per cell outside candidate iteration loops, allowing background counts to be derived via subtraction: `bgN = opaqueCount0 - fgN` and `bgTrans = transCount0 - fgTrans`.
3. Array bounds check elimination (BCE) hints were added for pixel sample buffers in `findBestCandidateFast`.

## Hardware Target
- Target OS / CPUs: Modern Linux (Ubuntu LTS, Fedora, CachyOS), x86_64 multi-core systems (AMD Zen 4+ / AVX-512).
- CPU Focus: Eliminating high-latency `idiv` pipeline stalls and reducing POPCNT instruction volume in candidate evaluation inner loops.

## Topology / Data Flow
```
[ Subpixel RGBA Buffer [128]uint8 ]
               │
               ▼
   [ Cell Total Sum & Popcounts ]
  (opaqueCount0, transCount0 once)
               │
               ▼
┌──────────────────────────────────────────┐
│ Candidate Evaluation Loop (up to 256)    │
│ ├─ fgN = POPCNT(candM0 & opaque0)        │
│ ├─ bgN = opaqueCount0 - fgN              │
│ ├─ fgTrans = POPCNT(candM0 & trans0)     │
│ ├─ bgTrans = transCount0 - fgTrans       │
│ └─ Fast RGB Reciprocal Division:         │
│    fgAvgR = (fgSumR * divInv[fgN]) >> 32 │
│    bgAvgR = (bgSumR * divInv[bgN]) >> 32 │
└──────────────────────────────────────────┘
               │
               ▼
      [ Optimal Cell Result ]
```

## Benchmark Evidence

### `go test -bench=BenchmarkFindBestCell -benchmem ./v1/sparkline`

| Benchmark Mode | Before (ns/op) | After (ns/op) | Delta / Speedup |
| :--- | :--- | :--- | :--- |
| `BenchmarkFindBestCellVertical` | 942,427 ns/op | 790,041 ns/op | **-16.2%** (~1.19x faster) |
| `BenchmarkFindBestCellHalfSplit` | 694,659 ns/op | 615,200 ns/op | **-11.4%** (~1.13x faster) |
| `BenchmarkFindBestCellSpark` | 1,563,707 ns/op | 1,344,867 ns/op | **-14.0%** (~1.16x faster) |
| `BenchmarkFindBestCellQuad` | 2,349,125 ns/op | 1,951,764 ns/op | **-16.9%** (~1.20x faster) |
| `BenchmarkFindBestCellSextant` | 5,184,582 ns/op | 4,282,863 ns/op | **-17.4%** (~1.21x faster) |
| `BenchmarkFindBestCellSixHalf` | 5,433,888 ns/op | 4,374,213 ns/op | **-19.5%** (~1.24x faster) |
| `BenchmarkFindBestCellBest` | 6,794,165 ns/op | 5,076,531 ns/op | **-25.3%** (**1.34x faster**) |

### `go test -bench=BenchmarkRenderSerial -benchmem ./v1/sparkline`
- Before: 1,055,179 ns/op (32 allocs/op)
- After: 934,278 ns/op (32 allocs/op, **-11.5% rendering latency**)
