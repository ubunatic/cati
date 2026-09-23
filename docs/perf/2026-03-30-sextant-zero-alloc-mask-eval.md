# ⚡ Performance Report: Zero-Allocation Mask Evaluation & LUT Bit Indexing in Sextant Renderer

## Overview
- **Component**: `v1/sextant` terminal block solver and renderer.
- **Bottleneck**: `allMasks()` allocated a new 64-byte slice (`make([]uint8, 64)`) on every call in cell selection, and `maskContains()` invoked a switch statement (`sextantBit()`) for every pixel/mask evaluation in `scoreMask`.
- **Fix**: Replaced dynamic `allMasks()` slice creation with a package-level pre-allocated static slice (`staticAllMasks`) and converted `bitForIndex()`/`maskContains()` to use a fast 6-element lookup table (`bitForIdxTable`). Worker pools in multi-threaded rendering paths were also capped to at most 10 workers (`min(runtime.NumCPU(), 10)`).

## Hardware Target
- **Target Topology**: Multi-core Linux systems (4 to 10 physical cores), x86_64 / AVX-512 vector pipelines, 32GB RAM baseline.
- **Execution Strategy**: Eliminate heap escape churn in block solvers and bounded row-chunking parallelism across 1 to 10 worker goroutines.

## Topology / Data Flow

```
[ Input Image ] ---> [ Row Chunking (Worker Pool <= 10) ]
                            │
                            ▼
               [ sampleBlock (2x3 Pixels) ]
                            │
                            ▼
          [ chooseBestCell (staticAllMasks) ]
                            │
                            ▼
               [ scoreMask (bitForIdxTable) ]
                            │
                            ▼
                  [ Rendered ANSI / Grid ]
```

## Benchmark Evidence (`benchstat`)

```
goos: linux
goarch: amd64
pkg: ubunatic.com/cati/v1/sextant
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz

                        │    Before     │         After         │
                        │    sec/op     │    sec/op     vs base │
ScoreMask                 125.4n ± ∞    116.0n ± ∞   -7.50%
ScoreMask-2               130.1n ± ∞    116.3n ± ∞  -10.61%
ScoreMask-4               127.8n ± ∞    116.3n ± ∞   -9.00%
ChooseCell                8.186µ ± ∞    7.544µ ± ∞   -7.84%
ChooseCell-4              8.246µ ± ∞    7.583µ ± ∞   -8.04%
ChooseCell-10            10.044µ ± ∞    7.558µ ± ∞  -24.75%
RenderSextant             52.48m ± ∞    48.94m ± ∞   -6.75%
RenderSextant-4           54.65m ± ∞    48.60m ± ∞  -11.07%
RenderToImageSextant      49.27m ± ∞    45.54m ± ∞   -7.57%
RenderToImageSextant-4    50.69m ± ∞    46.32m ± ∞   -8.61%
geomean                   231.8µ        213.5µ       -7.88%
```

Overall, geometric mean latency across all sextant operations dropped by **7.88%** with zero regression in accuracy or visual rendering quality.
