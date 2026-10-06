# 094 — Restore quadblock fastpath parity for partial alpha and nonzero bounds

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: incoming merge `41f5f47` (GitHub PR #43), [performance notes](../docs/perf/2026-10-05-lockfree-wavefront-quadblock.md), [Testing](../docs/Testing.md), [Rendering Bug Playbook](../docs/RenderingBugPlaybook.md)

---

## 1. Problem & Motivation
Review of the incoming quadblock optimization found two violations of the fast/reference output-parity contract. Partially transparent NRGBA images now produce incorrect RGB values in reconstructed images, affecting image evaluation. Nonzero image origins expose an existing reconstruction bug that the new fast path bypasses. Existing parity tests use opaque, origin-zero fixtures and miss both cases.

## 2. Technical Specification / Findings
1. **New NRGBA regression:** `computeQuadCell` copies raw NRGBA bytes into `color.RGBA` for complete 2x2 cells. NRGBA stores unpremultiplied channels; the reference `safePixel` calls `toRGBA`, which obtains premultiplied channels from `Color.RGBA()`. For a constant 2x2 NRGBA image containing `(200,100,50,128)`, `RenderToImageJ(..., Options{}, 4)` produces `(200,100,50,128)` with `core.Fastpath=true`, versus `(100,50,25,128)` with it false. At width 3, the partial final cell takes `safePixel`, producing `(100,50,25,128)` in the last column while the first two columns remain `(200,100,50,128)`: a visible seam caused solely by cell bounds.
2. **Existing origin bug, newly exposed as fast/reference divergence:** reference `RenderToImageJ` computes `px0 = b.Min.X + tc*2` and `py0 = b.Min.Y + tr*2`, then compares them with destination-relative `pixW` and `pixH`. For a constant 2x2 image bounded by `image.Rect(5,7,7,9)`, it skips every cell and returns transparent pixels. The incoming direct-write path correctly uses destination-relative coordinates, so fast and reference modes now differ. Fix the reference guard; do not reproduce its blank output in the fast path.

Minimal reproducible input construction (run in a Go probe importing `image`, `image/color`, `v1/core`, and `v1/quadblock`):

```go
img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
for y := 0; y < 2; y++ {
    for x := 0; x < 3; x++ {
        img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
    }
}
core.Fastpath = true
fast := quadblock.RenderToImageJ(img, quadblock.Options{}, 4)
core.Fastpath = false
reference := quadblock.RenderToImageJ(img, quadblock.Options{}, 4)
// Compare fast.Pix and reference.Pix, then repeat with translated bounds.
```

The existing full suite and quadblock race suite passed during review. Neither result covers these missing parity cases. The row scheduler's atomic publication satisfies the current left/above dependencies; no scheduler correctness defect was identified.

## 3. Implementation & Verification Plan
**Goal**: Restore quadblock fast/reference parity for partial alpha and translated image bounds, or stop and report when blocked on a user decision or denied permission.

- Add regressions that fail before the fix, covering alpha 0/1/128/254/255, varying colors, even/odd sizes, nonzero bounds, and subimages with parent stride. Compare actual cell/grid and reconstructed pixel output for serial and worker rendering. Check ANSI routes where they preserve the affected input representation.
- Convert NRGBA samples with exactly the reference premultiplication and rounding semantics; keep transparent pixels zeroed. Correct the reference reconstruction's coordinate guard.
- Verify the expected pixel values independently; do not merely compare two implementations that share the same mistake.
- Follow the Rendering Bug Playbook and predict any existing golden impact before changes. No goldens were regenerated during review.
- Run both fast and reference full suites, quadblock race tests, `make preflight`, and relevant benchmarks. Update the renderer documentation with the sampling/coordinate contract and accurate validation results.
