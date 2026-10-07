# 095 — Add cati glyphs command to inspect glyph masks and reconstructed pixels

**Status**: Open
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

We should turn this capability into a first-class CLI command: `cati glyphs`.

## 2. Technical Specification / Findings
1. **Command Structure (`cmd/glyphs.go`)**:
   - `cati glyphs [mode|expr]`: Lists glyph inventory for a named mode (e.g. `oct`, `vec`, `half`, `six`) or debug expression (e.g. `d10,50`), displaying each glyph, codepoint, dimensions ($W \times H$), and a visual subpixel mask card.
   - `cati glyphs test [image|pattern] -m <mode>`: Renders test inputs or preset patterns (single-cell asymmetric probes, diagonal cuts, multi-cell grids) and displays a side-by-side terminal comparison of **Source Pixels** vs **Reconstructed Pixels** alongside the glyph output.
   - Support flags: `-m / --mode` (render mode or glyph set ID), `-W / --width`, `--info` (metadata inspection).
2. **Data Flow**:
   - Uses `spec.ResolveGlyphSetExpression` to obtain authoritative shapes and geometry.
   - Uses `sparkline.RenderToGrid` / `sparkline.RenderToImageWithOptions` for glyph selection and faithful mask reconstruction.
   - Uses `halfblock.Render` to output 1:1 pixel maps for clear terminal inspection.

## 3. Implementation & Verification Plan
**Goal**: `/goal Implement the "cati glyphs" command to inspect glyph inventories, subpixel masks, and reconstructed pixels side-by-side, and verify it with automated tests and preflight, or stop and report when blocked on a user decision or denied permission.`

- Implement `cmd/glyphs.go` registering `cati glyphs` in Cobra.
- Port visual comparison logic from `scripts/reconstruct_demo.go` into `cati glyphs test` / `cati glyphs inspect`.
- Add unit and CLI tests in `cmd/glyphs_test.go`.
- Run `go vet ./...`, `make install`, `make test`, and `make preflight`.
- Update CLI documentation and man pages.
