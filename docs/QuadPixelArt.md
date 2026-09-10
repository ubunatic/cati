---
title: Quad-Block Pixel Art
weight: 50
---

# Quad-Block Pixel Art in the Terminal


This document captures the design, aspect-ratio math, and implementation decisions
for the `internal/quadblock` package, which renders images using Unicode quadrant
block characters (U+2596–U+259F).

---

## Core idea: one cell, four pixels

Unicode quadrant characters divide a terminal cell into a **2×2 pixel grid**:

| Position | Name |
|----------|------|
| UL (upper-left)  | bit 3 — value 8 |
| UR (upper-right) | bit 2 — value 4 |
| LL (lower-left)  | bit 1 — value 2 |
| LR (lower-right) | bit 0 — value 1 |

`fg` colour fills the marked quadrants; `bg` fills the rest.
The same two-colour-per-cell constraint as half-block applies.

### Character lookup table (4-bit mask → rune)

| Mask | Filled | Char |
|------|--------|------|
| 0000 | —           | ` ` (space) |
| 0001 | LR          | `▗` |
| 0010 | LL          | `▖` |
| 0011 | LL+LR       | `▄` |
| 0100 | UR          | `▝` |
| 0101 | UR+LR       | `▟` *(approx — no exact Unicode char)* |
| 0110 | UR+LL       | `▞` |
| 0111 | UR+LL+LR    | `▟` |
| 1000 | UL          | `▘` |
| 1001 | UL+LR       | `▚` |
| 1010 | UL+LL       | `▙` *(approx — no exact Unicode char)* |
| 1011 | UL+LL+LR    | `▙` |
| 1100 | UL+UR       | `▀` |
| 1101 | UL+UR+LR    | `▜` |
| 1110 | UL+UR+LL    | `▛` |
| 1111 | all         | `█` |

**Masks 0101 (UR+LR, "right column") and 1010 (UL+LL, "left column") have no
exact Unicode codepoint.** They are approximated with the nearest Hamming-1
character. This is a known limitation of the Unicode block-element range.

---

## Half-block vs. quad: layout and aspect ratio

### Half-block layout

Half-block characters split each terminal cell **once** — into a top and bottom
half. A 10×10 px image renders as **10 cols × 5 rows**:

```
    0123456789   ← pixel columns (= terminal columns)
  0 ▀▄▀▄▀▄▀▄▀▄  ← terminal row 0 covers pixel rows 0+1
  2 ▀▄▀▄▀▄▀▄▀▄  ← terminal row 1 covers pixel rows 2+3
  4 ▀▄▀▄▀▄▀▄▀▄
  6 ▀▄▀▄▀▄▀▄▀▄
  8 ▀▄▀▄▀▄▀▄▀▄
```

- 1 terminal col = 1 image pixel wide
- 1 terminal row = 2 image pixels tall

Terminal cells are **1:2 (W:H)** in screen aspect.
A 10-col × 5-row cell grid is `10·W : 5·2W = 10W : 10W` → **1:1 ✓**
The image appears with correct proportions.

### Quad-block layout (naïve)

Quad characters split each terminal cell **twice** — into a 2×2 grid.
The same 10×10 px image renders as only **5 cols × 5 rows**:

```
    02468        ← pixel columns (every other, since 2 px per col)
  0 ▞▞▞▞▞        ← terminal row 0 covers pixel rows 0+1
  2 ▞▞▞▞▞        ← terminal row 1 covers pixel rows 2+3
  4 ▞▞▞▞▞
  6 ▞▞▞▞▞
  8 ▞▞▞▞▞
```

- 1 terminal col = 2 image pixels wide
- 1 terminal row = 2 image pixels tall

Screen aspect of the 5-col × 5-row cell grid: `5·W : 5·2W = 5W : 10W` = **1:2**
The image is **horizontally squeezed** (or equivalently, vertically stretched).

---

## The 2× horizontal stretch correction

Each quad pixel occupies `cell_width/2 × cell_height/2` on screen.
Since `cell_height ≈ 2·cell_width`, each quad pixel is `cell_width/2 × cell_width`
— a **1:2 rectangle** (narrow and tall).

To make source pixels appear **square** in the rendered output, the pixel image
fed to `Render` must be **2× wider** than the source image:

| Source image | Naïve quad pixels | After 2× stretch |
|-------------|------------------|-----------------|
| 10×10 px    | 5 cols × 5 rows (1:2 screen) | 10 cols × 5 rows (1:1 screen ✓) |
| W×H px      | W/2 cols × H/2 rows | W cols × H/2 rows ✓ |

This matches the half-block output: both render a square image into N cols × N/2
rows, giving a 1:1 screen aspect.

### How `ScaleToFit` implements the correction

```go
// Treat the source as 2× wider when computing the scale factor.
stretchedW := srcW * 2
targetW, targetH := stretchedW, srcH

if maxW > 0 && targetW > maxW {
    targetH = srcH * maxW / stretchedW
    targetW = maxW
}
if maxH > 0 && targetH > maxH {
    targetW = stretchedW * maxH / srcH
    targetH = maxH
}
// ScaleNN upscales: a 10×10 source → 20×10 target (fits cols=10, rows=5).
return halfblock.ScaleNN(img, targetW, targetH)
```

Upscaling is **intentional** — without it a small source image would render with
the 1:2 pixel distortion regardless of the col/row limits.

---

## Two-colour constraint and neighbour-aware quantisation

