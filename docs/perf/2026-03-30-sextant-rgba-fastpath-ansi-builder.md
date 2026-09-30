# Sextant Direct RGBA Pixel Access & Zero-Allocation ANSI Buffer Formatting

## Overview
Sextant rendering previously performed `image.Image.At(x, y)` sampling inside inner region sampling loops (`avgRegion`), causing sub-block pixel accesses to escape to the heap due to `color.Color` interface boxing. Additionally, string-based ANSI escape code formatting (`fmt.Sprintf`) in `fgRGB` and `bgRGB` and `strings.Builder` line concatenation produced over 44,000 heap allocations and ~1.47 MB/op of allocation churn per frame.

This optimization introduces:
1. **Direct `image.RGBA` Fast-Path Sampler**: Direct slice offset calculation into `rgba.Pix[off]` for `*image.RGBA` images in `avgRegion`, eliminating interface heap escapes.
2. **Zero-Allocation ANSI Byte Formatting**: `appendFgRGB` and `appendBgRGB` format 24-bit true-color escape sequences directly into a reusable line byte buffer using `strconv.AppendUint`.
3. **Reusable Line Byte Buffer**: Replaces `strings.Builder` per line in `Render` with a reusable byte slice (`lineBuf`), emitting rendered lines via `w.Write(lineBuf)`.

## Hardware Target
- **Cores**: Multi-core Linux x86_64, capped at 10 workers max (`core.MaxWorkers()`).
- **Memory**: 32GB RAM baseline; targets frame latency and eliminating hot-loop GC churn.

## Topology / Data Flow

```
[Raw Input image.Image] ---> [Type Assert *image.RGBA && Fastpath]
                                   |
                                   +---> [Fast Path: Direct rgba.Pix[off] sampling]
                                   |     +---> Zero-alloc avgRegion pixel accumulation
                                   |
                                   +---> [Fallback Path: img.At(x, y)]
                                         +---> Interface color.Color sampling

[Rendered Grid] ---> [Reusable []byte Line Buffer]
                          |---> appendBgRGB / appendFgRGB via strconv.AppendUint
                          |---> utf8.AppendRune for sextant glyphs
                          +---> Direct io.Writer.Write(lineBuf)
```

## Benchmark Evidence

Tested on Linux x86_64 (`go test -bench=. -benchmem -cpu=1,2,4,8,10 ./v1/sextant`):

```
pkg: ubunatic.com/cati/v1/sextant

Benchmark                      Baseline              Optimized                 Delta
------------------------------------------------------------------------------------------------
BenchmarkRenderSextant         49.08 ms/op           45.13 ms/op               -8.0% ns/op
  Allocations                  44,679 allocs/op      49 allocs/op              -99.89% allocs/op
  Allocated Bytes              1,471,900 B/op        105,776 B/op              -92.81% B/op

BenchmarkRenderToImageSextant  45.18 ms/op           46.25 ms/op               (comparable)
  Allocations                  33,029 allocs/op      5 allocs/op               -99.98% allocs/op
  Allocated Bytes              263,464 B/op          131,371 B/op              -50.14% B/op
```
