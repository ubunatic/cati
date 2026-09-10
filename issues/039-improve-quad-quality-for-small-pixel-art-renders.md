# 039 — Improve quad quality for small pixel-art renders

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Quality
**Related**: [#001](001-quad-quality-algos.md), [#006](006-quality-metrics-and-pixel-art-scaling.md), [#013](013-shared-analysis-grid-for-halfblock-and-quadblock-convergence.md)

---

## Problem

For small pixel-art and logo renders, `quad` can look worse than `half` even
though it has higher nominal spatial resolution. The issue is especially
visible in the cati/emojig demo at small widths, where quad produces noisy
partial glyph patterns instead of the stable shapes produced by halfblock.

## Measured facts

- `half` uses a `1x2` pixel cell geometry; `quad` uses `2x2` input pixels per
  terminal cell.
- The configured `quad` mode is `quad_split_half`, not a generic four-color
  renderer.
- Quad still reduces each cell to two colors, then makes local quadrant-mask
  decisions.
- Local mask decisions can disagree with the row-average color split,
  especially around antialiased edges or near-equal colors, producing noisy
  partial glyphs such as `▝`, `▘`, `▛`, and `▜`.
- Halfblock averages more aggressively, so it can look cleaner at widths where
  quad's extra detail is below a useful visual scale.

## Reproduction and comparison

Run:

```sh
./cati modes -w 12
./cati --mode half -w 12 assets/cati_0001.png
./cati --mode quad -w 12 assets/cati_0001.png
```

Capture comparable output for widths 8 through 20. Compare default quad,
`quad_split_half`, and halfblock using both visual review and image metrics;
the small-logo case must be tested separately from the larger SSIM benchmark.

## Experiments

Evaluate, with reproducible fixtures and tuned parameters:

- a conservative halfblock fallback for unstable quad cells using the existing
  `HalfblockThreshold` capability;
- ambiguity blending or a stability rule for near-equal color/mask choices;
- alternative quad color/mask variants, including a comparison against the
  current split-half behavior.

## Acceptance criteria

1. A test fixture covers the cati/emojig-style small pixel-art case at widths
   8–20 and records baseline output for half and quad.
2. The selected change reduces visible noise or improves the agreed image
   metric without regressing larger-image quad quality or existing goldens.
3. Tests verify threshold/ambiguity behavior deterministically, including
   near-equal colors and hard edges.
4. Results document when halfblock fallback is selected and preserve the
   existing user-facing mode semantics.

## Resolution

The user-facing `quad` mode now enables `HalfblockThreshold: 2` alongside
`SplitHalf`. Ambiguous 3+ colour cells that cannot exactly represent at least
two source pixels use the stable halfblock fallback; clean two-colour edges
retain quadrant precision. The library behavior remains opt-in through the
same `Options` fields, and the small-logo width matrix plus near-equal,
hard-edge, and threshold-zero tests cover the decision. Quad golden updates
are expected where this conservative policy changes the reconstructed cell.
