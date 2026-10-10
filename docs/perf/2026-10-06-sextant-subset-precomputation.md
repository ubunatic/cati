# Sextant Block Solver Subset Precomputation & Contiguous Cell Buffering

## Overview
Evaluating 2x3 block cell candidates (`chooseBestCell`) in `v1/sextant` previously iterated over pixel indices and executed 6 integer divisions (`idiv`) per mask across 64 candidate masks per cell. This resulted in over 380 integer division instructions and 64 temporary `cellResult` struct instantiations per cell, consuming over 50% of the entire frame rendering time.

This optimization introduces two main improvements:
1. **Inductive Subset Precomputation & Zero-Division Candidate Search**:
   - Calculates subset RGB sums inductively (`sR = sumR[sub & (sub-1)] + pr[5-trailingZeros(sub)]`) for all 64 pixel subsets in $O(1)$ operations per subset.
   - Pre-calculates subset squared error contributions (`subScore[sub]`) once before candidate evaluation.
   - Evaluates each of the 64 candidate masks via 2 table lookups (`totalSqSum + subScore[fgOpaque] + subScore[bgOpaque]`) with zero divisions, zero pixel loops, and zero struct allocations in the candidate loop.
   - Defers `cellResult` struct construction until after the best candidate mask is determined.

2. **Contiguous Grid Row Memory Allocation**:
   - Pre-allocates a single contiguous slice buffer (`buf := make([]core.Cell, rowCount*colCount)`) in `RenderToGrid`, eliminating per-row slice header heap allocations.
   - Uses direct byte slice indexing (`dst.Pix[off]`) in `RenderToImageJ` to bypass `image.RGBA.SetRGBA` bounds overhead.

## Hardware Target
- Target Architecture: Modern x86_64 / Zen 4+ APUs (4–10 Cores, AVX-512 / L1 Cache friendly contiguous slice layouts).
- Memory Model: 32GB RAM baseline; zero-allocation hot paths prevent CPU cache line eviction and GC pressure.

## Topology / Data Flow

```
[ 2x3 Pixel Sample ] ---> [ Inductive Subset Sums (sub & sub-1) ]
                                   |
                                   v
                      [ Precalculate subScore[64] ]
                                   |
                                   v
    [ Candidate Loop (64 Masks) ] ---> 2 Lookups + Additions (0 Divs, 0 Allocs)
                                   |
                                   v
               [ Post-Loop: Single cellResult Construct ]
                                   |
                                   v
             [ Contiguous Grid Buffer (RenderToGrid) ]
```

## Benchmark Evidence

Commands:
`go test ./v1/sextant/... -bench=. -benchmem -cpu=1,2,4,8,10`

### Benchmark Delta Summary

| Benchmark | Before | After | Delta |
| :--- | :--- | :--- | :--- |
| `BenchmarkChooseCell-10` | 5,668 ns/op (0 B/op, 0 allocs/op) | 2,692 ns/op (0 B/op, 0 allocs/op) | **-52.5% ns/op (2.1x faster)** |
| `BenchmarkRenderSextant-10` | 32,427,875 ns/op (114,216 B/op, 60 allocs/op) | 15,770,400 ns/op (116,264 B/op, 18 allocs/op) | **-51.4% ns/op (2.06x faster), -70% allocs/op** |
| `BenchmarkRenderToImageSextant-10` | 33,067,719 ns/op (131,360 B/op, 5 allocs/op) | 16,219,403 ns/op (131,360 B/op, 5 allocs/op) | **-50.9% ns/op (2.04x faster)** |
