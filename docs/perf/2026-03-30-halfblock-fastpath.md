# Halfblock Direct RGBA Fast-Path & Reusable Byte Buffer Formatting

## Overview
Halfblock rendering previously performed `scaled.At(srcX, srcY)` pixel sampling inside cell evaluation loops (`RenderToGrid`), causing every pixel access to allocate on the heap due to `color.Color` interface boxing (32k+ allocations per image frame). Additionally, string-based ANSI escape code formatting (`fmt.Sprintf` via `fgRGB`/`bgRGB`) and per-line `strings.Builder` allocations in `Render` produced over 66k heap allocations per frame, leading to heavy GC pressure.

This optimization introduces:
1. **Direct `image.RGBA` Fast-Path Sampler**: Direct slice offset calculation into `img.Pix[off]` for `*image.RGBA` images in `safePixel` and `ScaleNN`, eliminating interface heap escapes.
2. **Zero-Allocation ANSI Byte Formatting**: `appendFgRGB` and `appendBgRGB` format 24-bit true-color escape sequences directly into a reusable line `[]byte` buffer using `strconv.AppendUint`.
3. **10-Core Ceiling Ceiling**: Parallel worker count in `RenderToGrid` capped at `min(core.MaxWorkers(), 10)`.

## Dual-Path Strategy
The fast-path optimizations are dual-path and gated by `core.Fastpath` (controlled via `CATI_FASTPATH` environment variable, default enabled):
- `CATI_FASTPATH=1` (Default): Uses direct `*image.RGBA` pixel slice indexing, zero-allocation byte buffer ANSI formatting, and fast nearest-neighbor scaling.
- `CATI_FASTPATH=0` (Fallback): Falls back to `img.At(x, y)` interface sampling, `fmt.Sprintf` ANSI string formatting, and reference `strings.Builder` line composition.

Differential parity unit tests (`TestFastpathDifferentialParity`) verify byte-for-byte ANSI output and pixel-for-pixel `RenderToImage` equivalence between both paths across `image.RGBA`, `image.NRGBA`, and `image.Gray` image types.

## Hardware Target
- **Cores**: Scaled for 4 to 10 physical cores with a hard 10-core worker ceiling.
- **Memory**: 32GB RAM baseline; targets latency and eliminating hot-loop GC churn rather than aggressive memory micro-tuning.

## Topology / Data Flow

```
[Input image.Image] ---> [CATI_FASTPATH Gate (Default: Fastpath)]
                             |
                             +---> Fast Path (CATI_FASTPATH != "0")
                             |     |---> Direct img.Pix[off] slice sampling in safePixel and ScaleNN
                             |     |---> Reusable line []byte buffer formatting (appendFgRGB / appendBgRGB)
                             |     +---> Bounded worker pool (capped at <= 10 cores)
                             |
                             +---> Simple Reference Path (CATI_FASTPATH=0)
                                   |---> img.At(x, y) interface sampling
                                   |---> fmt.Sprintf ANSI escape generation
                                   +---> Reference strings.Builder rendering
```

## Benchmark Evidence

Tested on Linux x86_64:

```
pkg: ubunatic.com/cati/v1/halfblock

Benchmark                      Baseline (Simple)     Fastpath (Direct RGBA)    Delta
-------------------------------------------------------------------------------------
BenchmarkRenderSerial          12.92 ms/op           2.14 ms/op                -83.4% ns/op (6.0x faster)
  Allocations                  99,339 allocs/op      83 allocs/op              -99.9% allocs/op
  Allocated Bytes              5,196,690 B/op        310,665 B/op              -94.0% B/op

BenchmarkRenderToImageSerial   1.67 ms/op            0.98 ms/op                -41.3% ns/op
  Allocations                  32,837 allocs/op      69 allocs/op              -99.8% allocs/op
  Allocated Bytes              526,290 B/op          395,216 B/op              -24.9% B/op
```
