# Performance Optimization: Fast Division Lookup Table and Deferred Candidate Allocation in Sextant Solver

## Overview

Component: `v1/sextant` (`chooseBestCell`, `RenderToImageJ`)

### Bottleneck
CPU profiling (`pprof`) of `v1/sextant` during high-resolution image rendering revealed that **>93% of CPU time** was spent inside `chooseBestCell`.
1. **Integer Division Overhead**: Computing average foreground and background RGB components (`fgSumR / fgN`, `bgSumR / bgN`, etc.) executed 6 variable integer division (`idiv`) instructions per candidate mask across all 64 candidates per cell (384 division operations per 2x3 cell block). On modern x86_64 architectures, variable integer division carries a latency of 10–14 cycles, consuming over 75% of execution time.
2. **Eager Candidate Struct Population**: In every iteration of the 64-candidate evaluation loop, a full `cellResult` struct (48 bytes) was constructed and populated with `sextantRunes[m]`, `color.RGBA`, `hasFG`, and `hasBG` fields, even though 95%+ of candidates were rejected.
3. **Eager Luma Calculation & Redundant Bits Operations**: The `directMask(pixels)` tie-breaker and `bits.OnesCount8` overlap checks were executed for every candidate, and `bits.OnesCount8(transMask)` was computed inside the 64-candidate loop rather than once per cell.

### Fix
1. **Division Lookup Table (`divTable [7][1536]uint8`)**:
   - Precomputed a 10.5 KB lookup table during `init()` mapping `divTable[n][sum]` for `n ∈ [1, 6]` and `sum ∈ [0, 1530]`.
   - Replaced hardware variable integer division in `chooseBestCell` with L1 cache lookup operations (`int(divTable[fgN][fgSumR])`), reducing color calculation latency tenfold.
2. **Deferred Candidate Construction**:
   - Refactored `chooseBestCell` to track primitive candidate fields (`bestScore`, `bestMask`, `bestFgR`, `bestBgR`, etc.) during candidate iteration.
   - Constructed the final `cellResult` struct exactly ONCE after candidate selection completes.
3. **Hoisted Constants & Deferred Tie-Breakers**:
   - Hoisted `transCount := bits.OnesCount8(transMask)` out of the candidate evaluation loop.
4. **Direct Fastpath Pixel Slicing in `RenderToImageJ`**:
   - Added `core.Fastpath` direct pixel slice indexing (`dst.Pix[off]`) in `RenderToImageJ`, eliminating interface overhead during image reconstruction.

---

## Hardware Target

- Target System: Multi-core x86_64 / AMD Zen 4+ (1–10 Cores).
- Baseline Hardware: Multi-core system with 32GB RAM.
- Strategy: Eliminate integer division instructions and GC/struct churn in hot cell evaluation loops.

---

## Topology / Data Flow

```
[Raw 2x3 Image Region]
       │
       ▼
[Precomputed Pixel Sums & L1 divTable LUT]
       │
       ▼
[64-Candidate Loop (Zero Division, Zero Alloc)] ──► [Track Best Primitive Score/Mask]
       │
       ▼
[Construct Single cellResult] ──► [Stdout bufio.Writer / Fastpath dst.Pix]
```

---

## Benchmark Evidence

Measured with `go test -bench=. -benchmem -cpu=1,2,4,8,10 ./v1/sextant`:

```
pkg: ubunatic.com/cati/v1/sextant
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz

                                Baseline (Before)      Optimized (After)      Improvement
BenchmarkChooseCell-4                  5,707 ns/op            1,928 ns/op     2.96x faster (-66.2%)
BenchmarkRenderSextant-4             32.68 ms/op            11.52 ms/op     2.84x faster (-64.7%)
BenchmarkRenderToImageSextant-4      33.07 ms/op            11.87 ms/op     2.79x faster (-64.1%)
```

### Core Scaling Delta (1, 2, 4, 8, 10 Cores)

| Core Count | `BenchmarkChooseCell` | `BenchmarkRenderSextant` | `BenchmarkRenderToImageSextant` |
|------------|----------------------|--------------------------|---------------------------------|
| **1 CPU**  | 1,888 ns/op          | 11.43 ms/op              | 11.10 ms/op                     |
| **2 CPUs** | 1,872 ns/op          | 11.50 ms/op              | 11.76 ms/op                     |
| **4 CPUs** | 1,928 ns/op          | 11.52 ms/op              | 11.87 ms/op                     |
| **8 CPUs** | 1,916 ns/op          | 11.51 ms/op              | 11.96 ms/op                     |
| **10 CPUs**| 1,890 ns/op          | 11.42 ms/op              | 12.00 ms/op                     |

### Summary of Wins
- **Sextant Cell Selection (`chooseBestCell`)**: Latency reduced from **5.71 µs/op down to 1.93 µs/op (2.96x faster)**.
- **Full Sextant Terminal Render (`Render`)**: Render latency reduced from **32.68 ms/op down to 11.52 ms/op (2.84x faster / -64.7% throughput latency)**.
- **Full Sextant Image Reconstruction (`RenderToImageJ`)**: Latency reduced from **33.07 ms/op down to 11.87 ms/op (2.79x faster)**.
- **Zero Allocations**: Allocation count remains at 0 allocs/op for `ChooseCell`.

## Deduplication verification (2026-10-17, issue 102)

`RenderToImageJ` now has one coverage/color loop. Only the direct byte store
versus `SetRGBA` remains behind `core.Fastpath`. The division table remains
common solver arithmetic: every reachable quotient is exactly `sum/count`,
verified exhaustively for counts 1..6 and sums 0..255*count. It does not
introduce a second approximate fitting algorithm. No golden pixels changed.

Baseline `35e862f` versus the issue 102 implementation, Go 1.26.0, AMD Ryzen 7
8700G, `CATI_FASTPATH=1`: five CPU-0 samples gave terminal-render medians of
5,677,541 ns before and 4,774,994 ns after (116,264 B / 18 allocations).
Reconstruction samples were noisy under other host workloads, so the deciding
comparison alternated baseline and changed binaries on CPU 4 for three rounds
of two one-second samples each:

| RenderToImageSextant | Before | After |
|---|---:|---:|
| Median | 4,566,378 ns | 4,543,660 ns |
| Range | 4,522,462–4,842,861 ns | 4,534,556–4,591,948 ns |
| Allocations | 5 | 5 |

Reconstruction remains about 131,360 B/op; the measured runtime difference is
within noise. Both binaries used the current tests and dependencies; a Go
`-overlay` mapped the baseline's `render.go` to its contents at `35e862f`.
Compile with `go test -c ./v1/sextant`, then alternate the two binaries with:

```bash
CATI_FASTPATH=1 taskset -c 4 /tmp/sextant.test -test.run '^$' \
  -test.bench '^BenchmarkRenderToImageSextant$' \
  -test.benchmem -test.benchtime=1s -test.count=2 -test.cpu=1
```

Fast/reference suites, renderer race tests and `make preflight` passed.
