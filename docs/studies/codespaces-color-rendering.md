---
title: "Study: GitHub Codespaces Workbench Color Rendering Artifacts"
date: 2026-09-22
author: cati team
status: complete
---

# Study: GitHub Codespaces Workbench Color Rendering Artifacts

## Executive Summary

When rendering terminal image artwork using `cati` inside GitHub Codespaces (VS Code Web workbench), users reported severe visual distortions and color shifts in Light Theme (`codespaces-light.png`) that do not appear in Dark Theme (`codspaces-dark.png`). Most noticeably:
* Bright yellow subject regions (such as smiley face graphics) turn into a dark, murky olive-brown (`RGB(117,109,0)`).
* Higher-order modes (`six`, `2x3`, `3x3`, `all`, `all+`, `z`, `z+`) exhibit dark horizontal and vertical stripes across rendered cells.
* Subject contours (mouths, eyes, borders) dissolve or blend into the surrounding terminal background.

This study provides a quantitative investigation into the feedback images (`docs/feedback/codespaces-light.png` and `docs/feedback/codspaces-dark.png`) and identifies four distinct root causes in the interaction between `cati` ANSI escape outputs, the xterm.js web terminal emulator, VS Code contrast enforcement, and browser font rasterization.

---

## 1. Empirical Evidence & Quantitative Analysis

Comparing `docs/feedback/codespaces-light.png` (Light Theme, background `#f6f8fa`) and `docs/feedback/codspaces-dark.png` (Dark Theme, background `#010409`) across identical test grids ($1028 \times 947$ pixels):

### Pixel Count & Color Shift Summary
* **Total Bright Yellow Pixels (`R > 200, G > 180, B < 100`)**:
  * **Dark Theme**: 105,555 pixels
  * **Light Theme**: 89,852 pixels (**15,703 fewer bright yellow pixels**)
* **Murky Olive-Brown Pixels (`RGB(117,109,0)` / `#756d00`)**:
  * **Dark Theme**: 0 pixels
  * **Light Theme**: **21,718 pixels**

### Breakdown by Render Mode
| Mode Family | Modes Included | Light Theme Murky Pixels | Bright Yellow (Light) | Bright Yellow (Dark) | Behavior |
| :--- | :--- | :---: | :---: | :---: | :--- |
| **Basic Block** | `full`, `half`, `quad`, `quad+`, `bars`, `bars+` | **0** | ~50,486 | ~50,486 | **Clean color reproduction** |
| **Sextant & Composite** | `six`, `2x3`, `3x3`, `all`, `all+`, `z`, `z+` | **21,718** | ~39,366 | ~55,069 | **Severe color shift & striping** |

In basic modes (`full`, `half`, `quad`), 0 pixels were distorted. All 21,718 distorted pixels occurred exclusively in higher-order sextant (`six`, `2x3`, `3x3`) and sparkline composite (`all`, `all+`, `z`, `z+`) modes.

---

## 2. Root Cause Analysis

### Cause 1: VS Code / xterm.js Minimum Contrast Enforcement (`terminal.integrated.minimumContrastRatio`)

The primary cause of the yellow-to-olive color transformation is VS Code's built-in terminal contrast adjustment feature.

1. **The Mechanism**: By default, VS Code / xterm.js enables `"terminal.integrated.minimumContrastRatio": 4.5`. When foreground text is rendered over the terminal background, xterm.js calculates the WCAG luminance contrast ratio between the foreground color and the cell's background color.
2. **Missing Background Escapes (`hasBG = false`)**:
   In `cati`'s sextant (`v1/sextant/render.go`) and sparkline renderers, sub-cell candidates where only a subset of sub-pixel bits are active emit a 24-bit foreground ANSI escape (`\x1b[38;2;251;233;3m`) but **no background escape** (`\x1b[48;2;...m`) when `hasBG = false`.
3. **Contrast Calculation in Light Theme**:
   * **Foreground**: Bright Yellow (`#fbe903`, relative luminance $L \approx 0.82$)
   * **Effective Background**: Default Terminal Background (`#f6f8fa`, $L \approx 0.95$)
   * **Contrast Ratio**: $\frac{0.95 + 0.05}{0.82 + 0.05} \approx 1.15 : 1$ (well below the 4.5:1 requirement).
