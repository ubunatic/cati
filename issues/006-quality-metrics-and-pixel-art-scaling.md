# 006 — Quality Metrics: Blockiness, Edge Continuity, and Pixel-Art Scaling

**Status:** 🔴 Open  
**Ref:** `cmd/ssim.go`, `cmd/render_quality_test.go`, `internal/quadblock/`, `internal/halfblock/`

---

## Motivation

Current SSIM (pyramid reference) measures overall luminance similarity but cannot
distinguish between two failure modes that matter differently to the eye:

1. **Blocky artefacts** — hard colour boundaries at every 2×2 cell boundary that
   do not correspond to edges in the source. Naive quad mode produces these on
   smooth gradients (e.g. sky, skin).

2. **Edge smearing** — real source edges (toy light-sabre, helmet outline, PCB
   traces) are blurred or misaligned because the algorithm averages across the
   boundary. `BlendAmbiguous` is the worst offender; `splithalf` is much better.

A per-algorithm score that captures both failure modes independently would:
- Guide algorithm selection (use blocky-tolerant modes for line art, blur-tolerant
  for photos)
- Enable automatic mode selection per image content
- Provide ground truth for future pixel-art scaling algorithm development

---

## Phase 1 — Blockiness Score

**What:** Penalise algorithms that introduce grid-aligned edges not present in
the source.

**How:**

1. Compute the horizontal and vertical gradient magnitude of the rendered image
   at cell-boundary positions (every 2px for quad, every 1×2 for halfblock).
2. Compute the same gradient at the same positions in the pyramid reference.
3. `blockiness = mean( max(0, rendered_grad - ref_grad) )` at all cell boundaries.
   Excess gradient at a boundary = artificially introduced block edge.
4. Normalise to [0, 1]; score = `1 - blockiness` so higher = less blocky.

**Implementation sketch (`cmd/metrics.go`):**

```go
// GradientAt computes |∂L/∂x| + |∂L/∂y| at pixel (x, y) using central differences.
func gradMag(img image.Image, x, y int) float64

// BlockinessScore returns 1 - excess_block_edge_gradient normalised to [0,1].
// cellW/cellH: the cell size in pixels (1×2 for halfblock, 2×2 for quad).
func BlockinessScore(ref, rendered image.Image, cellW, cellH int) float64
```

**Expected outcome:**

| Mode | Blockiness score |
|------|-----------------|
| halfblock | ~0.95 (soft row boundary) |
| quad/default | ~0.70 (every cell is a hard box) |
| quad/splithalf | ~0.85 |
| quad/pca2+blend | ~0.90 (blending softens block edges) |

---

## Phase 2 — Edge Continuity Score

**What:** Reward algorithms that preserve real source edges across cell
boundaries.

**How:**

1. Detect edges in the pyramid reference using a Sobel operator; threshold to a
   binary edge map.
2. Detect edges in the rendered image at the same positions.
3. `edge_continuity = |ref_edges ∩ rendered_edges| / |ref_edges|`
   (recall of source edges in the render).
4. Weight by gradient magnitude of the reference (stronger source edge = higher
   weight), so thin noise edges don't dominate.

**Implementation sketch:**

```go
// EdgeMap returns a float64 image of gradient magnitudes using 3×3 Sobel.
func EdgeMap(img image.Image) [][]float64

// EdgeContinuity returns the weighted recall of reference edges in rendered [0,1].
func EdgeContinuity(ref, rendered image.Image) float64
```

**Expected outcome:**

| Mode | Edge continuity |
|------|----------------|
| halfblock | ~0.80 (1px row resolution = misses vertical sub-cell edges) |
| quad/splithalf | ~0.85 (preserves per-pixel colour, small mask may clip edge) |
| quad/pca2+blend | ~0.65 (blending smears edges across cell boundary) |
| Future cleanEdge | ~0.95 (target) |

---

## Phase 3 — Higher-Res Cell Comparison (up to 8×8)

**Problem:** At the current per-cell scale (1×2 for halfblock, 2×2 for quad) the
SSIM window covers only a few cells; edge and blockiness signals are coarse.

**Proposed change:** treat each terminal cell as N×N source pixels for
comparison, up to N=8 (configurable `maxCellPx`):

