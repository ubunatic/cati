# 091 — Include fractional bars and quads in vector mode to support straight lines

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: #089, #090

---

## 1. Problem & Motivation
Vector mode renders flat horizontal/vertical edges as a sawtooth
(e.g. `cati modes -w 12 emojig`: bottom row `🭣🭧🭓🭓🭓🭓🭜🭘`).

## 2. Technical Specification / Findings
Root cause (proven by SSE arithmetic): the vector inventory has no 1/4 or 3/4
straight bars and no quadrants. For a target with only the top 4x4 row filled:
- `▀` (8 px) error = 4 px, `' '` error = 4 px
- `🭓` (0xCFFF complement-ish, 6 px in top rows) error = 2 px

So the optimizer correctly picks the diagonal, and repeats it along the edge.

Missing glyphs (4x4 masks, bit = y*4+x):
- Horizontal: `🮂` U+1FB82 upper 1/4 `0x000F`, `🮅` U+1FB85 upper 3/4 `0x0FFF`,
  `▂` lower 1/4 `0xF000`, `▆` lower 3/4 `0xFFF0`
- Vertical: `▎` left 1/4 `0x1111`, `▊` left 3/4 `0x7777`,
  `🮇` U+1FB87 right 1/4 `0x8888`, `🮊` U+1FB8A right 3/4 `0xEEEE`
- Quadrants: `▘ ▝ ▖ ▗ ▚ ▞ ▛ ▜ ▙ ▟`

`spec/glyph_sets_test.go` currently forbids quadrants in vector mode
(added in PR #40); that assertion must be removed/inverted.

## 3. Implementation & Verification Plan
1. Add the glyphs to `vectorRuneMasks` / rune list in `spec/load.go`.
2. Update the vector test: require straight bars and quads; keep sextant exclusion.
3. Verify `cati modes -w 12 emojig` and `circle`: no repeated diagonal on flat edges; SSIM should not drop.
4. Add vector golden renders (see #086) once output is stable.
