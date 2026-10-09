# Performance Optimization: Contiguous Grid Pre-allocation and Direct Fastpath Row Sampling in Halfblock

## Overview
In `v1/halfblock`, rendering an image via `RenderToGrid` previously allocated separate slice backing arrays for every row (`make([]core.Cell, width)`), incurring $O(\text{rowCount})$ heap allocations per frame. Furthermore, per-pixel color extraction (`safePixel`) performed repeated image boundary checks, type assertions (`img.(*image.RGBA)`), and byte offset arithmetic for every pixel in every row.

This optimization implements:
1. Contiguous single-slice allocation for the entire cell grid (`cellBuf := make([]core.Cell, rowCount*width)`), reducing grid slice allocations to $O(1)$.
2. Direct fastpath row sampling for `*image.RGBA` and `*image.NRGBA` inside `RenderToGrid` and `safePixel` under `core.Fastpath`, computing row byte offsets (`topRowOff`, `botRowOff`) once per row and reading raw pixel byte slices directly.
3. Output line byte buffer pre-allocation (`make([]byte, 0, width*32)`) in `Render` to prevent dynamic reallocation churn during ANSI escape formatting.

## Hardware Target
- **Target OS / Platform:** Modern Linux (x86_64 / Zen 4+ / AVX-512).
- **Core Ceiling:** Capped at $\le 10$ worker threads.
- **Memory Footprint:** 32GB UMA baseline, prioritizing inner-loop GC allocation elimination and throughput latency.

## Topology / Data Flow

```
[Raw Image (RGBA / NRGBA)]
          │
          ▼
[ScaleToFit (Nearest-Neighbor)]
          │
          ▼
[Contiguous Cell Grid Allocation]
 ├── Single backing slice: cellBuf [rowCount * width]
 └── Row headers slice: cells [rowCount]
          │
          ▼
[Fastpath Row Loop (core.Fastpath)]
 ├── Compute topRowOff & botRowOff per row
 ├── Direct Pix[off..off+4] sampling (RGBA & NRGBA premultiplied)
 └── Zero-alloc cell generation (pairToCell)
          │
          ▼
[Pre-allocated Line Buffer (width * 32)] ---> [Stdout / io.Writer]
```

## Benchmark Evidence

### `v1/halfblock` Benchmarks (`256x128` test image)

| Metric | Baseline | Optimized | Delta |
| :--- | :--- | :--- | :--- |
| **`BenchmarkRenderSerial` (ns/op)** | `2,230,587` | `1,921,290` | **-13.8%** |
| **`BenchmarkRenderSerial` (B/op)** | `310,664` | `297,488` | **-4.2%** |
| **`BenchmarkRenderSerial` (allocs/op)** | `83` | `7` | **-91.5%** |
| **`BenchmarkRenderToImageSerial` (ns/op)** | `1,014,825` | `728,419` | **-28.2%** |
| **`BenchmarkRenderToImageSerial` (B/op)** | `395,216` | `395,216` | **0.0%** |
| **`BenchmarkRenderToImageSerial` (allocs/op)** | `69` | `6` | **-91.3%** |

### Summary
- Heap allocations per frame dropped from **83 allocs/op** to **7 allocs/op** in `RenderSerial` and **69 allocs/op** to **6 allocs/op** in `RenderToImageSerial` (>91% allocation reduction).
- Frame rendering latency dropped from **1.01ms** to **0.72ms** in `RenderToImageSerial` (**28.2% throughput increase**).
