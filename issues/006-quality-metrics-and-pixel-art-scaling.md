# 006 — Quality Metrics: Blockiness, Edge Continuity, and Pixel-Art Scaling

**Status:** 🔄 In Progress (Phase 5 remaining)  
**Ref:** `cmd/metrics.go`, `cmd/ssim.go`, `internal/metrics/`, `internal/pixelart/`

---

## Motivation

SSIM alone cannot distinguish blocky artefacts from edge smearing. Multiple
metrics are needed for algorithm comparison and eventual auto-selection.

## Phase Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Blockiness score (Sobel edge excess at cell boundaries) | ✅ Done |
| 2 | Edge continuity score (weighted recall of Sobel edges) | ✅ Done |
| 3 | Higher-res cell comparison up to 8×8 | ❌ Not started |
| 4 | Pixel-art scaling algorithms (EPX, sharpen, edge-snap) | ✅ Done |
| 4b | Edge-snap cell encoder | ✅ Done |
| 5 | Composite score + auto mode selection | 🔴 Open |

## Key Findings

- **No single algorithm dominates all three metrics.** splithalf wins SSIM+Edge trade-off for photos; blend modes win Blk but lose Edge.
- **EPX/Scale2x has no effect on photos** — exact-pixel-equality condition never fires at photographic gradients.
- **Edge-snap improves Edge score** (0.976 → 0.987) vs splithalf, with small SSIM cost (−0.007).
- **Halfblock scores Edge ≈ 0.995** — NN scaling preserves edges well; perceived detail loss is colour quantisation, not edge erasure.

Detailed benchmark data moved to `docs/Quality.md` (to be created).

## Open Items

- Phase 5 composite score formula and weights TBD. Tpl vars live (`ssim`, `blockiness`, `edge_cont`).
- Should R-key cycle expose a `lum+ambig` mode for users who prefer smoother blocks?
- Phase 3 (8×8 cell comparison): cap N at 8 for bounded compute; `RenderToImageN` needed in halfblock/quadblock.
