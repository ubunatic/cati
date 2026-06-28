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

> [!WARNING]
> Reintroducing horizontal 1/8 block modes under `4×8` geometry will be
> approximate. Use an `8×8` spark-family geometry first if exact horizontal
> eighths become important.

---

## 4. Verification & The Test Helper Suite

The [testhelper](file:///home/uwe/projects/cati/internal/sparkline/testhelper/testhelper.go) package provides automated validation and visualization of all Cati renderers:

*   **Base Gradient Generation**: Generates horizontal and vertical gradients from Blue `(0, 0, 255)` to Yellow `(255, 255, 0)` at scales of `20x20`, `4x4`, `2x2`, and `1x1`.
*   **Visual Reconstruction**: Renders the gradients using all active algorithms (sparkline modes, `halfblock`, and `quadblock`) and outputs visual PNG reconstructions to subdirectories in `testdata/` (e.g. `testdata/demo_horiz_20x20/`).
*   **Embedded Metadata**: Custom text chunks (`tEXt`) are injected into the generated PNG byte streams to identify the rendering `Algorithm` and `Parameters` (e.g., `outCols`, `outRows`, `KMeans`) used.
*   **Golden Comparison**: [testhelper_test.go](file:///home/uwe/projects/cati/internal/sparkline/testhelper/testhelper_test.go) runs pixel-for-pixel comparisons against these golden files to prevent regressions. You can update golden expectations via:
    ```bash
    go test ./internal/sparkline/testhelper -args -update
    ```
