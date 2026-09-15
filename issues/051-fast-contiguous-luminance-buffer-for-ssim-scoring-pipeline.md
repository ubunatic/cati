# 051 — Fast Contiguous Luminance Buffer for SSIM Scoring Pipeline

**Status**: Open  
**Priority**: P2 (Medium)  
**Severity**: Moderate  
**Category**: Performance  
**Related**: [`internal/metrics/metrics.go`](../internal/metrics/metrics.go), [`docs/ImageQualityMetrics.md`](../docs/ImageQualityMetrics.md), [`examples/imgbrowser/preview.go`](../examples/imgbrowser/preview.go)

---

## 1. Problem Description

In [`internal/metrics/metrics.go`](../internal/metrics/metrics.go), `SSIMLuminance` evaluates structural similarity across non-overlapping $8\times 8$ windows.

Inside the nested window loop:
```go
for dy := range winSize {
    for dx := range winSize {
        la := luma(a.At(ba.Min.X+x+dx, ba.Min.Y+y+dy))
        lb := luma(b.At(ba.Min.X+x+dx, ba.Min.Y+y+dy))
        // ...
    }
}
```
Each pixel access calls `image.Image.At(x, y)` (a Go interface method dispatch), unpacks 16-bit RGBA channels, and converts them to `float64` via `luma()`. For an image with hundreds of windows, this executes tens of thousands of interface calls and redundant luminance calculations per frame.

---

## 2. Proposed Changes & Grouped Work

1. **Sequential Luminance Buffer Extraction**:
   - Extract `image.Image` into a contiguous 1D or 2D slice (`[]float32` or `[]uint8` fixed-point) in a single linear sequential memory pass before window processing.
   - For standard `*image.RGBA` or `*image.NRGBA` inputs, use direct slice pixel indexing without `At()` virtual method overhead.

2. **Pointer / Slice Stride Window Traversals**:
   - Evaluate $8\times 8$ window statistics ($\mu_A, \mu_B, \sigma_A^2, \sigma_B^2, \sigma_{AB}$) by iterating directly over contiguous slice offsets with fixed row strides.

3. **SIMD / Unrolled Accumulators**:
   - Unroll inner $8\times 8$ accumulation loops so compilers can auto-vectorize luminance and variance sums.

---

## 3. Success Criteria & Verification

- [ ] Benchmark `BenchmarkSSIMLuminance` shows a $5\times\text{--}10\times$ speedup over current implementation.
- [ ] Metric computation latency for standard terminal viewports drops to $<0.3\,\text{ms}$.
- [ ] Accuracy matches `metrics.SSIMLuminance` within floating-point epsilon ($< 10^{-6}$).
- [ ] All tests in `internal/metrics` pass (`make test`).
