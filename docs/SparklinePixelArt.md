# Sparkline Pixel Art — Rendering Algorithms & Verification

This document captures the architecture, design decisions, optimal-split algorithm, scanning traversal, and testing harness for the **Cati** sparkline rendering mode.

---

## 1. Overview & Modes

Sparkline mode displays scalar gradients and pixel grids in the terminal by
mapping each terminal cell's `4×8` source block to the Unicode glyph and two
colours that minimise reconstruction error.

Cati provides two sparkline-family modes:

| Mode | Visual Representation | Growth Direction | Character Set |
| :--- | :--- | :--- | :--- |
| `Vertical` (`spark/vert`) | ` ▂▃▄▅▆▇█` | Bottom-to-Top (Upward) | `U+2581` – `U+2588` |
| `Quad` (`spark/quad`) | vertical spark blocks plus `▘▝▖▗▀▄▌▐▚▞▛▜▙▟█` | Best 2D mask | fractional block + quad block candidates |

`spark/quad` is the only spark mode currently exposed in the main interactive
render-mode cycle. `spark/vert` remains in the library and test suite as a
useful scalar baseline.

### Removed Modes
Earlier versions included `spark/upper`, `spark/right`, and `spark/left`.
`spark/upper` and `spark/right` were redundant foreground/background inversions.
`spark/left` was removed because it produced weak visual results under the
current `4×8` sparkline geometry. Horizontal 1/8 blocks need an `8×8` base grid
to be represented as cleanly as vertical eighths.

---

## 2. The Optimal Split Algorithm

To render a terminal cell, Cati analyzes its corresponding source pixel block of size $W_c \times H_c$. In `spark/vert`, it selects the character index `bestK` (0..7) and colors (`barColor` and `emptyColor`) that minimize the Total Squared Error (SSE) between the reconstructed cell and the source pixels.

For each possible split level `ci` (0 to 7):
1. **Division**: The cell's pixels are split into two regions: the "bar" (covering $\frac{ci+1}{8}$ of the cell) and the "empty" space (covering the remaining $\frac{7-ci}{8}$).
2. **Color Averaging**: The average RGB color of the bar region becomes the candidate `fgAvg`, and the average RGB color of the empty region becomes the candidate `bgAvg`.
3. **Error Calculation**: The sum of squared Euclidean distances in RGB space is computed between each pixel in the block and its assigned region's average color:
   $$\text{SSE} = \sum_{p \in \text{bar}} (p - fgAvg)^2 + \sum_{p \in \text{empty}} (p - bgAvg)^2$$
4. **Minimization**: The level `ci` yielding the lowest SSE is chosen as `bestK`.

### `spark/quad` Candidate Masks

`spark/quad` generalizes the same SSE idea from one-dimensional split levels to
two-dimensional candidate masks. For each terminal cell it:

1. Evaluates the vertical sparkline masks.
2. Evaluates quad, half, full, and space masks on the same `4×8` block.
3. Averages source pixels inside the mask to get foreground colour.
4. Averages source pixels outside the mask to get background colour.
5. Reconstructs the block and selects the rune with the lowest SSE.

Quad candidates are upsampled to `4×8`: each quadrant covers a `2×4` rectangle.
This keeps `spark/quad` in the sparkline geometry family and avoids changing the
pure `quadblock` renderer.

### Tiebreaker: prefer non-splitting characters

When two candidates have equal primary SSE, a secondary tiebreaker is applied
to avoid artifacts on solid-colour regions.

**Split penalty** is `0` if at most one of (FG colour, BG colour) would emit an
ANSI colour sequence (i.e. at least one region is transparent / empty), and `1`
if both regions need a colour sequence. For a fully opaque solid-colour block:

- `█` (full block): the background region is empty (`bgN = 0`) → `bgAvg.A = 0`
  → only FG sequence needed → `splitPenalty = 0`
- `▁`–`▇` (partial vertical): both regions are opaque → `splitPenalty = 1`

So `█` wins all ties on uniform blocks, producing clean single-colour output
with one sequence per cell instead of two. For mixed blocks the penalty is
irrelevant because SSE differs.

**Transparent-pixel cost** is a separate primary-tier mechanism: any transparent
source pixel that falls inside a coloured region accumulates
`transparentPixelCost = 3 × 255²` per pixel added to the SSE. This forces
candidates that extend colour into transparent rows to lose to candidates that
leave those rows empty, overriding what a pure RGB SSE would prefer.

### Half-cell fit: where the transparent rows come from

A terminal char is two stacked half-cells (`CellH/2` px each). When the
aspect-preserving scaled height ends mid-cell, `imgutil.FitDims` snaps the
partial last row to the **nearest half-cell boundary** so it maps onto a
representable glyph:

- **nearer a top half-cell** → keep `CellH/2` rows of content and append exactly
  `CellH/2` transparent rows (`extH = CellH/2`). The last char renders as a clean
  upper-half block `▀` — FG = content colour, BG region fully transparent so no
  BG sequence is emitted.
