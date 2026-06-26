# 009-explore-more-sparkline-rendering-modes.md

## Issue
Explore more rendering modes for sparklines, including vertical/horizontal sparkline characters (like 1/8, 3/4 blocks and their foreground/background inversion). Also check what other full-cell characters are usable (triangles, curves, etc.). Assume each cell is at least 8x8 pixels; analyze the 8x8 block to decide on the best character + foreground/background combination.

## Motivation
Current sparkline implementation may be limited to a set of horizontal block elements (e.g., Unicode block elements like ▁▂▃▄▅▆▇█). We want to explore:
- Vertical sparklines (using vertical block elements?).
- Using fractional block elements (e.g., U+258F LEFT THREE EIGHTHS BLOCK, U+2592 MEDIUM SHADE, etc.) and their inverted forms (swapping foreground and background).
- Other full-cell Unicode blocks that might represent shapes like triangles, curves, or other patterns that could be used for sparklike visualization in an 8x8 cell.

## Tasks
- [x] Research Unicode block elements (U+2580 to U+259F) for fractional blocks and their orientations.
- [ ] Research other Unicode blocks that might contain suitable full-cell characters for 8x8 pixel representation (e.g., Geometric Shapes, Block Elements, Box Drawing, Braille Patterns, etc.).
- [ ] For each candidate character, analyze its 8x8 pixel representation (assuming a typical font) to determine how well it represents different fractions or shapes when combined with foreground/background colors.
- [ ] Consider vertical orientations: are there characters that naturally vertical? Or can we rotate characters? (Note: terminal may not support rotation, but we can use different characters that resemble vertical bars.)
- [ ] Consider using braille patterns (U+2800..U+28FF) which are designed for 2x4 dot patterns, but can be interpreted in 8x8? Might be too small.
- [ ] Evaluate which characters provide the best visual resolution for sparklines in an 8x8 cell.
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
2. Create a test program to render various Unicode characters in an 8x8 grid (using a fixed-width font) and evaluate their suitability for representing different levels (0% to 100%).
3. Consider both horizontal and vertical orientations.
4. Document findings and propose a set of characters for horizontal and vertical sparkline modes.

## Status
🔴 Open