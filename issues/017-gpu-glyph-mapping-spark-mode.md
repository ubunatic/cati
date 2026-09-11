# 017 — GPU Assistance for Glyph Mapping, Starting with Spark Mode

**Status:** 🟡 In Progress  
**Refs:** `v1/sparkline`, `cmd/interactive.go`, `cmd/metrics.go`, `cmd/ssim.go`, `v1/quadblock`, `v1/halfblock`

---

## Context

The glyph-mapping path does per-cell analysis to match image blocks against candidate glyph masks. Spark mode and composite spark modes (e.g. spark+quad, spark+six) are the most compute-heavy paths because each cell tests multiple candidate masks with FG/BG separation and SSE scoring.

Spark mode operates on fixed-size analysis blocks with a clearly bounded reconstruction step:

- source pixels are grouped into blocks (e.g. `4×8`, `2×3`, `2×6`, `4×24`)
- each block is mapped to a glyph plus fg/bg colours
- the same mapping feeds ANSI output, `RenderToImage`, and SSIM/quality scoring

## Goal

Explore vectorized, SIMD-friendly, and GPU-assisted paths for glyph selection and reconstruction in spark mode, establishing reference performance baselines.

The intended use is acceleration of the glyph mapping work itself, not a rewrite of the terminal I/O or viewer UI.

## Hotspot Analysis & Vectorized CPU Solver

Analysis of `FindBestCell` candidate search revealed three major bottlenecks in the original scalar path:
1. **Redundant Pixel Access & Function Pointer Overhead**: For $K$ candidates and an $N$-pixel block, image pixels were accessed $2 \times K \times N$ times, each evaluating dynamic closures (`cand.mask(x, y, w, h)`).
2. **Dual-Pass Loop Over Block Pixels**: Evaluating SSE required one loop to sum FG/BG colors and a second loop over all pixels to compute Euclidean distance to average colors.
3. **Heap/Interface Overhead**: Dynamic candidate matching prevented compiler loop vectorization.

### Optimization Strategy (`v1/sparkline/solver.go`)

1. **Bitwise Mask Representation (`mask128`)**:
   Glyph candidate masks for blocks up to 128 pixels (4x8=32, 2x3=6, 2x6=12, 4x24=96) are represented as 64-bit/128-bit bitmasks. Pixel opacity and candidate FG/BG memberships are evaluated using fast bitwise operations (`AND`, `AND NOT`, `bits.OnesCount64`, `bits.TrailingZeros64`).
2. **Single-Pass Pixel Extraction**:
   Block pixels are loaded into stack arrays in a single pass. Total color sums and $\text{totalSqSum} = \sum ||p_i||^2$ are computed once per cell.
3. **Algebraic SSE Reduction ($O(1)$ Error Evaluation)**:
   Leveraging the algebraic expansion:
   $$\sum_{i \in \text{region}} ||p_i - \mu||^2 = \sum_{i \in \text{region}} ||p_i||^2 - 2\mu \cdot \sum_{i \in \text{region}} p_i + N_{\text{region}} ||\mu||^2$$
   The second pixel iteration loop is eliminated entirely. SSE is evaluated with a few scalar integer operations per candidate.
4. **Zero Allocation**:
   Precomputed static bitmask tables for standard geometries (4x8, 2x2, 2x3, 2x6, 4x24) yield 0 B/op and 0 allocs/op across all modes.

## Benchmark Measurements (AMD Ryzen 7 8700G)

### Candidate Matching Kernels (`BenchmarkFindBestCell*`, 256×128 image, 1024 cells)

| Kernel Mode | Baseline (Original CPU) | Optimized (Vector/Bitwise) | Speedup | Allocs |
| :--- | :--- | :--- | :--- | :--- |
| `Vertical` (4×8, 8 cands) | `2.97 ms/op` | `0.30 ms/op` (304 µs) | **9.7×** | `0 B/op, 0 allocs` |
| `HalfSplit` (4×8 / 2×2, 6 cands) | - | `0.24 ms/op` (243 µs) | - | `0 B/op, 0 allocs` |
| `Spark` (4×8, 22 cands) | - | `0.53 ms/op` (526 µs) | - | `0 B/op, 0 allocs` |
| `Quad` (4×8, 16 cands) | `11.68 ms/op` | `0.76 ms/op` (763 µs) | **15.3×** | `0 B/op, 0 allocs` |
| `Sextant` (4×8, 66 cands) | - | `1.63 ms/op` | - | `0 B/op, 0 allocs` |
| `SixHalf` (4×8, 72 cands) | - | `1.73 ms/op` | - | `0 B/op, 0 allocs` |
| `Best` (4×24, 88 cands) | - | `2.05 ms/op` | - | `0 B/op, 0 allocs` |

### Full Serial Render Pipeline

| Pipeline Benchmark | Baseline | Optimized | Speedup |
| :--- | :--- | :--- | :--- |
| `BenchmarkRenderSerial` | `3.01 ms/op` | `0.56 ms/op` (558 µs) | **5.4×** |
| `BenchmarkRenderToImageSerial` | `2.98 ms/op` | `0.54 ms/op` (541 µs) | **5.5×** |

## GPU Acceleration Insights

With the CPU kernel running in **0.30ms - 0.76ms** serially for a full 256×128 image (and <0.1ms with multi-worker parallelism):
- Per-cell GPU dispatch is unviable due to PCIe/command buffer launch latency (~10–50 µs).
- Any future GPU path (Vulkan Compute / OpenCL / WebGPU compute) must dispatch at least an entire frame or multi-frame video buffer in a single compute pass.
- The bitwise algebraic solver formulated here maps directly to GPU compute shader workgroups (each workgroup processing a `4×8` cell with 32 threads using subgroup ballot and warp shuffle reductions).

