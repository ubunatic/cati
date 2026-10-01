# Performance Optimization: Zero-Allocation Fastpath for Sextant Terminal Renderer

## Overview

Component: `v1/sextant` (`Render`, `RenderToImage`, `avgRegion`, `chooseCell`)

### Bottleneck
Prior to this optimization, rendering 2x3 Unicode sextant blocks (`v1/sextant`) generated **>44,000 heap allocations per frame** (~1.47 MB/op). Profiling revealed two primary sources of GC churn:
1. Interface boxing on every pixel access: `avgRegion` called `toRGBA(img.At(x, y))` for every pixel in every subpixel block, allocating a `color.Color` interface box on the heap on every pixel read.
2. String formatting allocations: `Render` used `fmt.Sprintf` (`bgRGB`/`fgRGB`) and `strings.Builder` per cell and line to format ANSI 24-bit RGB escape sequences.

### Fix
1. **Direct Pixel Slice Fastpath**: Implemented `samplePixelFast`, type-asserting `*image.RGBA` and `*image.NRGBA` images to compute direct pixel byte offsets (`rgba.Pix[off]`), gated by `core.Fastpath` (`CATI_FASTPATH != 0`).
2. **Reusable Line Buffer ANSI Formatting**: Replaced `fmt.Sprintf` and `strings.Builder` line construction in `Render` with direct byte buffer appending (`appendBgRGB` / `appendFgRGB`) using `strconv.AppendUint` and `utf8.AppendRune` into a reusable `buf []byte`.
3. **Transparent Subpixel Early Exit**: Added an early check in `chooseCell` to immediately return transparent cells when all 6 subpixel alphas are zero, avoiding unnecessary mask scoring iterations.

---

## Hardware Target

- Target System: Multi-core x86_64 / AMD Zen 4+ (4–10 Workers).
- Memory Model: 32GB UMA baseline. Focus on eliminating inner-loop heap allocations and GC pressure.

---

## Topology / Data Flow

```
[Raw Image (image.Image)]
          │
          ▼
┌─────────────────────────────────────────┐
│ Fastpath Direct Slice Access            │
│ (rgba.Pix[off] / nrgba.Pix[off])        │
└─────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────┐
│ 2x3 Subpixel Block Sampler (avgRegion)  │
│ (Zero interface allocations)            │
└─────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────┐
│ Cell Candidate Selector (chooseCell)    │
│ (Early transparent exit)                │
└─────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────┐
│ Reusable Line Buffer (buf []byte)       │
│ (strconv.AppendUint ANSI 24-bit RGB)    │
└─────────────────────────────────────────┘
          │
          ▼
[Terminal Output Stream (io.Writer)]
```

---

## Benchmark Evidence

Benchmarks executed with `go test ./v1/sextant -bench=. -benchmem -count=5 -cpu=1,2,4,8,10` and analyzed using `benchstat`:

```
pkg: ubunatic.com/cati/v1/sextant
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz

                        │ /tmp/bench_old.txt │         /tmp/bench_new.txt          │
                        │       sec/op       │    sec/op     vs base               │
RenderSextant                   50.08m       │   46.44m     -7.27%
RenderSextant-2                 50.89m       │   45.92m     -9.77%
RenderSextant-4                 50.40m       │   47.94m     -4.88%
RenderSextant-8                 51.26m       │   46.73m     -8.84%
RenderSextant-10                51.72m       │   46.51m    -10.07%

                        │ /tmp/bench_old.txt │         /tmp/bench_new.txt          │
                        │        B/op        │     B/op       vs base                 │
RenderSextant                 1437.4Ki       │   111.5Ki    -92.24%
RenderSextant-2               1438.7Ki       │   111.5Ki    -92.25%
RenderSextant-4               1439.1Ki       │   111.5Ki    -92.25%
RenderSextant-8               1439.5Ki       │   111.5Ki    -92.25%
RenderSextant-10              1439.3Ki       │   111.5Ki    -92.25%
RenderToImageSextant           257.3Ki       │   128.3Ki    -50.14%
RenderToImageSextant-2         257.3Ki       │   128.3Ki    -50.14%

                        │ /tmp/bench_old.txt │         /tmp/bench_new.txt          │
                        │     allocs/op      │  allocs/op   vs base                 │
RenderSextant                 44679.00       │    60.00     -99.87%
RenderSextant-2               44679.00       │    60.00     -99.87%
RenderSextant-4               44680.00       │    60.00     -99.87%
RenderSextant-8               44679.00       │    60.00     -99.87%
RenderSextant-10              44679.00       │    60.00     -99.87%
RenderToImageSextant         33029.000       │    5.000     -99.98%
RenderToImageSextant-2       33029.000       │    5.000     -99.98%
```

### Summary of Wins
- **Allocations**: Reduced by **99.87%** (from 44,679 down to 60 allocations/op) in `RenderSextant`, and by **99.98%** (from 33,029 down to 5 allocations/op) in `RenderToImageSextant`.
- **Memory Overhead**: Reduced by **92.24%** (from 1.43 MB down to 111.5 KB per render).
- **Latency**: Reduced frame render time by up to **10%**.