- **nearer a full cell** → round up to a full content cell (`extH = 0`); the last
  char is an ordinary content row.

Snapping to the *nearest half-cell* (rather than keeping the raw remainder and
padding `CellH − rem` transparent rows) is essential: a mid-cell remainder such
as 6 content + 2 transparent rows matches no block glyph, so the selector would
fall back to quadrant/diagonal chars (`▌ ▚ ▘`) whose colour bleeds into the
transparent area — a garbled bottom row in `RenderOpts`. The snap guarantees
`extH ∈ {0, CellH/2}`, upholding the half-char transparency invariant.

---

## 3. Pixel Scanning Traversal & Pitfalls

The legacy 1D split logic requires that the pixel array passed to the error minimization function is segmented along the split line. This introduces a critical traversal requirement:

*   **Vertical Spark Mode (`Vertical`)**: Must scan pixels in **row-major** order (row 0, row 1, ..., row H-1). A horizontal split boundary in 1D then maps to a horizontal boundary dividing the top and bottom rows of the cell block.
*   **Quad Combo Mode (`Quad`)**: Uses explicit 2D masks instead of scan-order-dependent splits.
*   **Cropped image bounds**: Interactive panning passes cropped `SubImage`
    values into the renderer. These images may have non-zero `Bounds().Min`.
    Sparkline sampling must add `b.Min.X` / `b.Min.Y` when deriving `x0`,
    `x1`, `y0`, and `y1`; sampling from relative `(0,0)` coordinates reads
    out-of-bounds black pixels and makes the background appear to pan while the
    image stays pinned.
*   **Rendering reconstruction**: `sparkline.RenderToImage` must share the same
    cell selection and mask semantics as `RenderOpts`. The app uses it for SSIM
    and other quality metrics, so changing glyph masks requires updating both
    ANSI rendering and image reconstruction together.
*   **Display-size contract**: The `4×8` sparkline footprint is a renderer-local
    glyph grid, not permission to shrink the visible terminal cell rectangle.
    Interactive viewport construction expands spark crops to the footprint
    required by the shared `src px/cell` zoom model, then validates the emitted
    cell size. A small `32×32` source at fit/1:1 must render as `32×16` cells,
    not silently become `8×4` cells just because one spark cell analyzes a
    `4×8` block.

> [!WARNING]
> Reintroducing horizontal 1/8 block modes under `4×8` geometry will be
> approximate. Use an `8×8` spark-family geometry first if exact horizontal
> eighths become important.

---

## 4. Verification & The Test Helper Suite

The `testhelper` package (`internal/sparkline/testhelper/`) provides automated
validation and visualization of all Cati renderers. It exposes three generator
functions that create source images on the fly so no static binaries need to be
committed to the repo for these test cases.

### Generator functions

| Function | What it produces | Located under |
|---|---|---|
| `GenerateGradients` | Horizontal + vertical blue→yellow gradients at 20×20, 4×4, 2×2, 1×1 | `testdata/demo_horiz_NxN/`, `testdata/demo_verti_NxN/` |
| `GenerateFixtures` | Solid-red 4×4 regression fixture | `testdata/solid_red_4x4/` |
| `GenerateGeometrics` | Four 20×20 geometric images (see below) | `testdata/demo_*_20x20/` |

**Geometric images** (`GenerateGeometrics`):

| Subfolder | Description | Colours |
|---|---|---|
| `demo_diag_20x20` | 45° diagonal split (top-left vs bottom-right) | red / blue |
| `demo_circle_20x20` | Filled disc, radius 8, centred at (9.5, 9.5) | yellow / blue |
| `demo_checker_20x20` | Checkerboard with 4×4 px cells | red / blue |
| `demo_cross_20x20` | 4-pixel-wide cross centred on image | yellow / blue |

Pure saturated colours give each algorithm unambiguous ground truth at every
cell boundary: a correct renderer must produce the source colour with no bleed
across a hard edge.

### Golden comparison

`TestGoldenRenders` (in `cmd/golden_render_test.go`) runs every combination of
`(source image, char width, algorithm)` and compares against a stored PNG.
Goldens are stored at **4×8 px/char** — the sparkline native resolution — so all
three algorithms (halfblock 1×2, quad 2×2, spark 4×8) are upscaled to the same
common resolution before comparison.

`TestCLIRender` (in `cmd/cli_render_test.go`) does the same for ANSI terminal
output, storing `.ansi` golden files.

Run with `-update` to regenerate all goldens:
```bash
go test ./cmd/... -update
```

### Interactive demo table

```bash
make demo-widths
make demo-darth
make demo-solder DEMO_WIDTH=80 DEMO_STEPS=3
```

Runs `scripts/demo_widths.go` (build-tag `ignore`, excluded from normal builds)
and prints demo renders in terminal tables. Multi-image runs group by render
mode with one image per column. Single-image runs group by image with
`halfblock`, `quad/splithalf`, and `spark/quad` side by side. `-w` selects the
maximum render width and `-n` selects how many 80% downscale steps to show
(default 2). Useful for a quick visual sanity check of all render modes after
algorithm changes.
