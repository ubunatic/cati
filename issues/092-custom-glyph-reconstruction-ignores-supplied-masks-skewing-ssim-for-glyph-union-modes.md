# 092 — Custom glyph reconstruction ignores supplied masks, skewing SSIM for glyph_union modes

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: [#025](025-spec-driven-render-modes.md), [#086](086-add-vec-and-oct-golden-render-tests-and-investigate-mask-orientation.md), [#093](093-fix-octant-glyph-to-subpixel-mapping.md), [SparklinePixelArt](../docs/SparklinePixelArt.md), [Rendering Bug Playbook](../docs/RenderingBugPlaybook.md)

---

## 1. Problem & Motivation
Cati can map source pixels to custom glyphs and foreground/background colors,
but cannot faithfully reconstruct pixels from some of those glyphs for quality
evaluation. SSIM then measures an image different from the represented output.
This undermines mode comparisons and quality-driven rendering decisions,
especially for vector (`vec`) and octant (`oct`) modes and future glyph sets.

The required round trip is source image → selected glyphs and colors → pixel
reconstruction → comparison with the reference image. Reconstruction must use
the same coverage and geometry as selection, rather than a separate incomplete
guess based on Unicode code points.

## 2. Technical Specification / Findings
Targeted inspection on 2026-10-06 confirmed:

- `v1/sparkline/render.go:candidatesForShapes` turns each supplied `Shape.Mask`
  into a row-major coverage predicate. Custom-glyph selection uses this predicate.
- `renderToImageWithOptions` selects cells using those custom candidates, then
  paints pixels through `reconstructedCellColor`. That helper calls
  `maskContains(cell.Ch, ...)`, losing the selected custom mask.
- `maskContains` recognizes standard blocks/quadrants and sextants, but defaults
  to `true` for unknown glyphs. Native octants and many vector glyphs therefore
  reconstruct as full foreground blocks, discarding their background regions.
  For example, U+1CD00 represents only octant 3 (mask 0x04 on a 2×4 lattice),
  yet the rune-only reconstruction paints all eight regions as foreground.
  Even a recognized rune can reconstruct incorrectly if the caller supplied
  different coverage; the supplied inventory must remain authoritative.
- Both serial and worker custom-shape reconstruction share this painting path.
  `cmd/ssim.go` passes glyph options to these APIs; `cmd/smart_render.go`
  also uses custom-shape reconstruction. Consequently, correcting glyph selection
  alone does not correct the image used for quality evaluation.

The magnitude and direction of SSIM distortion depend on the selected glyphs,
colors, and input; do not assume scores always rise or fall. #093 fixed octant
glyph-to-mask definitions, but did not fix this reconstruction data flow.
This investigation establishes the code-path mismatch, not a measured corpus-wide
score impact. This is reconstruction of modeled glyph coverage, not a requirement
to rasterize terminal fonts or parse ANSI output.

## 3. Implementation & Verification Plan
**Goal**: Make custom-glyph reconstruction faithfully paint the selected glyph's
supplied mask and foreground/background colors at the resolved cell geometry,
and verify the resulting quality-evaluation path, or stop and report when blocked
on a user decision or denied permission.

Before implementation, check live code and recent commits; re-verify the findings
against HEAD. Keep the fix centered on carrying or resolving the authoritative
selected coverage through reconstruction, with the same coordinate scaling as
selection. Avoid a second hardcoded vector/octant mask inventory. Unsupported
coverage must not silently become a full block, and reconstruction failures must
not produce plausible quality scores from a substitute source image.

Done means:

- Regression tests first reproduce the current defect using an asymmetric
  native octant and vector shape with distinct foreground/background colors.
  Assert exact reconstructed pixels against independently known coverage.
- All resolved octant and vector shapes reconstruct consistently with their
  supplied masks. Cover custom coverage for an already-recognized rune, unions
  with scaled geometry, empty/full shapes, color inversion, transparency, and
  partial or uneven cell dimensions where supported.
- Serial and worker APIs agree, and standard block/quadrant/sextant behavior
  remains correct. Rendering and reconstruction agree on selected glyphs and
  colors; do not require an arbitrary lossy source image to achieve SSIM 1.
- An SSIM integration check uses independently constructed reference pixels,
  proving that evaluation sees the corrected reconstruction. Check benchmark
  and smart-render consumers and record representative before/after scores.
- Update the relevant rendering documentation and add inspectable regression
  coverage with algorithm/geometry metadata where golden images are used.
  Predict golden impact first and follow the Rendering Bug Playbook; never
  regenerate goldens merely to pass tests.
- Run focused regression tests, the full suite, and required build/preflight
  checks. Record validation and close this ticket only after completion.
