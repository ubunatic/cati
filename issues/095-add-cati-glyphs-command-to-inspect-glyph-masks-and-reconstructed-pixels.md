# 095 — Add cati glyphs command to inspect glyph masks and reconstructed pixels

**Status**: Closed
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
   - `cati modes glyphs [mode|expr]`: Lists the glyph inventory for a named mode (e.g. `oct`, `vec`, `half`, `six`) or debug expression (e.g. `d10,50`), displaying each glyph, codepoint, dimensions ($W \times H$), and a visual ASCII/halfblock subpixel mask card (`--format ascii|halfblock`).
   - `cati modes test [image|pattern] [-m mode|expr]`: Renders custom images (positional or `-i/--image`) or built-in probe patterns and displays a side-by-side terminal comparison of **Source Pixels (1:1)** vs **Reconstructed Pixels (1:1)** alongside the glyph output and MSE/PSNR/SSIM summary.
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
embedded metadata. M1 command tests and preflight passed. All three milestones
are complete.

M2 implemented: `cati modes test [pattern|image] [-m mode|expr]` runs all three
default demonstrations or one selected pattern/image, with `-i/--image` and
`--list-patterns` support. Glyph selection and reconstruction share spec masks
and a transparently padded cell grid. Source and reconstruction panels retain
1:1 pixels; metrics exclude padding and report RGBA8 MSE/PSNR and luminance
SSIM (unavailable below 8x8). Independent metadata-bearing PNG fixtures cover
exact octant probes and the numerically explained lossy vector diagonal.
Verification passed: `go test ./cmd/... ./internal/metrics/...`, `make test`
(both fastpath configurations), `make preflight`, and installed CLI smoke checks.

- [x] **M1 (Glyph Inventory & Subpixel Mask Cards)**: Delivered in `e900675` with `cmd/modes_glyphs.go`, comprehensive tests, an independent PNG fixture, and documentation.
- [x] **M2 (Reconstruction Probe Comparison)**: Delivered in `ffc87d5` with `cmd/modes_probe.go`, built-in probe/asymmetric, diag/vector, and grid/multi patterns, custom images, mode expressions, shared MSE/PSNR semantics, and three independent PNG fixtures.
- [x] **M3 (Final Verification & Closure)**: Final tests and preflight passed; delivery documentation and issue indexes synchronized.

## 4. Final Verification — 2026-10-07

- `go test ./...` passed at delivery commit `ffc87d5`; command tests completed in 60.227s. All packages passed or reported no test files.
- `make preflight` passed: built and installed cati, catiplay, catibrowse and examples; `go vet ./...` passed; demo-widths reported `OK: no render errors`.
- M1/M2 also passed `make test` in both default and reference (`CATI_FASTPATH=0`) configurations, focused command/metric tests, and installed CLI smoke checks for help, presets, glyph cards, and custom-image unions.
- Tests verify mask orientation, inventory ordering, authoritative glyph reconstruction, nonzero bounds, alpha handling, transparent padding, ANSI panel alignment, errors, metrics, and metadata-bearing golden pixels. Existing renderer goldens remain unchanged.
- Documentation covers delivered syntax, presets, 1:1 pixel semantics, and metric limits in `docs/System.md`, `docs/SparklinePixelArt.md`, and `docs/ImageQualityMetrics.md`.

SSIM is explicitly unavailable below 8x8; larger-image SSIM uses complete 8x8
windows only. The vector diagonal's expected fitting loss is documented and
tested (RGBA8 MSE 1012.796875). No unresolved work remains for this ticket;
related issues such as #086 and #055 retain their separate scope.
