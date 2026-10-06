# Lock-Free Wavefront Scheduling and Direct Subpixel Fastpath in Quadblock

## Overview

Quadblock cell compilation requires contextual color information from adjacent cells (`leftCell` and `aboveCell`) when evaluating candidate color pairs or split-half neighbor candidates.
Previously, parallel quadblock rendering enforced cell-dependency ordering by launching workers through fine-grained channels for each anti-diagonal grid line, followed by a `sync.WaitGroup` barrier (`wg.Wait()`) per diagonal.
For a 160×67 cell grid, this executed 226 barrier synchronizations and thousands of channel operations per frame, creating CPU scheduler contention and ruining cache locality.

This optimization replaces the per-diagonal channel barriers with a lock-free wavefront scheduler using atomic row progress tracking (`atomic.Int32`).
Each row worker processes cells sequentially left-to-right (automatically satisfying `leftCell` dependencies), and waits on `rowProgress[tr-1] >= tc + 1` before evaluating cell `(tr, tc)`.
In addition, direct 2×2 subpixel sampling was added for `*image.RGBA` and `*image.NRGBA` images in `computeQuadCell`, eliminating closure allocation and function call overhead, and direct slice writes were implemented in `RenderToImageJ`.

## Hardware Target

- Architecture: Modern Multi-Core x86_64 (4 to 10 cores baseline, Zen 4 / AVX-512)
- Topology: Lock-free atomic progress array (`[]atomic.Int32`) with L1/L2-cache-friendly contiguous row indexing.

## Topology / Data Flow

```
[Raw Image (RGBA / NRGBA)]
         │
         ▼ (Direct 2x2 Subpixel Fastpath)
┌─────────────────────────────────────────────────────────────┐
│ Lock-Free Wavefront Scheduler (Worker Pool <= 10 Cores)    │
│                                                             │
│ Worker 0: Row 0 ─────────────────────────────> Store Prog   │
│               │ (rowProgress[0] >= tc+1)                    │
│               ▼                                             │
│ Worker 1: Row 1 ─────────────────────────────> Store Prog   │
│               │ (rowProgress[1] >= tc+1)                    │
│               ▼                                             │
│ Worker 2: Row 2 ─────────────────────────────> Store Prog   │
└─────────────────────────────────────────────────────────────┘
         │
         ▼ (Direct Pixel Slice Writes)
[Output Grid / image.RGBA]
```

## Benchmark Evidence

Measured with `go test -bench=. -benchmem -cpu=1,2,4,8,10 ./v1/quadblock`:

```
BenchmarkRenderSerial                296  3959345 ns/op  411504 B/op  72 allocs/op
BenchmarkRenderParallel-2            364  3229787 ns/op  412733 B/op  79 allocs/op
BenchmarkRenderParallel-4            452  2577540 ns/op  412715 B/op  79 allocs/op
BenchmarkRenderParallel-8            440  2683160 ns/op  412780 B/op  79 allocs/op

BenchmarkRenderToImageSerial         194  6160335 ns/op  524352 B/op   3 allocs/op
BenchmarkRenderToImageParallel-2     254  4678907 ns/op  525782 B/op  10 allocs/op
BenchmarkRenderToImageParallel-4     351  3253809 ns/op  525794 B/op  10 allocs/op
BenchmarkRenderToImageParallel-8     320  3468247 ns/op  525790 B/op  10 allocs/op
```

- **Render Parallel Throughput Gain:** ~35% faster execution time at 4 cores (2.57 ms/op vs 3.96 ms/op).
- **RenderToImage Throughput Gain:** ~47% faster execution time at 4 cores (3.25 ms/op vs 6.16 ms/op).
