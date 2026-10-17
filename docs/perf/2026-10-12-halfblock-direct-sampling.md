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

## Deduplication verification (2026-10-17, issue 102)

Grid construction shares the `cell.gridCell` conversion. RGBA and NRGBA byte
readers are reused by grid construction and direct reconstruction, which now
shares `storeOpaquePixel`. The row loops retain concrete image-type selection;
small inline helpers avoid indirect calls per pixel. Reconstruction preserves
premultiplied RGB, forces painted alpha to 255, and clears transparent pixels.
Odd-height, nonzero-origin subimages and a source-color oracle cover this
invariant in serial and parallel execution. No golden changes are expected.

Baseline: `35e862f`; changed: issue 102 implementation. Go 1.26.0, AMD Ryzen 7
8700G, `CATI_FASTPATH=1`, CPU affinity 0, five samples of 500 ms each. Medians:

| Benchmark | Before | After | Bytes / allocations |
|---|---:|---:|---:|
| RenderSerial | 1,068,366 ns | 832,321 ns | 310,664 / 20 |
| RenderToImageSerial | 125,528 ns | 78,446 ns | 131,216 / 3 |

Both binaries were compiled with `go test -c ./v1/halfblock`; the baseline used
`-overlay` to map `render.go` to its contents from `git show 35e862f:v1/halfblock/render.go`,
keeping tests and dependencies identical. Run each compiled binary with:

```bash
CATI_FASTPATH=1 taskset -c 0 /tmp/halfblock.test -test.run '^$' \
  -test.bench 'BenchmarkRender(Serial|ToImageSerial)$' \
  -test.benchmem -test.benchtime=500ms -test.count=5 -test.cpu=1
```

These shared-host measurements support the absence of a regression; they are
not a portable speed-up guarantee. Fast/reference suites, renderer race tests,
and `make preflight` passed without regenerating goldens.

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
