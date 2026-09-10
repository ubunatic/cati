# 038 — Investigate blocky sextant rendering at small widths

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Quality
**Related**: [#006](006-quality-metrics-and-pixel-art-scaling.md), [#020](020-sextant-column-mask-nul-glyph.md), [Sparkline Pixel Art](../docs/SparklinePixelArt.md)

---

## Problem

The `six`/sextant mode looks unusually blocky in the small logo demo, notably
around width 12. Some of this is an inherent limit of the representation, but
the current selector and terminal glyph compatibility may add avoidable visual
degradation.

## Measured facts

- `six` uses a `2x3` pixel cell geometry.
- Each terminal cell expresses six binary on/off regions and only limited
  color information (the renderer reduces the cell to a small number of
  colors).
- The output uses Unicode sextant glyphs; terminal font coverage and fallback
  behavior can change the apparent weight and shape of those regions.
- Sextant selection scores all 64 six-region masks, including the two column
  aliases and empty/full special cases.

## Reproduction and diagnosis

Run from the repository root:

```sh
./cati modes -w 12
./cati --mode six -w 12 assets/cati_0001.png
printf '🬀🬁🬂🬃🬄🬅\n'
```

Compare widths 8, 12, 16, and 20 in at least two terminals/fonts while
capturing the same ANSI output. Record the selected masks, source dimensions,
rendered dimensions, and whether the terminal has native U+1FB00 glyphs or is
using fallback glyphs. Separate expected coarse-resolution blockiness from
differences caused by mask selection or font rendering.

## Scope

Do not promise to remove the intrinsic coarse `2x3`/binary representation
limit. Investigate actionable improvements such as better mask selection,
stable handling of near-ties, and an explicit compatibility/fallback policy
for terminals without usable sextant glyphs.

## Acceptance criteria

1. A reproducible diagnostic records the visual and measured behavior at
   widths 8–20 and distinguishes renderer output from terminal/font effects.
2. Tests cover sextant mask selection and any new tie, fallback, or
   compatibility policy.
3. The chosen improvement is demonstrated by visual or image-quality
   regression tests without degrading existing sextant goldens.
4. Documentation explains the remaining representation limit and how users
   can diagnose font fallback.

## Previous Attempt (Reopened)

The previous attempt made the native sextant path score 60 native masks, but was
reopened after visual regression. It omitted the supported `▌`/`▐` masks and
empty/full masks, and transparent pixels had no overpaint penalty. The cati
logo consequently showed repeated bottom-edge protrusions. `RenderToImage`
also skipped transparent pixels, so image tests could hide what ANSI output
painted. These defects must be fixed before #038 can close.

## Reopened Plan

- Score all 64 six-region masks, including empty/full and the `▌`/`▐` aliases.
- Penalize candidate coverage of transparent pixels and make reconstruction
  reflect emitted glyph coverage.
- Add small-logo width 8–20 regression checks for transparent-edge overpaint,
  supported vertical splits, and ANSI/image consistency.
- Re-run both Go decoder-family golden suites and document any remaining
  representation or font-fallback limitations.

## Resolution

The sextant selector now evaluates all 64 masks deterministically and applies a
large penalty when the emitted ANSI cell would paint transparent source
regions. Empty/full cells and the `▌`/`▐` aliases are included. Native image
reconstruction now uses the same emitted-coverage model as ANSI output, so
transparent-edge overpaint is tested rather than hidden. Focused tests cover
aliases, transparent edges, background escapes, reconstruction, ANSI/image
agreement, and the cati logo across widths 8–20. Sextant goldens were updated
for both supported Go decoder families because the corrected coverage model
changes the rendered native-sextant pixels.
