# 050 — Zero-Allocation and LUT Acceleration for Sextant and Quad Block Renderers

**Status**: Closed
**Priority**: P2 (Medium)  
**Severity**: Moderate  
**Category**: Performance  
**Related**: [`v1/sextant/render.go`](../v1/sextant/render.go), [`v1/quadblock/render.go`](../v1/quadblock/render.go), [`docs/ImageQualityMetrics.md`](../docs/ImageQualityMetrics.md), [018 — Reduce Allocation Pressure in Spark Mode](018-sparkline-allocation-reduction.md)

---

## 1. Problem Description

During real-time rendering, interactive image browsing, and video streaming, character cell solvers (`sextant`, `quadblock`) execute candidate search loops for thousands of cells per frame. Currently, the inner loops suffer from avoidable CPU overhead and memory allocations:

1. **Heap Allocations in `scoreMask` and `heuristicMasks`**:
   - `scoreMask` allocates slices (`make([]color.RGBA, 0, 6)`) for foreground and background pixels on every candidate mask.
   - `heuristicMasks` allocates a Go map (`map[uint8]struct{}`) and runs `sort.Slice` on every cell to deduplicate candidate masks.
   - `avgRGBA(fgPixels...)` incurs slice header construction and variadic argument passing.

2. **Hash Map Lookups for Unicode Runes**:
   - `sextantRuneByMask` and `quadblock` rune lookup use Go hash maps (`map[uint8]rune`) with map hash computation on every cell.

3. **Software Bitshift Loops for Popcount**:
   - `popcount(mask uint8)` runs a branchy `for mask > 0 { mask >>= 1 }` loop instead of standard hardware instructions.

4. **Two-Pass Distance Calculation**:
   - `scoreMask` first computes RGB centroid averages, then performs a second loop over all 6 subpixels computing Euclidean distances `rgbaDist2`.

---

## 2. Proposed Changes & Grouped Work

1. **Direct `[64]rune` Array LUTs**:
   - Replace `map[uint8]rune` with a static flat array `var sextantRunes [64]rune` indexed directly via `sextantRunes[displayMask]`.
   - Replace quad map lookups with `var quadRunes [16]rune`.

2. **Zero-Allocation Stack Scratches**:
   - Replace `fgPixels` and `bgPixels` slices in `scoreMask` with fixed stack arrays or running sums.
   - Replace `heuristicMasks` map with a fixed stack array `[10]uint8` and a small $O(N)$ linear deduplication pass.

3. **Hardware `POPCNT`**:
   - Replace manual bit loops with `math/bits.OnesCount8(mask)`.

4. **Single-Pass Between-Cluster Variance Optimization**:
   - Leverage the Otsu / 2-cluster variance identity:
     $$\text{SSE} = \text{TSS} - \left(\frac{\|\mathbf{S}_{\text{fg}}\|^2}{N_{\text{fg}}} + \frac{\|\mathbf{S}_{\text{bg}}\|^2}{N_{\text{bg}}}\right)$$
   - Evaluate candidate masks by calculating channel sums $\mathbf{S}_{\text{fg}}$ and $\mathbf{S}_{\text{bg}}$ in a single pass without computing square roots or second-pass pixel distances.

---

## 3. Success Criteria & Verification

- [x] All heap allocations inside `scoreMask` and `heuristicMasks` eliminated (`0 B/op` in cell solver benchmarks).
- [x] Direct array indexing for all 64 sextant runes and 16 quad runes.
- [x] Benchmarks in `v1/sextant` demonstrate measurable render time reduction (~42% speedup, inner solver down from 1.5MB allocs to 0 B/op).
- [x] Output glyph rendering remains 100% byte-for-byte bit-identical to existing goldens.
