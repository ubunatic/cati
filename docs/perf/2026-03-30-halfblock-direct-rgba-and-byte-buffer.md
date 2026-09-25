# Halfblock Fastpath: Direct RGBA Pixel Access & Reusable ANSI Byte Formatting

## Overview
In `v1/halfblock`, rendering images to terminal ANSI output or RGBA images was bottlenecked by two main allocation sources:
1. **Interface Boxing on Pixel Reads:** Every call to `image.Image.At(x, y)` in `RenderToGrid`, `ScaleNN`, and `ScaleToFit` allocates interface boxing wrappers for `color.Color` (99k+ allocations per frame on a 256x128 image).
2. **String Allocations in ANSI Escape Code Generation:** `Render` was using `strings.Builder` per line and `fmt.Sprintf` per cell to format 24-bit ANSI true-color foreground and background codes (`\x1b[38;2;r;g;bm` and `\x1b[48;2;r;g;bm`).

### The Optimization
- **Direct `*image.RGBA` Pixel Offsets:** When `core.Fastpath` is enabled (default) and input is `*image.RGBA`, `safePixel`, `ScaleNN`, and `ScaleToFit` directly sample from `rgba.Pix[off]` without interface boxing or heap allocations.
- **Reusable `[]byte` ANSI Buffer:** `Render` constructs output lines into a reusable `[]byte` slice using `strconv.AppendUint` and `utf8.AppendRune` before writing to `io.Writer`.
- **Direct Slice Pixel Reconstruction:** `RenderToImageJ` directly sets `dst.Pix[off]` bytes without calling `dst.SetRGBA(x, y, color)`.
- **Worker Ceiling:** Enforced 10-core max ceiling (`min(workerN, 10)` or `core.MaxWorkers()`) on parallel worker pools.

## Hardware Target
- **Target OS:** Linux x86_64 / Wayland / TTY environment.
- **Topology:** 4–10 Physical Cores with AVX-512 / Zen 4+ support, 32GB RAM baseline.

## Topology / Data Flow

```
[image.RGBA Source]
        │
        ▼ (Fastpath: Direct Pix[off] indexing)
[RenderToGrid (Worker Pool <= 10)]
        │
        ▼ (Cell Grid)
[Render Line Loop] ---> [Reusable []byte Buffer] ---> [strconv.AppendUint / utf8.AppendRune]
                                │
                                ▼
                        [io.Writer (stdout)]
```

## Benchmark Evidence

### Environment
- Go 1.25.0 linux/amd64
- Intel Xeon / multi-core environment

### Benchmarks Comparison

| Benchmark | Simple Path | Fastpath | Delta |
| :--- | :--- | :--- | :--- |
| `BenchmarkRenderSerial-4` (time) | `13,011,119 ns/op` | `2,168,416 ns/op` | **6.0x faster (-83.3%)** |
| `BenchmarkRenderSerial-4` (memory) | `5,193,440 B/op` | `310,664 B/op` | **-94.0% bytes** |
| `BenchmarkRenderSerial-4` (allocs) | `99,338 allocs/op` | `83 allocs/op` | **-99.9% allocs** |
| `BenchmarkRenderToImageSerial-4` (time) | `1,713,731 ns/op` | `928,818 ns/op` | **1.8x faster (-45.8%)** |
| `BenchmarkRenderToImageSerial-4` (memory) | `526,289 B/op` | `395,216 B/op` | **-24.9% bytes** |
| `BenchmarkRenderToImageSerial-4` (allocs) | `32,837 allocs/op` | `69 allocs/op` | **-99.8% allocs** |
