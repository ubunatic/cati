# Sextant Solver Candidate Evaluation & Direct RGBA Image Fastpath

## Overview
Evaluating 2×3 Unicode sextant block candidates (`chooseBestCell`) accounts for over 93% of CPU execution time during terminal image rendering. In addition, `RenderToImageJ` performed virtual color method dispatch (`image.Image.SetRGBA`) for every reconstructed subpixel, creating unnecessary heap call overhead and memory operations.

This optimization implements four key improvements in `v1/sextant`:
1. **Uniform Opaque Cell Fastpath**: When all non-transparent subpixels in a 2×3 block are identical in color, `chooseBestCell` skips iterating through the 64 candidate masks and returns the exact single-color candidate directly.
2. **Fast Integer Luma & Unrolled Index Sum Aggregation**: Replaced floating-point `luma()` conversions in `directMask` and `chooseBestCell` with integer luma calculations (`2126*R + 7152*G + 722*B`) and unrolled `switch entry.count` scalar index sum assignments.
3. **Deferred Cell Result Construction & Candidate Pruning**: `chooseBestCell` defers `cellResult` struct construction until after the winning mask is determined, and prunes sub-optimal mask candidates early with `if score > bestScore { continue }`.
4. **Direct RGBA Slice Rendering & Atomic Dispatch**: `RenderToImageJ` uses direct slice offset writes on `*image.RGBA` destination buffers (`dst.Pix[off]`) and lock-free `atomic.Int32` row scheduling across worker threads.

## Hardware Target
- **Topology**: 4–10 Cores (capped with `core.MaxWorkers()`), Modern x86_64 / AMD Zen 4+.
- **Memory Target**: Zero allocations per cell in candidate search, contiguous 64-byte aligned pixel buffer access.

## Topology / Data Flow

```
[Source RGBA/NRGBA Image]
        │
        ▼
[2×3 Subpixel Block Sampler (sampleBlock)]
        │
        ├── All Opaque Pixels Equal? ──► [Instant Closed-Form Cell Result]
        │
        ▼
[Fast Integer Luma (2126R + 7152G + 722B)]
        │
        ▼
[64-Mask Candidate Error Scoring (unrolled sums)]
        │
        ├── score > bestScore ─────────► [Skip Overlap / Popcount Calculations]
        │
        ▼
[Single Post-Loop cellResult Construction]
        │
        ▼
[Direct RGBA Slice Reconstruction / ANSI Line Buffer]
```

## Benchmark Evidence

Command: `go test -bench=. -benchmem -cpu=1,2,4,8,10 ./v1/sextant/`

```
pkg: ubunatic.com/cati/v1/sextant
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz

BenchmarkChooseCell-4:
  Before:  5715 ns/op,     0 B/op,  0 allocs/op
  After:   5123 ns/op,     0 B/op,  0 allocs/op  (-10.4% latency)

BenchmarkRenderSextant-4:
  Before:  32.75 ms/op, 114216 B/op, 60 allocs/op
  After:   29.19 ms/op, 114216 B/op, 60 allocs/op  (-10.9% frame time)

BenchmarkRenderToImageSextant-4:
  Before:  33.16 ms/op, 131360 B/op,  5 allocs/op
  After:   29.05 ms/op, 131216 B/op,  3 allocs/op  (-12.4% frame time, -40% allocs)
```