Each terminal cell has exactly one fg and one bg colour. When a 2×2 pixel block
contains **more than two distinct colours**, the renderer must quantise to two.

### Scoring algorithm (`pickBestPair`)

For every candidate colour pair `(ca, cb)`:

```
score = coverage × 4 + continuity
```

- **coverage**: number of the 4 pixels that exactly match `ca` or `cb` (0–4)
- **continuity**: count of how many of those colours already appear as fg/bg in
  the **left** or **above** neighbour cell (0–4)

Coverage is weighted 4× so exact matches dominate, but continuity breaks ties,
keeping colour transitions smooth across cell boundaries.

The higher-count colour of the winning pair becomes `fg`; the other becomes `bg`.

---

## Quality rendering variants

The `Options` struct controls quality trade-offs available to the caller.
Pre-processing steps (colour reduction) are applied to the scaled image before
calling `RenderOpts`.

### Rendering options (`Options`)

| Field | Type | Effect |
|-------|------|--------|
| `HalfblockThreshold` | `int` | Fall back to `▀`/`▄` when exact coverage < N; also stabilizes ambiguous `SplitHalf` cells |
| `Blend` | `BlendMode` | Neighbourhood pixel blending (see below) |
| `SplitHalf` | `bool` | Derive fg/bg from halfblock row-averages; apply quad mask for sub-cell precision |
| `SplitHalfNeighbors` | `bool` | Extends `SplitHalf`: also tries left/above cell colours as bg candidate, picks lowest quantisation error |
| `LumSplit` | `bool` | Split sub-pixels at mean BT.601 luminance; colour each group's average |

### Blend modes

| Constant | Behaviour |
|----------|-----------|
| `BlendNone` | Sample each sub-pixel at its exact center (default) |
| `BlendAlways` | 3×3 weighted blend (4:2:1) for every sub-pixel |
| `BlendAmbiguous` | Same 3×3 blend, but only on cells with 3+ distinct colours |
| `BlendAmbiguousWide` | 5×5 blend (radius 2) on ambiguous cells |

**Practical note (2026-06-24):** `BlendAmbiguous` / `BlendAmbiguousWide` produce
visible blurring on photographic content. `SplitHalf` and `SplitHalfNeighbors`
give the cleanest results. Halfblock mode is perceptually most pleasant because
its 1:1 "square pixels" are easier on the eye than quad's 1:2 sub-pixels.

The user-facing `quad` mode enables `SplitHalf` with `HalfblockThreshold: 2`.
Clean two-colour edges remain quadrant glyphs; unstable cells use a stable
halfblock glyph instead of isolated noisy quadrants. Library callers can select
the same policy with `Options{SplitHalf: true, HalfblockThreshold: 2}`.

### Colour space reduction (`ReduceColors`)

```go
// Apply before ScaleToFit / RenderOpts:
img = quadblock.ReduceColors(img, quadblock.ColorANSI256)
```

| Constant | Palette |
|----------|---------|
| `ColorFull` | 24-bit true colour (no reduction) |
| `ColorANSI256` | ANSI xterm 256: 16 basic + 6×6×6 cube + 24 grays |
| `ColorANSI16` | 16 basic ANSI terminal colours |
| `ColorGray8` | 8-level grayscale (BT.601 luma) |
| `ColorGray16` | 16-level grayscale |
| `ColorGray64` | 64-level grayscale |

Nearest-colour matching uses squared Euclidean distance in linear RGB.
Transparent pixels are preserved.

The renderer also has worker-aware copies of `RenderOpts` and `RenderToImage`.
The serial code remains the baseline implementation; the parallel copies are
called only when the CLI job count is greater than 1, so the current algorithm
behaviour stays pinned while the worker path is exercised separately.

### LumSplit algorithm

For each 2×2 cell:
1. Compute BT.601 luma `L = 0.299·R + 0.587·G + 0.114·B` for each sub-pixel.
2. Compute mean luma as the split threshold.
3. Sub-pixels at or above threshold → **bright group** (fg); below → **dark group** (bg).
4. fg colour = average of original colours in bright group.  
   bg colour = average of original colours in dark group.
5. Build the quad mask as usual.

This is the "grayscale-as-base + colour overlay" approach: luminance drives the
structure; colour is derived from the real pixel values.

---

## Package structure

```
internal/quadblock/
  render.go       — quadChar table, Options, compileCell, ScaleToFit, RenderOpts
  colorspace.go   — ColorReduction type, ReduceColors, palette definitions
  render_test.go  — unit tests: char table, mask, quantisation, neighbour lookup
  show_test.go    — visual test: go test -v -run TestShowImages
```

`ScaleToFit`, `Render`, `RenderOpts`, and `ReduceColors` are the public surface;
all internals are unexported.
The package imports `internal/halfblock` for `ScaleNN` and `LoadImage` (tests).

---

## Known limitations

- **Masks 0101 / 1010** (vertical column patterns) have no exact Unicode
  codepoint. The approximations (`▟` / `▙`) add one extra quadrant.
- **Terminal font support**: quadrant chars (U+2596–U+259F) require a geometric
  font renderer. `foot`, `kitty`, `alacritty`, `wezterm`, `ghostty` are safe.
  `gnome-terminal` and `xterm` may render them incorrectly.
- **Cell aspect ratio assumption**: the 2× stretch assumes a 1:2 (W:H) terminal
  cell. Most modern terminals match this; bitmap fonts or unusual DPI may differ.
