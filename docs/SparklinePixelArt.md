# Sparkline Pixel Art — Rendering Algorithms & Verification

This document captures the architecture, design decisions, optimal-split algorithm, scanning traversal, and testing harness for the **Cati** sparkline rendering mode.

---

## 1. Overview & Orientation Modes

Sparkline mode displays scalar gradients and pixel grids in the terminal by mapping image regions to fractional Unicode block characters. 

Cati provides two orientation modes, each with 1/8th resolution:

| Mode | Visual Representation | Growth Direction | Character Set |
| :--- | :--- | :--- | :--- |
| `LowerHorizontal` (`spark/lower`) | ` ▂▃▄▅▆▇█` | Bottom-to-Top (Upward) | `U+2581` – `U+2588` |
| `LeftVertical` (`spark/left`) | `▏▎▍▌▋▊▉█` | Left-to-Right (Rightward) | `U+258F` – `U+2589` |

### Removal of Redundant Modes
Earlier versions of Cati included `UpperHorizontal` (`spark/upper`) and `RightVertical` (`spark/right`) modes. These were simulated by swapping foreground and background colors on the lower and left block characters. However, they were removed due to redundancy (the same gradients can be clearly represented by cycling between `spark/lower` and `spark/left`) and the index-mapping complexity they introduced.

---

## 2. The Optimal Split Algorithm

To render a terminal cell, Cati analyzes its corresponding source pixel block of size $W_c \times H_c$. It selects the character index `bestK` (0..7) and colors (`barColor` and `emptyColor`) that minimize the Total Squared Error (SSE) between the reconstructed cell and the source pixels.

For each possible split level `ci` (0 to 7):
1. **Division**: The cell's pixels are split into two regions: the "bar" (covering $\frac{ci+1}{8}$ of the cell) and the "empty" space (covering the remaining $\frac{7-ci}{8}$).
2. **Color Averaging**: The average RGB color of the bar region becomes the candidate `fgAvg`, and the average RGB color of the empty region becomes the candidate `bgAvg`.
3. **Error Calculation**: The sum of squared Euclidean distances in RGB space is computed between each pixel in the block and its assigned region's average color:
   $$\text{SSE} = \sum_{p \in \text{bar}} (p - fgAvg)^2 + \sum_{p \in \text{empty}} (p - bgAvg)^2$$
4. **Minimization**: The level `ci` yielding the lowest SSE is chosen as `bestK`.

---

## 3. Pixel Scanning Traversal & Pitfalls

The 1D split logic requires that the pixel array passed to the error minimization function is segmented along the split line. This introduces a critical traversal requirement:

*   **Horizontal Modes (`LowerHorizontal`)**: Must scan pixels in **row-major** order (row 0, row 1, ..., row H-1). A horizontal split boundary in 1D then maps to a horizontal boundary dividing the top and bottom rows of the cell block.
*   **Vertical Modes (`LeftVertical`)**: Must scan pixels in **column-major** order (column 0, column 1, ..., column W-1). A 1D split boundary then maps to a vertical boundary dividing the left and right columns of the cell block.

> [!WARNING]
> Scanning pixels in row-major order for vertical modes causes a structural mismatch. The algorithm splits the block horizontally but maps it to a vertical left block character. This mismatch causes severe vertical/horizontal stripe artifacts (comb lines).

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
