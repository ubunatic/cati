# Cati Glossary & Metric Taxonomy

This glossary defines standard terminology used across Cati's rendering pipelines, spec registry, quality assessment tools, and CLI commands.

---

## 1. Error Types & Quality Assessment

When evaluating terminal image rendering, information loss occurs at two distinct stages: downsampling to the cell grid, and discretizing grid pixels into Unicode character shapes. Distinguishing these two error metrics is critical.

### End-to-End Reconstruction Error ($E_{\text{src}}$ / Source-Space Error)
* **Aliases**: Source Reconstruction Error, Total Fidelity Loss, Task Error.
* **Definition**: The visual difference between the **final rendered terminal output** (reconstructed and rasterized on a high-resolution canvas) and the **original high-resolution source image** ($I_{\text{src}}$).
* **Formula**:
  $$E_{\text{src}} = \text{MSE}\big(I_{\text{src}},\, \text{Upscale}_{\text{NN}}(\text{Reconstruction}(\text{TerminalCells}))\big)$$
* **What it measures**: The overall fidelity of the image displayed in the terminal given the user's terminal column/row budget. It combines both **spatial downsampling loss** (aliasing/resolution loss) and **glyph quantization loss**.
* **Behavior**: Modes with finer subpixel grids (e.g. Sextant $2\times 3$, Quad $2\times 2$) achieve **substantially lower error** than low-resolution modes (e.g. Half-block $1\times 2$, Full-block $1\times 1$).

### Representation Error ($E_{\text{fit}}$ / Fitting Error)
* **Aliases**: Quantization Error, Codebook Matching Error, Discretization Error, Approximation Error.
* **Definition**: The difference between the **rendered glyph reconstruction** and the **intermediate downscaled target grid** ($I_{\text{down}}$) configured for that mode's native subpixel dimensions ($W_{\text{sub}} \times H_{\text{sub}}$).
* **Formula**:
  $$E_{\text{fit}} = \text{MSE}\big(I_{\text{down}},\, \text{Reconstruction}(\text{TerminalCells})\big)$$
* **What it measures**: How closely the mode's glyph codebook can reproduce the intermediate downscaled bitmap at that mode's native resolution, ignoring how much detail was discarded during downscaling.
* **Behavior**: Full-block ($1\times 1$) trivially achieves $E_{\text{fit}} = 0\%$ because a $1\times 1$ subpixel cell exactly matches `█` or ` `, whereas a complex 2D codebook with partial coverage (e.g. Sextant $2\times 3$ with missing shapes) may have non-zero fitting residual.

---

## 2. Comparison Summary

| Metric | Target / Reference | Answers the Question | Ranking for `full` ($1\times 1$) vs `six` ($2\times 3$) |
| :--- | :--- | :--- | :--- |
| **End-to-End Error** ($E_{\text{src}}$)<br>`err` in `cati modes` | Original Source Image | *"Which algorithm best displays the original image in my terminal?"* | **`six` wins** (much higher detail, lower error). `full` is worst. |
| **Fitting Error** ($E_{\text{fit}}$)<br>`err_fit` | Downscaled Subpixel Grid | *"How well did the solver fit the glyphs to its own low-res target?"* | **`full` wins (0%)** because $1\times 1$ matching is trivial. |

---

## 3. Spatial & Geometry Concepts

### Terminal Cell
* The fundamental atomic display unit in a text terminal. In standard terminal fonts, a cell typically has an aspect ratio of roughly $1:2$ (width is half of height).

### Subpixel Grid
* The internal partition of a single terminal cell into virtual pixels:
  * **Full Block (`full`)**: $1 \times 1$ subpixel per cell.
  * **Half Block (`half`)**: $1 \times 2$ subpixels per cell (upper/lower halves `▀`, `▄`).
  * **Quadrant Block (`quad`)**: $2 \times 2$ subpixels per cell (`▖`, `▗`, `▘`, `▙`, `▚`, `▛`, `▜`, `▟`).
  * **Sextant Block (`six` / `sextants`)**: $2 \times 3$ subpixels per cell (64 combinatorial subpixel states in Unicode 13.0 Block 1FB00).
  * **Sparkline (`spark` / `bars`)**: $1 \times 8$ vertical fractions per cell (` `, `▂`, `▃`, `▄`, `▅`, `▆`, `▇`, `█`).

### Aspect Ratio Correction (2× Horizontal Scaling)
* Because terminal characters are approximately twice as tall as they are wide, visual graphics must sample or stretch by $2\times$ horizontally so circles and square pixels render without vertical distortion. See [`QuadPixelArt.md`](QuadPixelArt.md) and [`RenderPipelines.md`](RenderPipelines.md).

---

## 4. Glyph Sets & Registry Architecture

### Glyph Set
* A curated, cohesive collection of Unicode characters defined in `spec/render_modes.yaml` with a shared subpixel geometry and semantic category (e.g., Set 0 Half Blocks, Set 4 Quadrants, Set 6 Sextants, Set 86 Smooth Horizontal Bars).

### Composed Mode
* A render mode formed by combining multiple glyph sets (e.g. `spark+six`, `quad+spark`). Solvers match target subpixel bitmaps against the union of glyphs from all constituent sets.

### 1D vs 2D Boundary Dynamics
* The behavioral differences between 1-dimensional level bars (sparklines) and 2-dimensional spatial block sets (quadrants, sextants) when approximating diagonal or curved edges, which can lead to boundary artifacts like hairlines, ears, and horns. See [`GlyphEdgeArtifacts.md`](GlyphEdgeArtifacts.md).

---

## 5. Optimization & Smart Rendering

### Smart Mode (`--smart` / `+smart`)
* An adaptive rendering optimizer that probes candidate rendered column widths near the target width, evaluates candidate reconstruction fidelity against the reference image via PSNR/MSE, selects the optimal candidate width, and pads the remaining terminal columns to maintain a clean layout.

### Reference Pyramid Downscaling
* A multi-step image downsampling method (`PyramidDownscale`) that repeatedly halves image dimensions using box averaging before performing the final scaling step, avoiding aliasing artifacts common in single-step decimation.

---

## 6. Related Documentation

* [`RenderPipelines.md`](RenderPipelines.md) — Step-by-step Mermaid diagrams and pixel data flows.
* [`GlyphEdgeArtifacts.md`](GlyphEdgeArtifacts.md) — Analysis of 1D vs 2D boundary artifacts and edge dynamics.
* [`QuadPixelArt.md`](QuadPixelArt.md) — Aspect ratio, quadrant math, and color quantization.
* [`SparklinePixelArt.md`](SparklinePixelArt.md) — 1D optimal split characters and sparkline rendering.
* [`Spec.md`](Spec.md) — Authoritative specification for render modes, buttons, and keys.
* [`RenderingBugPlaybook.md`](RenderingBugPlaybook.md) — Root cause analysis and golden verification guidelines.
