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

## Results

Added all eight quarter/three-quarter bars and ten quadrants with exact
row-major 4×4 masks. Inventory tests check every new mask, duplicate runes,
and four opposite-bar complement pairs; sextant exclusion and the
hourglass/bowtie complement check remain intact.

Reproduction-first: `TestVectorFlatQuarterEdgeUsesStraightBar` failed before
the inventory change, choosing U+1FB69 (`🭩`) for an opaque white top row over
three black rows. Its `0xFF90` mask mixes four white and two black pixels in
the background (mean 170), giving RGB SSE 260100; a quarter bar has SSE 0.
After the change, the test requires a straight quarter bar and exact foreground
and background grid colours (zero fitting error). No goldens were regenerated.

Installed-binary results from `cati modes -w 12 emojig` and
`cati modes -w 12 circle` (ANSI colours stripped below; standard vector rows):

| Asset | SSIM before | SSIM after |
|---|---|---|
| emojig | 0.57 | 0.58 |
| circle | 0.78 | 0.79 |

```text
emojig — vector, w=12, ssim=0.58
  🭈▄▆▆▆▆▄🬽
 🭄🭪🭉🬽▌🭇🮂🬿▌🭏
▐🭏▀🭀🮅🭃🭎▀🭊🭄🭅▌
▐▀▀▛🭕▀▀🭞🭒🭆🭄▌
 🭕▀🭌▄▄▄▄🭃🭆🭠
  🭣🭧🮅🮅🮅🮅🭜🭘

circle — vector, w=12, ssim=0.79
    🭊▀▀🬿
 🭉🭆      🭑🬾
 ▎        ▊
 ▎        ▊
 🭎🬿      🭊🭃
    🭑▀▀🭆
```

The emojig bottom flat span now uses four straight upper-three-quarter bars
instead of four repeated diagonals; circle's vertical sides use quarter bars.
Curved corners continue to use diagonal glyphs.

### Separate open problem

An extra image-reconstruction assertion exposed an existing limitation:
`sparkline.reconstructedCellColor` calls the hardcoded `maskContains` lookup
instead of the supplied custom mask. For U+1FB82 it reconstructs the whole
cell as white although the grid selects the correct bar and colours.
This affects custom-glyph reconstruction/SSIM and needs follow-up alongside
#086; changing `v1/sparkline/render.go` is outside this milestone's file scope.
The reported SSIM values above are the current CLI measurements.

### Verification

- `go vet ./...`: passed.
- `HTO=0 go test ./... > /tmp/dev091-test.txt 2>&1`: passed;
  `grep -- '--- FAIL' /tmp/dev091-test.txt` found no failures.
- `make install`: passed; both modes commands above used the installed binary.
- `make preflight`: passed, including demo render checks.
- Reviewed the final diff: only inventory, tests, documentation, and this ticket
  change; no golden updates.
