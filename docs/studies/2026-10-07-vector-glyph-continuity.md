---
title: "Study: Vector Glyphs Need Continuity, Not Just Alignment"
date: 2026-10-07
author: Cati Team
status: complete
---

# Vector Glyphs Need Continuity, Not Just Alignment

On 2026-10-07 we fixed one vector-mode bug and then tried two new modes, `halfvec` and
`sixvec`. We dropped both. This study records why, so the next attempt starts from
what we know.

## 1. The vec6 jaggies (fixed)

`cati assets/doom1.png -W 100 --crop 8:3:46:24 -m vec6` drew the flat gray HUD bar
as `🮂🮂🮂🭩🭂🭍🭩🮂`. In a 4x6 cell the upper quarter bar `🮂` covers 2 rows, because
1.5 rows round up, while `▂` covers 1. No glyph had a 1-row top edge. A probe that
scored each glyph with the real renderer showed that on a 1-row bright bar the
diagonals won: `🭘`/`🭂` scored 84k against 116k for `🮂`.

Drawing `🮂` as 1 row by hand did not help. The teeth moved to the cells that need
2 rows. The fix was to add the eighth bars `▔▇🮃▅▃🮄▁🮆` (b412bc4). Now every row
boundary of every vector set has a straight bar
(`TestVectorEveryRowBoundaryHasStraightBar`). See
[SparklinePixelArt.md](../SparklinePixelArt.md).

**Lesson:** if a mode has no straight edge at a row boundary, the error minimiser
fills the gap with diagonals.

## 2. `halfvec`: half blocks plus 45° triangles (dropped)

The idea was `half` (` █▀▄`) plus triangles whose edges line up with half-block
pixels. A half-block pixel is about square (1 cell wide, ½ cell tall), so a triangle
that splits it corner to corner would have a true 45° edge.

- **No such glyph exists.** Every filled diagonal in Unicode 16 has its corners at
  x ∈ {0, ½, 1} and y ∈ {0, ⅓, ⅔, 1}, or at the cell centre. None has a corner at
  y = ½ on the left or right edge, so no right triangle has a full-width side and a
  ½-tall side.
- **The closest glyphs** are `🭭` (top half) and `🭯` (bottom half), which have their
  apex at the cell centre, and `🮚`, which is both together. `🮛` and the side
  triangles `🭬🭮` were rejected as too narrow.
- **Failure:** `█🭭█` cuts a V notch into a flat surface. On the HUD crop, halfvec drew
  rows of `🭯`/`🭭` teeth, because a ¼-cell bar has no straight glyph in this set.

## 3. `sixvec`: sextants plus third-based diagonals (dropped)

The Unicode smooth-mosaic diagonals `🬼`…`🭧` end on the ⅓/⅔ row lines and the ½ column
line, which are the sextant pixel lines. Chains of them join cleanly:

```
hill            🭈🭆🭂█🭍🭑🬽
ramp into bars  🬭🬭🭆🬹🬹🭑🬭🬭
staircase        🭈🭆🭂
              🭈🭆🭂███
ceiling valley  🭣🭧🭓🭞🭜🭘
```

The renderer would still put incompatible neighbours side by side. For example,
`🬭🭑` steps from an edge at ⅔ to one at ⅓, because the two glyphs are chosen
independently.

## 4. The common cause: choosing each cell alone

Every render mode picks each cell's glyph and colours by squared error for that cell
alone. Aligned glyph corners guarantee that an edge *can* continue into the next cell,
not that it *will*. With per-cell selection:

- a flat edge that falls between the available straight bars gets diagonal teeth (§1, §2);
- two diagonals that each fit well can meet at different heights (§3).

## 5. What a future attempt needs: continuity rules

Before any new diagonal mode, design selection rules that look at the neighbouring
cells. Options, none of them built yet:

- **Edge-height matching:** give each glyph the heights where its edge touches the left
  and right cell borders, and penalise a pair whose heights differ (`🬭🭑` = ⅔ vs ⅓).
- **Flat-first:** accept a diagonal only when it beats the best straight glyph by a
  clear margin. The margin would be a spec value.
- **Row-wise optimisation:** choose a whole row at once (dynamic programming over the
  edge heights), so the error is traded against continuity.

Each of these changes the renderer, not just glyph lists. It needs its own issue, a
probe on the HUD crop and the reconstruction probes, and a predicted golden impact
before any code is written (see [RenderingBugPlaybook.md](../RenderingBugPlaybook.md)).
