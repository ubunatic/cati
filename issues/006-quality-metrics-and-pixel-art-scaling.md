# 006 — Quality Metrics: Blockiness, Edge Continuity, and Pixel-Art Scaling

**Status:** 🔄 In Progress  
**Ref:** `cmd/metrics.go`, `cmd/ssim.go`, `cmd/ssim_bench_test.go`, `internal/quadblock/`, `internal/halfblock/`

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

## Phase 1 — Blockiness Score ✅ Implemented

**What:** Penalise algorithms that introduce grid-aligned edges not present in
the source.

**How:** Precompute Sobel gradient grids (`sobelGrid`) for reference and rendered;
measure excess gradient at cell-boundary pixel positions:
- halfblock: only horizontal boundaries (every 2 pixel rows) — no vertical cell grid
- quad: both axes (every 2 pixel rows and 2 pixel columns)

`blockiness_score = 1 - clamp(mean_excess / 0.15, 0, 1)` — higher = fewer block artefacts.

**Implementation:** `cmd/metrics.go` — `lumaGrid`, `sobelGrid`, `blockinessFromGrids`

**Observed results (3 sample images):**

| Mode | SSIM | Blk | Edge |
|------|------|-----|------|
| quad/splithalf | 0.838 | 0.830 | 0.976 |
| quad/lum+ambig | 0.833 | **0.912** | 0.838 |
| halfblock | 0.825 | 0.760 | **0.995** |
| quad/pca2 | 0.823 | 0.797 | 0.989 |
| quad/default | 0.749 | 0.739 | 0.971 |
| quad/blend-wide | 0.744 | **0.903** | 0.664 |

Key insight: `lum+ambig` and `pca2+wide` score high on Blk (blending softens
block edges) but score low on Edge (blending smears real edges). `splithalf`
balances both well. `blend-wide` has highest Blk but lowest Edge — over-blurring
destroys edge information.

---

## Phase 2 — Edge Continuity Score ✅ Implemented

**What:** Reward algorithms that preserve real source edges across cell boundaries.

**How:** Weighted recall of Sobel edges in reference image vs rendered image.
For each pixel in the reference Sobel grid exceeding threshold 0.05, credit is
given if the rendered image has a comparable edge within a ±1 pixel neighbourhood.
Score = weighted_hit / weighted_total where weights = reference gradient magnitude.

**Implementation:** `cmd/metrics.go` — `edgeContinuityFromGrids`

