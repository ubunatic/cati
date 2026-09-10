# 009 — Explore more sparkline rendering modes

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [#041](041-assess-sparkline-and-composite-rendering-quality-at-small-widths.md), `docs/SparklinePixelArt.md`

---

## Issue
SetIdeas delivery is now owned by #025 (registry), #014 (experimental coverage),
and #043 (mode surface/migration). This ticket retains broader Unicode/font
research and supplies reproducible font evidence to #014; it does not duplicate
their implementation. Moredots and diagonals remain outside the current delivery.

Explore Unicode and terminal-font glyph candidates that could complement the
current sparkline algorithms. Algorithm quality, mask scoring, reconstruction,
transparency, and small-width assessment are tracked in #041; this ticket
should not duplicate that implementation investigation. Evaluate candidates
against the actual cell geometry in `spec/render_modes.yaml`, not an
unverified generic 8x8 bitmap.

## Motivation
Current sparkline implementation may be limited to a set of horizontal block elements (e.g., Unicode block elements like ▁▂▃▄▅▆▇█). We want to explore:
- Vertical sparklines (using vertical block elements?).
- Using fractional block elements (e.g., U+258F LEFT THREE EIGHTHS BLOCK, U+2592 MEDIUM SHADE, etc.) and their inverted forms (swapping foreground and background).
- Other full-cell Unicode blocks that might represent shapes like triangles, curves, or other patterns that could be used for sparklike visualization in an 8x8 cell.

## Scope and Tasks
- [x] Research Unicode block elements (U+2580 to U+259F) for fractional blocks and their orientations.
- [ ] Research other Unicode blocks that might contain suitable full-cell characters for 8x8 pixel representation (e.g., Geometric Shapes, Block Elements, Box Drawing, Braille Patterns, etc.).
- [ ] For each candidate character, capture its rasterization in named,
  reproducible fixed-width fonts and record the font/toolchain; do not treat a
  single font as universal terminal behaviour.
- [ ] Consider vertical orientations: are there characters that naturally vertical? Or can we rotate characters? (Note: terminal may not support rotation, but we can use different characters that resemble vertical bars.)
- [ ] Consider using braille patterns (U+2800..U+28FF) which are designed for 2x4 dot patterns, but can be interpreted in 8x8? Might be too small.
- [ ] Evaluate which characters provide useful resolution for the relevant
  4x8, 2x6, and 4x24 composite grids, and state where an 8x8 assumption does
  not apply.
- [ ] Propose a set of characters and usage guidelines for vertical/horizontal sparkline modes.

## Preliminary Web Search Findings
- Unicode Block Elements (U+2580–U+259F) includes characters like:
  - U+2581 LOWER ONE EIGHTH BLOCK (▁)
  - U+2582 LOWER ONE QUARTER BLOCK (▂)
  - U+2583 LOWER THREE EIGHTHS BLOCK (▃)
  - U+2584 LOWER HALF BLOCK (▄)
  - U+2585 LOWER FIVE EIGHTHS BLOCK (▅)
  - U+2586 LOWER THREE QUARTERS BLOCK (▆)
  - U+2587 LOWER SEVEN EIGHTHS BLOCK (▇)
  - U+2588 FULL BLOCK (█)
  - And their counterparts for upper, left, right, etc. (e.g., U+258F LEFT THREE EIGHTHS BLOCK, U+2590 LEFT FIVE EIGHTHS BLOCK, etc.)
- There are also characters like U+2591 LIGHT SHADE, U+2592 MEDIUM SHADE, U+2593 DARK SHADE which might be used for fractional shading.
- Braille patterns (U+2800–U+28FF) are 2x4 dots, but can be scaled to 8x8? Each dot is 2x4, so scaling to 8x8 might be done by repeating dots? Not sure if suitable for sparklines.
- Geometric shapes (U+25A0–U+25FF) include triangles, but they are not necessarily filling the whole cell.

## Next Steps
1. Conduct more focused web search for "vertical sparkline unicode characters" and "fractional block unicode".
2. Create a reproducible probe to rasterize candidate Unicode characters in
   fixed-width fonts and evaluate suitability for levels and shapes. Keep the
   probe or its checked-in test form if it becomes a project capability.
3. Consider both horizontal and vertical orientations.
4. Document findings and propose characters and usage guidelines, including
   font-coverage/fallback limitations. File any implementation follow-up only
   after #041’s evidence shows it belongs in the renderer.

## Status
🔴 Open
