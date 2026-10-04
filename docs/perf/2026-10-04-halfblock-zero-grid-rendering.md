# ⚡ Performance Report: Halfblock Zero-Grid-Allocation Rendering & Direct Image Conversion

## Overview
- **Component**: [`v1/halfblock`](file:///home/uwe/projects/cati/v1/halfblock/render.go) terminal renderer and rasterizer.
- **Bottleneck**:
  1. **Intermediate Grid Allocation in Streaming Render**: Calling `Render(w, img, cols, opts)` previously invoked `RenderToGrid`, constructing an intermediate `*core.Grid` containing 2D row slices (`[][]core.Cell`) and allocating thousands of `core.Cell` structs on the heap (310 KB and 83 allocs/op), even when streaming ANSI escape sequences directly to an `io.Writer`.
  2. **Separate Slice Allocations in `RenderToGrid`**: `RenderToGrid` allocated row slices individually (`cells[i] = make([]core.Cell, width)`), creating `rowCount + 1` slice headers on the heap.
  3. **Double Conversion in `RenderToImageJ`**: `RenderToImageJ` rendered the image into a `*core.Grid` only to immediately unpack `cell.ch`, `cell.fg`, and `cell.bg` back into destination pixels (`dst.Pix`), generating 395 KB and 69 allocs/op.
- **Fix**:
  1. **Direct Row-by-Row ANSI Streaming**: `Render` now processes rows directly from `scaled` into a reusable line byte buffer (`buf []byte`) when rendering serially (`opts.Jobs <= 1`), bypassing `*core.Grid` creation entirely.
  2. **Flat Slice Allocation**: `RenderToGrid` allocates a single flat backing array (`flat := make([]core.Cell, rowCount*width)`) sliced into row views, reducing grid slice allocation overhead to 2 allocations.
  3. **Direct Image Conversion in `RenderToImageJ`**: `RenderToImageJ` converts pixel pairs directly into `dst.Pix` without constructing intermediate `*core.Grid` or cell structs.
  4. **Direct Row Offset Sampling**: Computes row byte offsets (`topRowOff := (topY - rect.Min.Y) * stride`) once per row for `*image.RGBA` inputs under `core.Fastpath`.

## Hardware Target
- **Target OS**: Modern Linux (Ubuntu, Fedora, CachyOS).
- **Target Hardware**: Multi-core x86_64 systems (4–10 cores, AVX-512, 32GB RAM baseline).

## Topology / Data Flow

```
[Input image.Image]
        │
        ├───> Render(w, img, ...) [Serial / Default]
        │     └───> Direct Row Loop ---> PairToCell ---> [Line buf []byte] ---> io.Writer (0 Grid Allocs)
        │
        ├───> RenderToGrid(img, ...)
        │     └───> Flat Slice Allocation [flat []core.Cell] ---> Grid Struct
        │
        └───> RenderToImageJ(img, jobs)
              └───> Direct Row Loop ---> PairToCell ---> dst.Pix (1 Output Allocation)
```

## Benchmark Evidence

Tested on Linux x86_64 (`Intel Xeon @ 2.30GHz`):

```
pkg: ubunatic.com/cati/v1/halfblock

Benchmark                      Baseline                      Optimized                     Delta
------------------------------------------------------------------------------------------------
BenchmarkRenderSerial          2,208,257 ns/op               1,686,040 ns/op               -23.6% ns/op (~1.3x faster)
  Allocations                  83 allocs/op                  3 allocs/op                   -96.4% allocs/op
  Allocated Bytes              310,664 B/op                  33,408 B/op                   -89.2% B/op

BenchmarkRenderToImageSerial   1,007,820 ns/op               514,714 ns/op                 -48.9% ns/op (~2.0x faster)
  Allocations                  395,216 B/op                  131,232 B/op                  -66.8% B/op
  Allocated Bytes              69 allocs/op                  3 allocs/op                   -95.7% allocs/op
```
