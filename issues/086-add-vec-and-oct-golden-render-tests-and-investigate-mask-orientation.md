# 086 — Add vec and oct golden render tests

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#083](083-add-vector-render-mode-using-linear-cell-split-characters.md), [#084](084-add-octant-render-mode-using-unicode-1cd0-block-characters.md), [#089](089-fix-vector-mode-rune-mask-mapping-for-symbols-for-legacy-computing.md), [#092](092-custom-glyph-reconstruction-ignores-supplied-masks-skewing-ssim-for-glyph-union-modes.md), [#093](093-fix-octant-glyph-to-subpixel-mapping.md), [Rendering Bug & Golden-Change Playbook](../docs/RenderingBugPlaybook.md)

---

## 1. Problem & Motivation
The reported vector (`vec`) and octant (`oct`) glyph-to-mask defects have been fixed in #089 and #093. Both modes still need inspectable golden renders to preserve the corrected output and catch future regressions in orientation, coverage, and color fitting. This ticket remains open for that golden-test coverage.

## 2. Technical Specification / Findings
- **Vector mapping complete (#089):** corrected diagonal half-planes, triangular blocks, hourglass/bowtie coverage, and complement pairs; the ticket records focused tests, full-suite success, and CLI verification.
- **Octant mapping complete (#093):** corrected all 26 reserved glyph mappings and the resulting native-octant assignment. Independent assertions verify all 230 native octants and reserved shapes; the user approved the visual result and confirmed the full suite passed. Fix commit: `5755171`.
- **Golden coverage remains:** establish a small, representative set for both modes with directional and asymmetric coverage, plus descriptive algorithm and parameter metadata in the PNGs. No goldens were changed by the octant fix.
- **Reconstruction dependency (#092):** custom-shape selection uses supplied masks, but pixel reconstruction still uses a hardcoded rune lookup and treats unknown glyphs as fully filled. Golden paths using that reconstruction must consume the authoritative masks before their images can be trusted. This is separate from the completed mapping fixes.

## 3. Implementation & Verification Plan
**Goal**: Add inspectable golden render tests for the corrected `vec` and `oct` modes, or stop and report if blocked on a user decision or denied permission. Done means the goldens cover representative orientations for both modes and every golden change is justified by the rendering behavior it records.

1. Inspect the golden rendering path and resolve any dependency on the incorrect reconstruction tracked in #092 before recording expected pixels.
2. Add deterministic fixtures that expose diagonal direction, asymmetric octant placement, empty/full coverage, and foreground/background inversion. Include a representative sample-image render for each mode.
3. Verify expected coverage independently of production masks; preserve the existing mapping regressions from #089 and #093. Embed algorithm/mode and rendering parameters as PNG `tEXt` metadata.
4. Follow the Rendering Bug Playbook: explain the expected pixels and any changed existing goldens; never regenerate merely to make tests pass.
5. Run focused golden tests, the full suite, and required build/preflight checks; document the coverage and close this ticket when complete.

## 4. Historical octant mapping audit (2026-10-06, resolved by #093)

Reported input: `cati assets/samples/sample-003-darth-daughter.jpg -m oct -W 10`.
Before the fix, `spec/load.go:generatedOctantShapes` mapped 256 row-major 2x4 masks
to glyphs, reserving 26 supposed pre-existing shapes and assigning the remaining
230 sequentially from U+1CD00. The reserved masks were incorrect, so the
sequential assignment was also incorrect.

Comparing the actual `ResolveGlyphSetExpression("octant")` output against
[Unicode 17.0 UnicodeData](https://www.unicode.org/Public/17.0.0/ucd/UnicodeData.txt)
finds **227 of 230 native octant glyphs have incorrect masks** (only 3 agree).
For each `BLOCK OCTANT-<digits>` name, the authoritative mask is the bitwise
union of `1 << (digit - 1)`; compare that with the production shape's boolean mask.
Examples: U+1CD00 (octant 3) gets 0x06 instead of 0x04; U+1CD01
(octants 2,3) gets 0x07 instead of 0x06; U+1CD02 (octants 1,2,3)
gets 0x09 instead of 0x07.

Independent errors in the reserved table include U+1FB00 (a sextant, height
1/3 rather than 1/4), U+1FB02 (sextants 1,2 rather than a single octant),
U+2594 (upper 1/8 rather than upper 1/4), and U+2587 (lower 7/8 rather than
upper 3/4). Mask 0xE0 is an asymmetric three-octant shape, not the horizontal
lower-three-eighths block U+2583.

The custom-shape renderer consumes masks in row-major order, so these findings
identify a glyph inventory defect rather than a transposed indexing convention.
Its fitting calculations use the incorrect masks, while the terminal displays
the actual Unicode glyph coverage. This can move edges and create apparent
alignment defects independently of font behavior.

`go test ./spec -run TestOctantAndVectorGlyphSets -count=1` passes: it checks
geometry, inventory size, and presence of native octant glyphs, but not their
semantic correspondence to masks. No rendering code or golden images changed
during this investigation. The inventory correction and independent full-table
mapping assertions were subsequently delivered in #093. The audit above records
the pre-fix defects; it does not describe the current mappings.
