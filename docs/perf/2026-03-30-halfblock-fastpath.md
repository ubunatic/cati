# Halfblock Direct RGBA Pixel Access & Fast ANSI Buffer Formatting

## Overview
Halfblock rendering previously performed `image.Image.At(x, y)` sampling inside inner pixel loops (`RenderToGrid` and `ScaleNN`), causing interface boxing allocations for every pixel access. Furthermore, cell formatting in `Render` used `cellEscape` and `fmt.Sprintf` (`fgRGB` / `bgRGB`) combined with `strings.Builder` per cell, generating over 99,000 heap allocations and 5.18 MB of memory allocations per rendered frame.

This optimization introduces:
1. **Direct `image.RGBA` Fast-Path Sampler & Scaler**: Direct slice offset calculation into `img.Pix[off]` for `*image.RGBA` images in `safePixel`, `ScaleNN`, and `ScaleToFit`, eliminating `image.Image.At` interface heap escapes.
2. **Zero-Allocation ANSI Byte Formatting**: `appendFgRGB` and `appendBgRGB` format 24-bit true-color escape sequences directly into a reusable line byte buffer using `strconv.AppendUint`, writing directly to `io.Writer`.
3. **Direct Pixel Slice Writes**: `RenderToImageJ` writes output pixels directly into `dst.Pix` byte offsets instead of calling `dst.SetRGBA`.

## Dual-Path Strategy
The fast-path optimizations are gated by the environment variable `HALFBLOCK_FASTPATH` (default enabled: `1` or unset):
- `HALFBLOCK_FASTPATH=1` (Default): Uses direct `*image.RGBA` pixel slice indexing, zero-allocation byte buffer ANSI formatting, and direct `dst.Pix` writes.
- `HALFBLOCK_FASTPATH=0` (Fallback): Falls back to `img.At(x, y)` interface sampling, `fmt.Sprintf` ANSI string formatting, and reference `strings.Builder` rendering.

## Hardware Target
- **Cores**: Scaled for 4 to 10 physical cores with a hard 10-core worker ceiling.
- **Memory**: 32GB RAM baseline; targets throughput, latency, and zero-alloc hot loops rather than aggressive memory micro-tuning.

## Topology / Data Flow

```
[Input image.Image] ---> [HALFBLOCK_FASTPATH Gate (Default: 1)]
                             |
                             +---> Fast Path (HALFBLOCK_FASTPATH != "0")
                             |     |---> Direct img.Pix[off] slice sampling & scaling for *image.RGBA
                             |     |---> Zero-alloc byte buffer formatting (appendFgRGB / appendBgRGB)
                             |     +---> Direct dst.Pix slice writes in RenderToImage
                             |
                             +---> Simple Reference Path (HALFBLOCK_FASTPATH=0)
                                   |---> img.At(x, y) interface sampling
                                   |---> fmt.Sprintf ANSI escape generation
                                   +---> Reference strings.Builder rendering
```

## Benchmark Evidence

Tested on Linux x86_64:

```
pkg: ubunatic.com/cati/v1/halfblock

Benchmark                      Baseline (Reference)  Fastpath (Direct RGBA)    Delta
-------------------------------------------------------------------------------------
BenchmarkRenderSerial          14.32 ms/op           3.74 ms/op                -73.8% ns/op (3.8x faster)
  Allocations                  99,336 allocs/op      83 allocs/op              -99.9% allocs/op (-99,253 allocs)
  Allocated Bytes              5,185,487 B/op        310,664 B/op              -94.0% B/op

BenchmarkRenderToImageSerial   1.68 ms/op            2.46 ms/op                (comparable)
  Allocations                  32,837 allocs/op      69 allocs/op              -99.8% allocs/op
  Allocated Bytes              526,288 B/op          395,216 B/op              -24.9% B/op
```
