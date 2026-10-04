# ⚡ Performance Report: Halfblock Direct RGBA Fast-Path & Zero-Allocation Buffer

## Overview
- **Component**: [`v1/halfblock`](https://codeberg.org/ubunatic/cati/src/branch/main/v1/halfblock/render.go) terminal renderer and scaler.
- **Bottleneck**: In the reference/simple path (`CATI_FASTPATH=0`), pixel sampling relies on generic `img.At(x, y)` calls, which trigger interface boxing of `color.Color` on every pixel and cause tens of thousands of heap allocations per frame. Furthermore, ANSI escape sequences were constructed with `fmt.Sprintf` (`\x1b[38;2;%d;%d;%dm`) and `strings.Builder`, allocating strings for every cell and row.
- **Fix**:
  1. **Direct `*image.RGBA` Pixel Slice Indexing**: In `safePixel`, `ScaleToFit`, and `ScaleNN`, type-assert `*image.RGBA` to index `img.Pix` directly and use 4-byte `copy()` slices, eliminating `img.At` interface dispatch and heap allocation.
  2. **Zero-Allocation ANSI Line Buffer**: Reusable byte slice per line (`buf = buf[:0]`) with `appendFgRGB` and `appendBgRGB` formatting escape sequences directly into the buffer via `strconv.AppendUint`. Runes are encoded directly into the slice using `utf8.EncodeRune`.
  3. **Direct Destination Rasterization**: In `RenderToImageJ`, calculate destination row strides directly and assign bytes to `dst.Pix` without repeated `dst.SetRGBA` bounds checking.

## Dual-Path Architecture
All fast paths are gated by [`core.Fastpath`](https://codeberg.org/ubunatic/cati/src/branch/main/v1/core/fastpath.go), initialized from `CATI_FASTPATH`:
- `CATI_FASTPATH=1` (Default): Direct RGBA access, zero-alloc byte buffer formatting, direct raster writes.
- `CATI_FASTPATH=0` (Fallback): Simple reference path (`img.At`, `fmt.Sprintf`, `strings.Builder`, `dst.SetRGBA`).

Parity is enforced by [`TestFastpathParity`](https://codeberg.org/ubunatic/cati/src/branch/main/v1/halfblock/render_test.go) ensuring byte-for-byte identical output between both paths.

## Topology / Data Flow

```
[Input image.Image] ---> [CATI_FASTPATH Gate (Default: 1)]
                             │
                             ├───> Fast Path (CATI_FASTPATH != "0")
                             │     ├───> Direct *image.RGBA.Pix slice offset indexing
                             │     ├───> Reusable byte slice formatting (appendFgRGB / appendBgRGB via strconv.AppendUint)
                             │     └───> Direct dst.Pix rasterization in RenderToImageJ
                             │
                             └───> Simple Reference Path (CATI_FASTPATH=0)
                                   ├───> img.At(x, y) interface sampling (escapes to heap)
                                   ├───> fmt.Sprintf ANSI escape generation
                                   └───> strings.Builder row assembly & dst.SetRGBA
```

## Benchmark Evidence

Tested on Linux x86_64 (`AMD Ryzen 5 PRO 5650U`):

```
pkg: ubunatic.com/cati/v1/halfblock

Benchmark                      Baseline (CATI_FASTPATH=0)    Fastpath (CATI_FASTPATH=1)    Delta
------------------------------------------------------------------------------------------------
BenchmarkRenderSerial          7.64 ms/op                    1.28 ms/op                    ~6.0x faster (-83.2% ns/op)
  Allocations                  99,339 allocs/op              83 allocs/op                  -99.9% allocs/op
  Allocated Bytes              5,200,358 B/op                310,664 B/op                  -94.0% B/op

BenchmarkRenderToImageSerial   1.10 ms/op                    0.59 ms/op                    ~1.9x faster (-46.1% ns/op)
  Allocations                  32,837 allocs/op              69 allocs/op                  -99.8% allocs/op
  Allocated Bytes              526,289 B/op                  395,217 B/op                  -24.9% B/op
```
