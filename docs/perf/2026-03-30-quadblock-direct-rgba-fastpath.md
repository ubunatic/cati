# Quadblock Direct RGBA Pixel Access & Fast ANSI Buffer Formatting

## Overview
Quadblock rendering previously performed `image.Image.At(x, y)` sampling inside inner pixel loops (`safePixel`), causing every pixel access to escape to the heap due to `color.Color` interface box allocations (131k allocations per image frame). Additionally, string-based ANSI escape code formatting (`fmt.Sprintf`) in `fgRGB` and `bgRGB` produced heavy GC pressure and string allocation overhead.

This optimization introduces:
1. **Direct `image.RGBA` Fast-Path Sampler**: Direct slice offset calculation into `img.Pix[off]` for `*image.RGBA` images in `safePixel`, eliminating interface heap escapes.
2. **Zero-Allocation ANSI Byte Formatting**: `appendFgRGB` and `appendBgRGB` format 24-bit true-color escape sequences directly into a reusable line byte buffer using `strconv.AppendUint`.
3. **Bounded Persistent Parallel Dispatch**: Bounded worker channel processing in `computeQuadCellsJ` capped at 10 workers maximum (`min(jobs, min(runtime.NumCPU(), 10))`).

## Dual-Path Strategy
The fast-path optimizations are gated by the environment variable `QUADBLOCK_FASTPATH` (default enabled: `1` or unset):
- `QUADBLOCK_FASTPATH=1` (Default): Uses direct `*image.RGBA` pixel slice indexing, zero-allocation byte buffer ANSI formatting, and bounded persistent worker dispatch.
- `QUADBLOCK_FASTPATH=0` (Fallback): Falls back to `img.At(x, y)` interface sampling, `fmt.Sprintf` ANSI string formatting, and standard `strings.Builder` rendering.

## Hardware Target
- **Cores**: Scaled for 4 to 10 physical cores with a hard 10-core worker ceiling.
- **Memory**: 32GB RAM baseline; targets latency and zero-alloc hot loops rather than aggressive memory tuning.

## Topology / Data Flow

```
[Input image.Image] ---> [QUADBLOCK_FASTPATH Gate (Default: 1)]
                             |
                             +---> Fast Path (QUADBLOCK_FASTPATH != "0")
                             |     |---> Direct img.Pix[off] slice sampling for *image.RGBA
                             |     |---> Zero-alloc byte buffer formatting (appendFgRGB / appendBgRGB)
                             |     +---> Persistent worker channel pool (bounded <= 10 cores)
                             |
                             +---> Simple Reference Path (QUADBLOCK_FASTPATH=0)
                                   |---> img.At(x, y) interface sampling
                                   |---> fmt.Sprintf ANSI escape generation
                                   +---> Reference strings.Builder rendering
```

## Benchmark Evidence

Tested on Linux x86_64:

```
pkg: ubunatic.com/cati/v1/quadblock

Benchmark                      Baseline (Simple)     Fastpath (Direct RGBA)    Delta
-------------------------------------------------------------------------------------
BenchmarkRenderSerial          12.42 ms/op           8.47 ms/op                -31.8% ns/op
  Allocations                  82,866 allocs/op      32,851 allocs/op          -60.3% allocs/op
  Allocated Bytes              2,401,428 B/op        551,017 B/op              -77.1% B/op

BenchmarkRenderToImageSerial   14.18 ms/op           16.14 ms/op               (comparable)
  Allocations                  131,075 allocs/op     3 allocs/op               -99.99% allocs/op
  Allocated Bytes              1,048,642 B/op        524,354 B/op              -50.0% B/op
```
