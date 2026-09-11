---
title: Glyph Edge Artifacts & 1D Bar Dynamics
weight: 65
---

# Glyph Edge Artifacts & 1D Bar Dynamics

This document analyzes the visual boundary artifacts that occur when combining 1D bar/sparkline glyph sets with 2D block shapes on curved or diagonal boundaries (such as circular avatars, emojis, and geometric test fixtures).

---

## 1. Observed Visual Phenomena

When comparing render modes at small or medium resolutions (e.g. `cati modes -w 12` on the `emojig` logo), several subtle boundary artifacts appear in hybrid and bar-heavy modes:

1. **`spark+six` & `bars+` "Thinline Ears" (`▁`)**: Small horizontal step protrusions appearing on the far-left and far-right curved extremities of the circle.
2. **`z+` "Vertical Hairlines" (`▏`)**: Thin vertical 1/8th-width hairline spikes on the right side of the circle.
3. **`quad+` "Horns" (`▆`)**: Stepped rectangular crests sticking up at the apex of the circle.

---

## 2. Root Cause Analysis

### A. The 1D vs. 2D Form-Factor Mismatch

* **Pure 2D Glyph Sets** (e.g. Quadrants in Set 4, Sextants in Set 6):
  Subdivide the terminal cell across **both $X$ and $Y$ axes** simultaneously (e.g. $2 \times 2$ or $2 \times 3$). When a curve cuts diagonally or smoothly through a cell, a 2D quadrant or sextant can match a corner or sub-quadrant without painting the entire remaining width or height of the cell.
* **1D Bar & Sparkline Sets** (e.g. Sets 14, 44, 86, 88):
  Subdivide the terminal cell along **only one axis**:
  * Vertical bars (`▁`–`▇`): span **100% of cell width** $\times$ partial height (1/8 to 8/8).
  * Horizontal bars (`▏`–`▉`): span partial width (1/8 to 8/8) $\times$ **100% of cell height**.

### B. Mean Squared Error (MSE / SSE) Optimization

Cati's renderer searches candidate glyph masks and two-color foreground/background pairs to minimize Total Squared Error (SSE):

$$\text{SSE} = \sum_{p \in \text{FG}} (p - \text{fgAvg})^2 + \sum_{p \in \text{BG}} (p - \text{bgAvg})^2 + \text{Cost}(\text{transparent pixels tainted})$$

When a convex circular curve enters only a fraction of a terminal cell:

1. **Ears on `bars+` / `spark+six`**:
   The curve enters the bottom-inner corner of the cell by a few pixels. The algorithm evaluates `▁` (the 1/8th-height bottom bar). Slicing the 1/8th-height bar dramatically reduces color error on those few yellow pixels compared to leaving the cell empty. However, because `▁` is 100% wide horizontally, it paints a horizontal shelf across the entire width of the cell, producing the visual "ear".
2. **Hairlines on `z+`**:
   The right side of the circle enters the leftmost 1–2 pixels of a column cell. Mode `z+` contains Set 88 (`allbars`), which includes `▏` (the 1/8th vertical hairline bar). The optimizer picks `▏` because it covers the yellow pixels on the left. But because `▏` is 100% tall vertically, it draws a full-height vertical spike into the empty background.
3. **Horns on `quad+`**:
   At the apex of the circle, the center two columns contain 3/4-height vertical bars (`▆`, from Set 14 `vbars`), while the adjacent shoulder columns use 1/2-height half-blocks (`▟`, `▙`, `▄`). Because `▆` is 75% cell height while its neighbors are 50%, the top contour forms a stepped pair of horns.

---

## 3. Design Decisions & Mitigation

### Why Mode `z` Differs from `z+`

In `spec/render_modes.yaml` and `docs/SetIdeas.md`:
* **Set 86 (`morebars`, used in `z` and `bars+`)**: Explicitly omits `▏` (1/8 hairline) and `▉` (7/8) to avoid hairline spikes on curved edges.
* **Set 88 (`allbars`, used in `z+`)**: Includes all 8th-step bars as an exhaustive debug union.

Mode `z` is therefore the intended clean high-resolution mode, while `z+` demonstrates full bar coverage with known hairline sensitivity.

### Mode Selection Guidelines

* **Organic / Circular Shapes (Avatars, Icons, Fonts)**:
  Prefer pure 2D modes such as **`six`** ($2 \times 6$ sextant + quad) or **`all`** ($6 \times 6$), which contour smooth curves without rectangular protrusions.
* **Bar Charts, Data Visualizations, & Orthogonal Grids**:
  Modes with 1D bar sets (**`bars`**, **`bars+`**, **`spark`**) excel at rendering linear data, gradients, and straight horizontal/vertical edges.
