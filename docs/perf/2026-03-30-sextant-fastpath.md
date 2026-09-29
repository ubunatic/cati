# Performance Improvement: Sextant Fastpath Direct Pixel Slicing & ANSI Byte Buffer Formatting

## Overview
- **Component:** `v1/sextant` (Terminal image renderer for Unicode 2x3 sextant block art)
- **Bottleneck:** During image sampling in `avgRegion()`, interface boxing calls to `image.Image.At(x, y)` produced tens of thousands of heap allocations per frame. Additionally, formatting ANSI 24-bit true-color escape sequences via `fmt.Sprintf` allocated thousands of temporary string objects in `Render()`, creating significant GC pressure.
- **Fix:**
  1. Gated on `core.Fastpath` (default enabled), implemented `safePixel()` to perform direct slice indexing on `*image.RGBA.Pix` byte arrays (`rgba.Pix[off]`), eliminating `color.Color` interface boxing.
  2. Replaced `fmt.Sprintf` calls in `Render()` with zero-allocation byte slice appending via `strconv.AppendUint` into reusable row line buffers.
  3. Replaced `dst.SetRGBA()` calls in `RenderToImageJ()` with direct byte offsets into destination `dst.Pix`.

## Hardware Target
- **Target OS:** Modern Linux
- **Target Topology:** Multi-core systems (capped at 10 workers for parallel chunking), x86_64 / AVX-512 architectures, 32GB RAM baseline.

## Topology / Data Flow

```
[Raw image.RGBA Input]
         |
         v
[safePixel Direct Pix Slicing]
         |
         v
[sampleBlock / avgRegion (6 Sub-regions)]
         |
         v
[chooseCell (Mask & Color Evaluation)]
         |
         v
[appendFgRGB / appendBgRGB (strconv.AppendUint)]
         |
         v
[Reusable Row Line Buffer ([]byte)] ---> [io.Writer]
```

## Benchmark Evidence

Tested with `go test -bench=. -benchmem -cpu=1,2,4,8,10 -count=5 ./v1/sextant`:

### `BenchmarkRenderSextant` (256x128 image rendering to ANSI)

| Metric | Before Optimization | After Optimization (Fastpath) | Change |
| :--- | :--- | :--- | :--- |
| **Allocs / op** | **44,679 allocs/op** | **60 allocs/op** | **-99.87% (-44,619 allocs)** |
| **Bytes / op** | **1,473,380 B/op** | **114,216 B/op** | **-92.2% (-1.35 MB/op)** |
| **Latency (ns/op)** | ~50,014,231 ns/op | ~44,836,895 ns/op | ~10.3% faster |

### `BenchmarkRenderToImageSextant` (256x128 image reconstruction)

| Metric | Before Optimization | After Optimization (Fastpath) | Change |
| :--- | :--- | :--- | :--- |
| **Allocs / op** | **33,029 allocs/op** | **5 allocs/op** | **-99.98% (-33,024 allocs)** |
| **Bytes / op** | **263,457 B/op** | **131,360 B/op** | **-50.1% (-132 KB/op)** |
| **Latency (ns/op)** | ~46,608,708 ns/op | ~44,631,931 ns/op | ~4.2% faster |
