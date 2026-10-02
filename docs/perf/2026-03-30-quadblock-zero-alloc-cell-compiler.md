# ⚡ Perf: Zero-Allocation Cell Compilation and Line Buffering in Quadblock Renderer

## Overview
Profiling of the quadblock renderer (`v1/quadblock`) revealed significant memory allocation churn during cell compilation and ANSI output generation. In a 256×256 frame rendering pipeline, over 8,200 heap allocations per call occurred due to:
1. Dynamic slice creation in `collectUnique` (`make([]color.RGBA, ...)` escaping to heap).
2. Temporary slice allocations in `pickBestPair` (`[]*quadCell{left, above}`) and `splitHalfCell` (`[]struct{...}`).
3. Closures instantiated per cell (`cellAt := func(...)`) inside cell computation routines (`computeQuadCell` and `RenderToImage`).
4. Re-allocation of byte slices when formatting ANSI escape outputs in `Render`.

By replacing dynamic slicing in `collectUnique` with fixed array returns `[4]color.RGBA`, inlining neighbor lookups, eliminating closure allocation overhead, and pre-allocating the ANSI line byte slice in `Render`, `RenderSerial` allocations dropped from **8,275 allocs/op to 72 allocs/op** (~99.1% reduction) and `RenderToImageSerial` allocations dropped from **16,387 allocs/op to 3 allocs/op** (~99.98% reduction), improving frame throughput by ~15%.

## Hardware Target
- **Cores & Workers**: Scaled and tested across 1, 2, 4, 8, and 10 CPU cores (`min(runtime.NumCPU(), 10)`).
- **Target Hardware**: Modern multi-core x86_64 / AMD APU architectures with 32GB RAM baseline.

## Topology / Data Flow

```
[Raw Pixel Input (2×2 Block)] ---> [Stack-Allocated Unique Color Collector [4]color.RGBA]
                                           |
                                           v
[Direct Neighbor Check (left, above)] ---> [Zero-Alloc Cell Compiler (compileCell)]
                                           |
                                           v
[Pre-Allocated Line Buffer (Width×32)] ---> [Stdout / Grid Output Writer]
```

## Benchmark Evidence

### Benchmark Comparison (`go test -bench=. -benchmem ./v1/quadblock`)

| Metric | Baseline (`RenderSerial`) | Optimized (`RenderSerial`) | Improvement |
| :--- | :--- | :--- | :--- |
| **ns/op** | 6,147,173 | 5,259,120 | **~14.4% faster** |
| **B/op** | 551,017 | 411,504 | **~25.3% less memory** |
| **allocs/op** | 8,275 | 72 | **~99.1% fewer allocs** |

| Metric | Baseline (`RenderToImageSerial`) | Optimized (`RenderToImageSerial`) | Improvement |
| :--- | :--- | :--- | :--- |
| **ns/op** | 10,688,089 | 8,983,458 | **~15.9% faster** |
| **B/op** | 786,496 | 524,352 | **~33.3% less memory** |
| **allocs/op** | 16,387 | 3 | **~99.98% fewer allocs** |