4. **Auto-Adjustment**:
   Because the contrast is $1.15:1$, xterm.js dynamically darkens the yellow foreground until it reaches 4.5:1 against `#f6f8fa`. This darkens `#fbe903` directly into dark olive-brown `#756d00` (`RGB(117,109,0)`).
5. **Why Basic Modes Are Unaffected**:
   Basic block modes (`half`, `quad`) use two-color cells with explicit background escapes (`\x1b[48;2;...m`) or full blocks (`█`). When xterm.js evaluates a cell with an explicit 24-bit background escape matching the image, the contrast is evaluated against that cell's explicit background (or full block fill), avoiding the auto-darkening transformation.

---

### Cause 2: Unpainted Sub-Cell Background Bleed & Transparency Leakage

Higher-order modes (`six`, `2x3`, `3x3`, `all`, `z`) select sparse Unicode glyphs to represent sub-cell detail. In cells where `hasBG` is false or where sub-pixel masks contain inactive bits ($0$s in the sextant/spark mask):
* **Sextant Inactive Bits**: The unpainted regions of the character cell are transparent within the glyph itself and rely on the terminal's default background to fill the remaining area.
* **Theme Mismatch**:
  * In **Dark Theme**, the default background is dark (`#010409`), which closely matches dark shadow contours around image subjects.
  * In **Light Theme**, the default background is light (`#f6f8fa`).
* Unpainted sub-cell gaps cause the white terminal background to bleed directly through subject faces and borders, creating bright cuts, broken outlines, and mottled textures.

---

### Cause 3: Unicode 13.0 Font Fallback & xterm.js Glyph Rasterization Gaps

Sextant characters ($U+1FB00$–$U+1FB3B$, added in Unicode 13.0) are absent in standard monospace terminal fonts installed in web containers (such as `Consolas`, `Menlo`, `Courier New`, or `Liberation Mono`).

1. **Font Fallback**: xterm.js falls back to secondary system fonts (e.g., `Noto Sans Symbols2` or `DejaVu Sans`).
2. **Cell Metrics Discrepancy**: Standard block elements ($U+2580$–$U+259F$) in primary monospace fonts are designed to fill 100% of character cell dimensions ($1 \times 1$ cell bounding box without line height padding). Fallback glyphs, however, often use standard font metrics with line-spacing, leading, or advance-width padding.
3. **Sub-Pixel Gap Artifacts**: In the web browser (Canvas/WebGL renderer), these font metric discrepancies manifest as horizontal or vertical gaps between adjacent cells. In `codespaces-light.png`, these gaps appear as dark horizontal stripes across the smiley face where adjacent row cells fail to seamlessly touch.

---

### Cause 4: Subject Contours & Transparency Inversion

In graphics with white/bright subject elements (e.g., white teeth/mouths or eyes):
* In **Dark Theme**, unpainted space cells or white foregrounds stand out distinctly against dark terminal backgrounds.
* In **Light Theme**, unpainted spaces or white features dissolve directly into the `#f6f8fa` terminal background, causing feature inversion (e.g., mouths appearing as hollow background cutouts rather than solid shapes).

---

## 3. Mitigations & Recommendations

### A. Recommended `cati` Codebase Enhancements
1. **Explicit Cell Background Emission**:
   Modify sub-cell renderers (`v1/sextant`, `v1/sparkline`) to emit explicit ANSI background escapes (`\x1b[48;2;...m`) for opaque cells even when `hasBG` would otherwise default to unpainted. Explicit background escapes prevent xterm.js from calculating contrast against the default terminal background and suppress theme auto-darkening.
2. **Theme-Aware Background Neutralization**:
   Provide an option or auto-detection flag (e.g., `--bg-color` or `--opaque-bg`) to fill unpainted sub-cell gaps with explicit dark/light colors matching source images rather than leaking terminal default background colors.

### B. Recommended User & Environment Workarounds for GH Codespaces
1. **Disable VS Code Contrast Adjustment**:
   Add the following setting to VS Code / Codespaces `settings.json`:
   ```json
   "terminal.integrated.minimumContrastRatio": 1
   ```
   This disables xterm.js foreground color alteration and preserves exact 24-bit RGB colors.
2. **Preferred Modes for Light Workbench Themes**:
   When using light terminal themes without modifying VS Code settings, prefer `--mode quad` or `--mode half`, which maintain 100% explicit cell coverage and are immune to `minimumContrastRatio` color shifts.