| Mode | Cell pixels (N=4) | Cell pixels (N=8) |
|------|------------------|--------------------|
| halfblock | 4×8 (4 wide, top/bottom 4×4) | 8×16 |
| quad | 2×2 per quadrant → 4×4 cell | 4×4 per quadrant → 8×8 cell |

**Steps:**

1. Reconstruct the rendered image at N×cell_size resolution using
   `RenderToImageN(vp, opts, N)` — upscale each cell's fg/bg block to N×N per
   quadrant pixel. For halfblock: top-half block = N×(N/2) solid fg, bottom = N×(N/2) solid bg.
2. Scale the pyramid reference to the same N×cell_size resolution.
3. All scores (SSIM, blockiness, edge continuity) computed at the higher resolution.

This directly shows how much original detail is lost: a PCB trace crossing a
single cell will appear as a tiny stripe in the reference but a solid colour in
the render → SSIM drops accordingly.

**Cap at N=8** to bound compute: 8×8 per cell at 80×40 terminal = 640×640
comparison image = reasonable for test + live hint bar.

**Implementation notes:**
- `RenderToImageN` lives in `internal/quadblock/render.go` (quad) and a new
  `internal/halfblock/render.go` (halfblock).
- The benchmark test gains a `cellScale int` parameter (default 4, max 8).
- `buildRef` in `cmd/ssim.go` gains a `cellScale` param and uses it when
  computing `viewW`, `viewH`.

---

## Phase 4 — Pixel-Art Scaling Algorithms

The edge-continuity score provides ground truth for evaluating pixel-art style
upscaling of the source before cell encoding. Good pixel-art scalers preserve
thin diagonal lines and corners instead of blurring them.

Algorithms to evaluate (all applied to the source image before `ScaleToFit`):

| Algorithm | Description | Ref |
|-----------|-------------|-----|
| **EPX / Scale2×** | Pixel doubles only if neighbours match; straight lines become smooth curves | Kreed 1999 |
| **hq2×/hq3×** | Combines luminance comparison + sub-pixel interpolation | Maxim Stepin |
| **cleanEdge** | Pixorama's edge-detection pre-pass: sharpens diagonal features | Pixorama |
| **OmniScale** | Neural-style learned pixel-art upscaler (needs weights or C port) | — |
| **xBR** | Pattern matching on 3×3 neighbourhood to decide sub-pixel colour | — |

**Integration plan:**
- Each scaler is a `func(src image.Image, scale int) image.Image` in a new
  `internal/pixelart/` package.
- The benchmark test accepts a `preScaler` option; all existing variants are
  re-run with and without each scaler.
- The winner(s) are added to `quadblock.Options` as `PreScale PreScaleAlgo`.
- `renderModes` in `cmd/interactive.go` gains new entries (e.g. `quad/splithalf+cleanedge`).

---

## Phase 5 — Composite Score + Automatic Mode Selection

Combine SSIM, blockiness, and edge-continuity into a weighted composite:

```
Q = w_ssim * SSIM + w_block * BlockinessScore + w_edge * EdgeContinuity
```

With learned or heuristic weights (e.g. 0.5, 0.25, 0.25).

Use image statistics (edge density, gradient variance) to automatically choose
the best render mode without manual R-key cycling:
- High edge density (line art, PCB): prefer `splithalf` or `cleanEdge` variant
- Low edge density (sky, gradients): prefer `pca2` or `lum-split`
- High variance (portraits): prefer `kmeans3+hb2`

---

## Benchmark Baseline (2026-06-25)

Reference: pyramid downscale, SSIM only, 80×40 terminal, 3 sample images.

```
quad/splithalf       0.8378 ← best quad
quad/splithalf-nb    0.8373
quad/hb3             0.8343
quad/pca2+blend      0.8329  (blurs, demoted with pyramid ref)
halfblock            0.8250  (correctly < 1.0 now)
quad/default         0.7492  (worst)
```

Key finding: `splithalf` wins on detail-heavy images (PCB); `pca2+blend`
competes on smooth images but is correctly penalised for blur with the pyramid
reference.

---

## Open Questions

- Should the composite score be shown in the hint bar, or just SSIM?
- OmniScale requires a neural model — do we ship weights or just document it?
- For `RenderToImageN`, should N be configurable at runtime (settings) or just
  at test time?
