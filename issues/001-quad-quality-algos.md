# 001 — Quad Image Quality: Advanced Rendering Algorithms

**Status:** 🔄 In Progress  
**Ref:** `internal/quadblock`, `docs/QuadPixelArt.md`

---

## Problem

The basic quad renderer maps 2×2 source pixels directly to a terminal cell.
When a cell has 3+ distinct colours, quantisation to two colours causes visible
artefacts — colour bleeding, harsh edges, and loss of fine detail.

---

## Algorithms

### ✅ Phase 1 — Implemented

| Option | Description |
|--------|-------------|
| `HalfblockThreshold` | Fall back to halfblock `▀`/`▄` when best-pair coverage < N |
| `Blend: BlendAlways` | Always blend each sub-pixel with its 8 pixel neighbours (4:2:1 weights) |
| `Blend: BlendAmbiguous` | Same 3×3 blend, but only triggered when the cell has 3+ distinct colours |
| `Blend: BlendAmbiguousWide` | Same trigger, 5×5 neighbourhood (2 px radius) |
| `SplitHalf` | Fix colours from halfblock row-averages; apply quad mask for sub-cell precision |
| `SplitHalfNeighbors` | `SplitHalf` + try neighbour cell fg/bg colours as bg candidate (picks lowest quantisation error) |
| `LumSplit` | Split cell by luminance: bright pixels → fg group, dark → bg group; colour each group's average |
| `ColorReduction` | Pre-reduce image to ANSI256 / ANSI16 / Gray8 / Gray16 / Gray64 palette |

**Observation (2026-06-24):** `BlendAmbiguous` / `BlendAmbiguousWide` produce too much blur.
`SplitHalf` and `SplitHalfNeighbors` give the cleanest results so far.  Halfblock is still most
pleasant because square pixels are easier on the eye.

---

### 💡 New idea — Grayscale-as-Base + Colour Overlay

Use grayscale structure to drive the luminance split, then apply colour:

1. Convert each sub-pixel to luminance (`L = 0.299R + 0.587G + 0.114B`).
2. Split the 4 sub-pixels at their mean luminance → bright group (fg) and dark group (bg).
3. Average the *original* (non-gray) pixel colours within each group → fg colour and bg colour.
4. Apply the quad mask as usual.

This is effectively `LumSplit` (already implemented).  Possible extensions:
- **Hue-preserving grayscale**: use perceptual gray (CIE L\*) instead of BT.601 luma for the split threshold.
- **Chroma boost**: after the split, boost the saturation of the averaged colours to compensate for the averaging loss.
- **Two-pass**: use `LumSplit` structure + `SplitHalf` colours (combine both ideas).

---

### 🔲 Phase 2 — Open

#### Blend: Cell-Neighbour Colour Anchoring
When a cell is ambiguous, blend current sub-pixel values towards the fg/bg
colours of already-rendered left/above cells (configurable weight `0..1`).
Stronger than the existing `continuity` bonus in `pickBestPair`; effectively
propagates decided colours into ambiguous regions.

```
anchoredPixel = lerp(rawPixel, neighborColor, anchorWeight)
```

#### SplitHalf: Neighbour-Guided 2nd Colour (option b)
In `SplitHalf` mode, instead of using the bottom (or top) row average as the
2nd colour, sample from fg/bg of the best-matching neighbouring halfblock cell.
This can produce sharper colour boundaries at region transitions.

#### Dithering
- **Error diffusion** (Floyd–Steinberg): propagate quantisation error to right/
  lower neighbours; reduces banding but adds noise texture.
- **Ordered dithering** (Bayer matrix 4×4 or 8×8): deterministic, no error
  propagation; better for animation.
- **Blue-noise dithering**: perceptually optimal noise distribution.

Apply timing (each has distinct visual effect):
- Before resize → dither at full source resolution, scale down
- After resize → dither at display resolution
- After colour reduction → dither in reduced palette space

#### When to Apply Colour Reduction
| Timing | Effect |
|--------|--------|
| Before resize | Palette snap before downscale; colours bleed during resize |
| After resize | Colours blended during resize; palette snap at display size |
| Before + after | Two-pass: coarse reduction then fine snap |

#### Interactive Parameter Menu (blocked: requires main cati code)
Once the quad mode is wired into the interactive viewer:
- `<m>` toggles a params panel below the image
- `<tab>` cycles through `Options` fields
- `<left>` / `<right>` changes enum values / increments integers
- Shows live re-render on each change

---

## Design Notes

### Palette matching
Current: nearest Euclidean distance in linear RGB.  
Better: CIE L\*a\*b\* colour space for perceptual uniformity (future).

### Blend radius trade-offs
| Radius | Kernel | Effect |
|--------|--------|--------|
| 1 | 3×3 | Mild smoothing; preserves most edges |
| 2 | 5×5 | Stronger; may blur fine lines |
| ≥3 | 7×7+ | Requires explicit handling of cell-boundary source pixels |

### SplitHalf rationale
Row averages are more representative than individual pixels at colour boundaries
where a single quad pixel may be noise. The quad mask then restores sub-cell
precision on top of the stable colour base.

---

## Test Coverage
All options are exercised visually in `TestShowImages` (6 variants per row).
Unit tests cover `compileCell` paths for each mode.
