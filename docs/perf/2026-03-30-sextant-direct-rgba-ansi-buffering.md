# ⚡ Performance Record: Direct RGBA Slice Indexing and Byte Buffer ANSI Formatting in Sextant Renderer

## Overview

In the `v1/sextant` block solver and renderer, inner pixel-sampling loops (`avgRegion`) were invoking `image.Image.At(x, y)` interface calls, causing dynamic interface boxing allocations for every sampled pixel. In addition, `fmt.Sprintf` calls for 24-bit ANSI color formatting (`\x1b[38;2;r;g;bm` / `\x1b[48;2;r;g;bm`) and `strings.Builder` allocations during row rendering generated over 44,000 heap allocation objects per render frame.

This optimization implements a dual-path direct `*image.RGBA` byte slice indexing mechanism (`rgba.Pix[off]`) in `avgRegion` when `core.Fastpath` is enabled, and replaces `fmt.Sprintf` formatting with byte buffer appending using `strconv.AppendUint`.

## Hardware Target
- Multi-core x86_64 systems (4–10 physical cores tested).
- Memory target: 32GB baseline, zero hot-loop allocation churn.

## Topology / Data Flow

```
[Raw image.Image Input]
          |
          v (Type assert *image.RGBA)
+-----------------------------------+
|  avgRegion Direct Pix Offset Loop |  ---> [Zero-Alloc Region Pixel Sampling]
+-----------------------------------+
          |
          v
+-----------------------------------+
|  Sextant Score & Cell Selection   |
+-----------------------------------+
          |
          v
+-----------------------------------+
| appendFgRGB / appendBgRGB (Byte)  |  ---> [Zero-Alloc ANSI String Formatting]
+-----------------------------------+
          |
          v
[Stdout / Destination io.Writer]
```

## Benchmark Evidence

`go test ./v1/sextant -bench=BenchmarkRenderSextant -benchmem`

| Metric | Before | After | Delta |
| :--- | :--- | :--- | :--- |
| **Throughput (ns/op)** | 48,351,202 ns/op | 43,916,039 ns/op | **-9.17%** latency |
| **Allocated Bytes (B/op)** | 1,473,123 B/op | 114,216 B/op | **-92.2%** memory footprint |
| **Allocations (allocs/op)** | 44,679 allocs/op | 60 allocs/op | **-99.87%** allocations |
