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

## Package structure

```
internal/quadblock/
  render.go       — quadChar table, compileCell, ScaleToFit, Render
  render_test.go  — unit tests: char table, mask, quantisation, neighbour lookup
  show_test.go    — visual test: go test -v -run TestShowImages
```

`ScaleToFit` and `Render` are the public surface; all internals are unexported.
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