**Key findings from observed data:**
- halfblock: Edge = 0.995 (almost all source edges preserved — NN sampling is faithful)
- quad/splithalf: Edge = 0.976 (strong — uses per-pixel colours for fg/bg)
- quad/pca2: Edge = 0.989 (PCA alone doesn't smear edges)
- quad/blend-wide: Edge = 0.664 (5×5 blending destroys real edges badly)
- blend modes generally trade Edge for Blk: smoother cell transitions but weaker edge recall

**Observations vs original expectation:**
- halfblock scores ~0.995 (much better than expected ~0.80) — confirms that NN scaling
  actually preserves edges very well; the perceived detail loss is about colour
  quantisation within cells, not edge erasure
- `pca2+blend` edge ≈ 0.834 (correctly worse than pure `pca2` at 0.989)

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

## Phase 4 — Pixel-Art Scaling Algorithms ✅ Infrastructure implemented

**Infrastructure:** `internal/pixelart/` package with `func(image.Image) image.Image` signature.
`renderCfg.preScale` field wires it into `scaleToFit`; interactive R-key cycle includes
`+epx2x` and `+sharp` variants. `renderCfg.id` used for comparison (func fields are not comparable).

### Implemented algorithms

| Algorithm | File | Description |
|-----------|------|-------------|
| **Scale2x (EPX)** | `pixelart.go` | Corner-aware 2× upscale (Kreed 1999); fires on exact pixel-equality |
| **Scale3x** | `pixelart.go` | 3× version of EPX |
| **Sharpen (unsharp mask)** | `pixelart.go` | 3×3 blur subtracted from original; amount=0.5 or 1.0 |

### Benchmark results (3 sample images, pyramid ref)

| Variant | SSIM | Blk | Edge | Notes |
|---------|------|-----|------|-------|
| quad/splithalf+epx3x | **0.838** | 0.831 | 0.976 | tied #1 overall |
| quad/splithalf+epx2x | **0.838** | 0.831 | 0.976 | tied #1 overall |
| quad/splithalf | **0.838** | 0.830 | 0.976 | no pre-scaler, same score |
| quad/splithalf+sharp0.5 | 0.825 | 0.811 | 0.978 | slight Blk improvement vs bare splithalf |
| halfblock+epx2x | 0.825 | 0.760 | 0.995 | marginal gain vs halfblock alone (darth img) |
| halfblock+sharp1.0 | 0.796 | 0.705 | 0.996 | SSIM drops — sharpening hurts at this scale |

### Key findings

1. **EPX / Scale2x has no measurable effect on photos.** The exact-pixel-equality
   condition (N==W etc.) almost never fires at photographic gradients. EPX degrades
   to simple 2× NN, which is then downscaled back — identical to the baseline NN.
   This is expected: EPX is designed for pixel art (flat-colour regions with 1-px edges),
   not continuous-tone photographs.

2. **Unsharp mask hurts SSIM but may improve perceived sharpness.** At large
   downscale ratios (25:1), sharpening the full-resolution image adds high-frequency
   noise that the cell encoder cannot represent → SSIM drops. It may still look
   sharper to the eye on some images (different from SSIM ranking).

3. **The cell-encoding algorithm (splithalf, pca2, lum-split) dominates.** No
   pre-scaler on the 4000px source can meaningfully change what arrives in a 2×2
   cell covering 25×37 source pixels. The real leverage is in how those 4 pixels
   are used to select fg/bg.

### Remaining pixel-art algorithms (open)

| Algorithm | Likely benefit | Notes |
|-----------|---------------|-------|
| **hq2×/hq3×** | Low — same photo limitation as EPX | Uses luminance threshold not exact equality |
| **xBR** | Low–medium for high-contrast images | More pattern matching; harder to implement |
| **cleanEdge** (Pixorama) | Medium — targets sub-pixel edge orientation | Would help if we scale 2× before downscale |
| **OmniScale** | Potentially high | Neural model; needs weights |
| **Bilateral filter pre-pass** | Medium — edge-preserving smooth | Gives cell encoder cleaner fg/bg signal |
| **Edge-snap cell encoder** | High — operates at the right scale | See Phase 4b below |

### Phase 4b — Edge-snap cell encoder ✅ Implemented

**Algorithm:** `internal/quadblock/algos.go: compileCellEdgeSnap`. Exposed via `Options.EdgeSnap bool`.

For each 2×2 cell (pixels: UL, UR, LL, LR at positions (±½, ±½) from cell centre):
1. Compute BT.709 luma per quadrant; compute `gx = (right col sum) - (left col sum)`, `gy = (bottom row sum) - (top row sum)`
2. If `gx²+gy²<64` (flat cell) → fall back to `compileCellDiameter`
3. With `a=gx+gy`, `b=gx-gy`, the dot product of each pixel position with the gradient is `{UL:−a, UR:+b, LL:−b, LR:+a}` (sign only, factor of ½ dropped)
4. Pixels with positive dot → fg group; negative → bg group; zero → resolved by `twoColorCell` nearest-colour
5. Average each group's colour → fg/bg pair; call `twoColorCell(pixels, fg, bg)` for final mask

**Interactive cycle:** R-key cycle includes `quad/edge-snap` (id=12) and `quad/edge-snap+ambig` (id=13).

**Benchmark results vs baselines:**

| Variant | SSIM | Blk | Edge | Notes |
|---------|------|-----|------|-------|
| quad/splithalf | **0.838** | 0.830 | 0.976 | current best overall |
| **quad/edge-snap** | 0.831 | 0.811 | **0.987** | beats splithalf on Edge (+0.011) |
| **quad/edge-snap+hb3** | 0.831 | 0.811 | **0.987** | same scores; hb3 fallback adds no value |
| quad/edge-snap+ambig | 0.831 | **0.913** | 0.840 | ambig blending wins Blk but loses Edge |
| halfblock | 0.825 | 0.760 | 0.995 | NN halfblock still wins Edge outright |
| quad/pca2 | 0.823 | 0.797 | 0.989 | close competitor to edge-snap |

**Key findings:**

1. **Edge-snap improves edge continuity over splithalf.** Edge score goes from 0.976 → 0.987 — a meaningful improvement. This confirms the gradient-direction split is more geometrically accurate than row-average colours for edge cells.

2. **SSIM cost is small but real.** SSIM drops from 0.838 → 0.831 (-0.007). The spatial grouping occasionally picks a suboptimal fg/bg pair for non-edge cells where the gradient is noise rather than structure. The diameter fallback (flat-cell threshold) reduces but doesn't eliminate this.

3. **PCA2 is the nearest neighbour.** `quad/pca2` scores (0.823, 0.797, 0.989) — PCA2 finds the same gradient direction as edge-snap when color variance aligns with luminance gradient. Edge-snap wins by using spatial position directly rather than color-variance projection.

4. **HalfblockThreshold fallback adds nothing here.** `edge-snap+hb3` scores identically to `edge-snap` — the flat-cell diameter fallback already handles degenerate cases; the halfblock threshold redundancy is not needed.

5. **`+ambig` combination:** Blending smooths block artifacts (Blk=0.913) but at a significant Edge cost (0.840). Same trade-off seen in other blend-mode combinations.

**Implementation note:** `compileCellEdgeSnap` falls back to `compileCellDiameter` for flat cells (gradient magnitude² < 64) and for degenerate cases where all pixels end up on the same side. This means edge-snap is strictly better than or equal to diameter for most cells.

---

## Phase 5 — Composite Score + Automatic Mode Selection (partial)

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

Reference: pyramid downscale, 80×40 terminal, 3 sample images.
All three metrics implemented; tpl vars `ssim`, `blockiness`, `edge_cont` live in hint bar.

```
variant              SSIM   Blk    Edge
─────────────────────────────────────────
quad/splithalf       0.838  0.830  0.976  ← best overall quad
quad/splithalf-nb    0.837  0.833  0.981
quad/hb≥3            0.834  0.830  0.971
quad/lum+ambig       0.833  0.912  0.838  ← best Blk, but loses Edge
quad/pca2+ambig      0.833  0.913  0.837
halfblock            0.825  0.760  0.995  ← best Edge, lowest Blk
quad/pca2            0.823  0.797  0.989
quad/lum-split       0.822  0.796  0.989
quad/blend-wide      0.744  0.903  0.664  ← worst Edge, good Blk
quad/default         0.749  0.739  0.971
```

Key finding: **No single algorithm dominates all three metrics.** `splithalf`
wins the SSIM+Edge trade-off for photos. Blend modes win Blk but lose Edge.
This motivates a composite score (Phase 5) and per-image automatic selection.

---

## Open Questions

- Phase 5 tpl vars `ssim`, `blockiness`, `edge_cont` are already live; composite
  score formula and weights still TBD.
- OmniScale requires a neural model — do we ship weights or just document it?
- For `RenderToImageN` (Phase 3), should N be configurable at runtime (settings)
  or only used in the benchmark test?
- Blend modes show the SSIM/Blk/Edge trade-off clearly — should the R-key cycle
  expose a `lum+ambig` mode for users who prefer smoother block transitions?
