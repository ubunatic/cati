# 020 — Sextant pure-column masks emit NUL glyph (garbled right edge)

**Status:** ✅ Closed  
**Refs:** [System.md](../docs/System.md), `internal/sextant/render.go` (`init`, `displayMask`, `scoreMask`)

---

## Problem

`cati assets/baby-720p.mp4 -m=xs -w=80` rendered a broken right border: the
right portion of most rows was unfilled (terminal-default background showing
through). Smaller repro `-w=6` already shows it; `-m=sq` (and every non-sextant
mode) is unaffected. Independent of `-j` (parallel rendering) — `-j=1` and
`-j=10` produce byte-identical output.

Repro: `cati assets/baby-720p.mp4 -m=xs -j=0 -w=6`.

## Root Cause

The `2×3` sextant block has 64 possible bit masks but only 60 dedicated Unicode
glyphs (`U+1FB00..`). Four masks have no sextant rune:

- `0` (empty) and `63` (full) — already special-cased in `scoreMask` to a space.
- `0b101010` (left column `1·3·5`) and `0b010101` (right column `2·4·6`) — these
  coincide with the pre-existing half-block characters `▌` (U+258C) and `▐`
  (U+2590), so Unicode omits them from the sextant set.

For the two column masks, `displayMask` found neither the mask nor its inverse
in `sextantRuneByMask` and returned `(0, false)`. `scoreMask` then set
`cell.ch = sextantRuneByMask[0] = rune(0)`, and the renderer wrote a literal NUL
byte. A NUL is zero-width in the terminal, so each affected cell printed nothing:
the row's visible width shrank by one per occurrence and the right edge went
unfilled. `directMask` produces mask `42` directly from any clean vertical split
(left brighter than right), so photographic frames hit it constantly.

The reconstructed image (`RenderToImage`, used for SSIM/goldens) was wrong too:
`cell.mask` was also `0`, so every pixel took the background colour — a vertical
split collapsed to a single flat colour (verified: old `demo_vert_split_8x8`
sextant golden was entirely blue, dropping the red left column).

The ANSI validator (`validateRenderedANSI`) missed it because
`utf8.DecodeRuneInString` reports the NUL as a width-1 rune, so the column count
matched and validation passed.

## Fix

Register the two half-block columns in `sextantRuneByMask` (`init`):
`0b101010 → ▌`, `0b010101 → ▐`. `displayMask` now resolves them and
`scoreMask`/`RenderToImage` produce the correct two-colour split.

## Golden Impact

Sextant-only — 17 `render_sextant_*` PNG goldens and 3 `cli_sextant_*.ansi`
goldens regenerated (clean vertical-split blocks across the synthetic
`demo_*` and photographic `sample_*` corpora). No non-sextant golden changed.

## Tests

- `internal/sextant`: `TestSextantNoNulGlyph` asserts no opaque mask resolves to
  `rune(0)` and that the two column masks map to `▌`/`▐`; `TestSextantCandidateCount`
  updated (`sextantRuneByMask` now 62 = 60 native + 2 half-block columns).
