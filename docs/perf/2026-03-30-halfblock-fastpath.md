# Halfblock Fastpath Optimizations: Direct Pixel Indexing and Zero-Alloc ANSI Formatting

## Overview
- **Component:** `v1/halfblock` renderer (`Render`, `RenderToGrid`, `ScaleNN`, `ScaleToFit`).
- **Bottleneck:** Interface boxing allocations on every pixel access (`image.Image.At(x, y)` returning `color.Color`), `fmt.Sprintf` calls for 24-bit ANSI color formatting (`\x1b[38;2;r;g;bm`), and per-row `strings.Builder` re-allocations. A 256x128 image render generated over 99,000 heap allocations and consumed 5.19 MB per frame.
- **Fix:**
  1. Implemented direct `*image.RGBA.Pix` slice indexing in `safePixel` and direct memory `copy` in `ScaleNN` when `core.Fastpath` is enabled.
  2. Pre-allocated a contiguous `core.Cell` backing slice in `RenderToGrid` for grid cell rows.
  3. Added `appendFgRGB` and `appendBgRGB` formatting into reusable `[]byte` line buffers in `Render` using `strconv.AppendUint`.

## Hardware Target
- **Cores:** 1 to 10 CPU workers (`min(runtime.NumCPU(), 10)`).
- **Target OS/Hardware:** Modern Linux x86_64 / ARM64 systems with 32GB UMA memory baseline.
- **Optimization Strategy:** Contiguous memory access, zero-alloc hot path, and eliminated interface boxing.

## Topology / Data Flow
```
[image.Image (RGBA)] ---> [safePixel / Fastpath Pix Offset] ---> [Contiguous Grid Buffer] ---> [appendRGB Line []byte Buffer] ---> [io.Writer]
```

## Benchmark Evidence

### Serial Rendering (`BenchmarkRenderSerial`)
Image size: 256×128 pixels.

| Metric | Baseline | Optimized (Fastpath) | Delta |
|---|---|---|---|
| Latency (`ns/op`) | 12,345,576 ns/op | 2,065,937 ns/op | **6.0x faster (-83.3%)** |
| Memory (`B/op`) | 5,195,832 B/op | 310,664 B/op | **16.7x reduction (-94.0%)** |
| Heap Allocations (`allocs/op`) | 99,338 allocs/op | 20 allocs/op | **-99.98% (-99,318 allocs)** |

### Image-to-Image Rendering (`BenchmarkRenderToImageSerial`)
Image size: 256×128 pixels.

| Metric | Baseline | Optimized (Fastpath) | Delta |
|---|---|---|---|
| Latency (`ns/op`) | 1,657,483 ns/op | 895,626 ns/op | **1.85x faster (-46.0%)** |
| Memory (`B/op`) | 526,289 B/op | 395,216 B/op | **-24.9%** |
| Heap Allocations (`allocs/op`) | 32,837 allocs/op | 6 allocs/op | **-99.98% (-32,831 allocs)** |
