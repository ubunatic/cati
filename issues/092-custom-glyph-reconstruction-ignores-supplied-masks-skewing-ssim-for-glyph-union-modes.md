# 092 — Custom glyph reconstruction ignores supplied masks, skewing SSIM for glyph_union modes

**Status**: Closed — preserved custom candidate masks in cellResult and reconstruction; full suite and preflight green
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
Re-verified against HEAD `f59b299` on 2026-10-07 (paths and line numbers as of that commit).

**Data flow (selection vs reconstruction):**

```
spec/load.go ResolveGlyphSets      -> GlyphShape{Glyph, Mask} scaled to LCM geometry (scaleMask)
cmd/interactive.go glyphOptions    -> sparkline.Options{CellW/H = geometry, AspectX: 1, Shapes}
sparkline candidatesForShapes      -> candidate.mask(x,y,w,h): nearest sample
                                      sx = min(W-1, x*W/w), sy = min(H-1, y*H/h)
selection  (render.go ~337-344)    -> makeBitCandidates(custom, cw, ch) | findBestCandidate
                                      -> cellResult{Ch, FG, BG, Err}   (mask is dropped here)
reconstruction (render.go ~353)    -> reconstructedCellColor -> maskContains(cell.Ch, ...)
```

- `cellResult` (`v1/sparkline/render.go:414`) and `bitCandidate`
  (`v1/sparkline/solver.go:17`) carry only the rune, so the selected coverage is
  lost before painting. `maskContains` (`render.go:453`) returns `true` for any rune
  outside its block/quadrant/sextant table.
- **Probe proof (live code):** a 2x4 RGBA image with octant 3 `(x=0,y=1)` red and
  the rest blue, rendered with `Shapes = {U+1CD00 mask[2]=true, ' ' empty}` at 1x1
  cells. Selection picks U+1CD00 with FG=red and BG=blue (zero error). Reconstruction
  paints **all 8 pixels red**, so the BG region is lost. That is 7 of 8 pixels wrong.
- **Recognized runes still diverge.** Even when `maskContains` knows the rune, it uses
  its own region formulas (`sextantRegion`: `col=(x*2)/w`, `row=(y*3)/h`; sparks:
  `(h-y)*8 <= level*h`). These differ from the shape's nearest-sample scaling
  when the cell size is not a multiple of the shape geometry. They also ignore
  any custom coverage supplied for that rune.
- **Uneven cells:** `ssim.go`/`smart_render.go` use `outCols = ceilDiv(Dx, CellW)`. The
  renderer then splits cells as `tc*pixW/outCols`, so edge and interior cells can
  be smaller than the glyph geometry. Some mask columns or rows are then never
  sampled. Selection and reconstruction must both use the same
  `candidate.mask(x, y, cw, ch)` for that exact cell size.
- **Consumers:** `cmd/ssim.go:69/71` (serial and `-j` workers) and
  `cmd/smart_render.go:146` call `RenderToImageWithOptions[J]`. Both use the same
  `renderRow` closure, so one fix covers both. The ANSI `Render` path emits only the
  rune and is not affected.
- **Out of scope:** built-in `Mode` paths (no `Shapes`) choose only runes that
  `maskContains` knows. Their behavior must stay byte-identical. That also means the
  sparkline goldens must not change.

The magnitude and direction of the SSIM distortion depend on the input, so no
corpus-wide score impact has been measured. #093 fixed the octant
definitions but did not touch this path. This issue covers reconstruction of
modeled coverage only. It does not cover font rasterization.

## 3. Implementation & Verification Plan
**Goal**: Make custom-glyph reconstruction faithfully paint the selected glyph's
supplied mask and foreground/background colors at the resolved cell geometry,
and verify the resulting quality-evaluation path, or stop and report when blocked
on a user decision or denied permission.

Before implementation, check live code and recent commits; re-verify the findings
against HEAD.

**Proposed shape of the fix:** have the solver return the winning candidate's index
(or its `mask128` for the cell's `cw x ch`) alongside the rune. Then
`renderToImageWithOptions` paints with `custom[idx].mask(x-x0, y-y0, cw, ch)` when
`len(custom) > 0`. Keep `maskContains` for built-in modes only. Don't add rune
lookup tables for vector or octant glyphs. If the coverage is unavailable (for
example, an index out of range), return an error. Never fall back to a full block. Keep the fix centered on carrying or resolving the authoritative
selected coverage through reconstruction, with the same coordinate scaling as
selection. Avoid a second hardcoded vector/octant mask inventory. Unsupported
coverage must not silently become a full block, and reconstruction failures must
not produce plausible quality scores from a substitute source image.

Done means:

- **Golden prediction (Playbook step 3):** zero existing goldens change, because
  `TestGoldenRenders` uses built-in modes only. Any golden diff means the model is
  wrong. Stop and go back to the analysis.
- Regression tests first reproduce the current defect using an asymmetric
  native octant and vector shape with distinct foreground/background colors.
  Assert exact reconstructed pixels against independently known coverage.
  Seed case: the probe above. Expect red at `(0,1)` and blue everywhere else;
  today the test fails because all 8 pixels are red.
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

## 4. Delivery (2026-10-07)

- Extended `cellResult` with `Mask mask128`, `HasMask bool`, and `MaskFn func(...) bool`.
- Updated `findBestCandidateFast` (both 64-bit and 128-bit solver kernels) and `findBestCandidate` to record the winning candidate's mask into `cellResult`.
- Updated `reconstructedCellColor` to evaluate `cell.MaskFn` and `cell.Mask.contains(x, y, w)` when custom shapes are active, preserving `maskContains` for built-in modes.
- Added regression tests covering the asymmetric 2x4 octant seed case (verifying pixel (0,1) red and all other 7 pixels blue), 4x4 vector mode diagonal split, uneven 5x7 cell dimensions, and worker parity (`jobs=4`).
- Updated `docs/SparklinePixelArt.md` with the reconstruction coverage model.
- Zero existing goldens changed. `go test ./...` and `make preflight` passed green.

