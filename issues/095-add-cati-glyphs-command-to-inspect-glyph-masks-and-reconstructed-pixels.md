# 095 — Add cati glyphs command to inspect glyph masks and reconstructed pixels

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#025](025-spec-driven-render-modes.md), [#086](086-add-vec-and-oct-golden-render-tests-and-investigate-mask-orientation.md), [#092](092-custom-glyph-reconstruction-ignores-supplied-masks-skewing-ssim-for-glyph-union-modes.md), [#093](093-fix-octant-glyph-to-subpixel-mapping.md), [SparklinePixelArt](../docs/SparklinePixelArt.md)

---

## 1. Problem & Motivation
When developing, debugging, and verifying render modes (such as vector, octant, braille, sextants, or custom glyph unions), developers and users need a first-class visual tool to inspect glyph inventories, check their resolved subpixel masks, and observe reconstructed pixels side-by-side against source inputs.

During the resolution of #092 and #094, dedicated visual probe scripts (`scripts/reconstruct_demo.go`) demonstrated how effective it is to display:
1. Emitted Unicode glyph art,
2. 1:1 Source pixel representation,
3. 1:1 Reconstructed pixel representation from the chosen candidate masks,
4. Quality and error metrics.

Rather than fragmenting the CLI with a separate top-level command competing with `cati modes`, integrate these glyph inspection and reconstruction probe capabilities directly into the `cati modes` command suite.

## 2. Technical Specification / Findings
1. **Command Integration (`cmd/modes.go` / `cmd/modes_glyphs.go`)**:
   - `cati modes glyphs [mode|expr]` (or `cati modes --glyphs [mode|expr]`): Lists the glyph inventory for a named mode (e.g. `oct`, `vec`, `half`, `six`) or debug expression (e.g. `d10,50`), displaying each glyph, codepoint, dimensions ($W \times H$), and a visual ASCII/halfblock subpixel mask card.
   - `cati modes test [image|pattern] -m <mode>` (or `cati modes --reconstruct` / `cati modes -i <image> --diff`): Renders test inputs or preset patterns (single-cell asymmetric probes, diagonal cuts, multi-cell grids) and displays a side-by-side terminal comparison of **Source Pixels (1:1)** vs **Reconstructed Pixels (1:1)** alongside the glyph output and metric summary.
2. **Data Flow**:
   - Uses `spec.ResolveGlyphSetExpression` to obtain authoritative shapes and geometry.
   - Uses `sparkline.RenderToGrid` / `sparkline.RenderToImageWithOptions` for glyph selection and faithful mask reconstruction.
   - Uses `halfblock.Render` to output 1:1 pixel maps for clear terminal inspection.

## 3. Implementation & Verification Plan
**Goal**: `/goal Implement glyph inventory mask inspection and pixel reconstruction probes integrated into "cati modes", and verify with automated tests and preflight, or stop and report when blocked on a user decision or denied permission.`

### Milestones

M1 implemented: `cati modes glyphs [mode|expr]` resolves inventories from the
spec and prints bordered ASCII or halfblock mask cards (`--format`). No selector
lists compositions and registered sets. Tests cover selector grammar, ordering,
geometry, mask orientation, errors, help, and an independent PNG fixture with
embedded metadata. M1 command tests and preflight passed. M2 reconstruction
probes are now implemented; M3 verification of the complete feature remains.

M2 implemented: `cati modes test [pattern|image] [-m mode|expr]` runs all three
default demonstrations or one selected pattern/image, with `-i/--image` and
`--list-patterns` support. Glyph selection and reconstruction share spec masks
and a transparently padded cell grid. Source and reconstruction panels retain
1:1 pixels; metrics exclude padding and report RGBA8 MSE/PSNR and luminance
SSIM (unavailable below 8x8). Independent metadata-bearing PNG fixtures cover
exact octant probes and the numerically explained lossy vector diagonal.
Verification passed: `go test ./cmd/... ./internal/metrics/...`, `make test`
(both fastpath configurations), `make preflight`, and installed CLI smoke checks.

- **M1 (Glyph Inventory & Subpixel Mask Cards)**: Add `cati modes glyphs [mode|id]` (or `cati modes --glyphs`) displaying character, codepoint, and visual subpixel mask cards for any resolved mode or glyph set.
- **M2 (Reconstruction Probe Comparison)**: Add `cati modes test` / `--reconstruct` comparing 1:1 Source Pixels vs Reconstructed Pixels side-by-side with emitted glyph art.
- **M3 (Tests, Preflight & Docs)**: Comprehensive unit tests in `cmd/modes_test.go`, verify `make preflight` and CLI help.
