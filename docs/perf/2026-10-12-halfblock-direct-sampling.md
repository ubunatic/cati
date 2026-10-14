# Performance Optimization: Direct Pixel Sampling and Zero-Grid Image Reconstruction in Halfblock Renderer

**Date:** 2026-10-12
**Component:** `v1/halfblock`, `v1/quadblock`, `v1/sextant`

## Overview
In high-throughput terminal media rendering, halfblock cell construction (`RenderToGrid`) and image reconstruction (`RenderToImageJ`) serve as core building blocks for video streaming and terminal UI previews.

Prior to this optimization:
1. `RenderToGrid` iteratively allocated individual row slices (`make([]core.Cell, width)`) in a loop, generating tens of heap allocations per frame, and evaluated per-pixel type assertions (`scaled.(*image.RGBA)`) on every pixel coordinate.
2. `RenderToImageJ` constructed an intermediate `*core.Grid` struct matrix before translating `core.Cell` properties back into pixel bytes in `dst.Pix`, creating unnecessary struct allocations and memory passes.
3. 4-byte `copy(dst.Pix[dOff:dOff+4], srcRGBA.Pix[sOff:sOff+4])` calls inside scaling loops (`ScaleToFit`/`ScaleNN`) invoked runtime `memmove` overhead.

**The Fix:**
- Pre-allocate a single contiguous slice `cellStorage := make([]core.Cell, rowCount*width)` and slice contiguous row views across `RenderToGrid` in `v1/halfblock`, `v1/quadblock`, and `v1/sextant`.
- Add direct row-slice fastpaths in `RenderToGrid` for `*image.RGBA` and `*image.NRGBA` image types.
- Bypass intermediate `*core.Grid` allocation in `RenderToImageJ` under `core.Fastpath` by sampling source pixel slices directly and emitting 24-bit RGBA bytes straight into `dst.Pix`.
- Replace 4-byte `copy(...)` in `ScaleToFit`/`ScaleNN` with direct element assignments guarded by upfront slice bounds checks.

## Hardware Target
- Architecture: x86_64 / Zen 4+ APU / Multi-Core Linux
- Worker Ceiling: Capped at 10 cores (`min(runtime.NumCPU(), 10)`)
- Focus: Throughput, frame rates, and zero hot-loop allocation churn.

## Topology / Data Flow

```
[Raw Image (RGBA / NRGBA)]
           |
           +---> Fastpath Type Assertion (Once per row)
           |
           +---> [Direct Row Sampling (Stride Offset)]
           |               |
           |               +---> RenderToImageJ: Direct dst.Pix Writes (Zero Grid)
           |               |
           v               v
  [Contiguous Cell Storage] ---> [bufio / Terminal Writer]
```

## Benchmark Evidence

Benchmarks performed on x86_64 host across 1 to 10 CPU cores:

```
pkg: ubunatic.com/cati/v1/halfblock

Benchmark                       Before                 After                  Delta
BenchmarkRenderToImageSerial    1,019,976 ns/op        233,617 ns/op          -77.10% (4.36x faster)
  Bytes/op                      395,217 B/op           131,216 B/op           -66.80%
  Allocs/op                     69 allocs/op           3 allocs/op            -95.65%

BenchmarkRenderSerial           2,214,470 ns/op        1,991,321 ns/op        -10.08%
  Bytes/op                      310,665 B/op           310,665 B/op           0.00%
  Allocs/op                     83 allocs/op           20 allocs/op           -75.90%

pkg: ubunatic.com/cati/v1/quadblock
BenchmarkRenderSerial           72 allocs/op           9 allocs/op            -87.50%

pkg: ubunatic.com/cati/v1/sextant
BenchmarkRenderSextant          60 allocs/op           18 allocs/op           -70.00%
```
