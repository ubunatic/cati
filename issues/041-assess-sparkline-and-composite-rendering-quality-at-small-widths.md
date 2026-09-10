# 041 — Assess sparkline and composite rendering quality at small widths

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#006](006-quality-metrics-and-pixel-art-scaling.md), [#009](009-explore-more-sparkline-rendering-modes.md), [#013](013-shared-analysis-grid-for-halfblock-and-quadblock.md), [#015](015-spark-bottom-row-halfcell-fit.md), [#017](017-gpu-glyph-mapping-spark-mode.md), [#038](038-investigate-blocky-sextant-rendering-at-small-widths.md), [Sparkline Pixel Art](../docs/SparklinePixelArt.md), [RenderingBugPlaybook](../docs/RenderingBugPlaybook.md)

---

## 1. Problem & Motivation

The sparkline family has more spatially dense glyph choices than native
halfblock or sextant rendering, but it has not had the evidence-based,
small-width assessment completed for `six` in #038. The current user-facing
family includes `half/split`, `spark`, `spark+quad`, `six+half`, and
`spark+six` (`spark_best` is its alias); the library’s `spark/vert` mode is a
useful scalar baseline. Composite modes can expose mismatches between their
candidate masks, colour selection, geometry, ANSI emission, and image
reconstruction even when each constituent glyph family works in isolation.

The goal is to identify justified fixes or document representation limits, not
to maximize candidate count or duplicate the Unicode research in #009.

## 2. Technical Specification / Assessment Plan

Exercise the production `v1/sparkline` path and command pipeline at widths 8–20,
with exact-width and near-boundary controls. Use the cati and emojig logos,
20x20 geometric fixtures, scalar gradients, hard horizontal/vertical splits,
diagonals, antialiased edges, and transparent borders. Compare all family
members at the same terminal dimensions and record mode geometry (`2x2`,
`4x8`, `2x6`, `4x24`), fitted dimensions, candidate family, selected rune,
foreground/background colours, transparent coverage, and reconstructed pixels.

Measure SSE/SSIM, blockiness, edge continuity, and any agreed composite score
from #006. Compare raw ANSI output with `RenderToImage`, including transparent
cells and background escapes. Check serial versus worker rendering, cropped
non-zero bounds, odd dimensions, explicit zoom, native smart-width selection,
and the shared continuous fit/half-cell invariant. Run the relevant decoder
families where source decoding can affect golden bytes.

Likely root-cause questions (to answer with production probes and numbers):

- Do `spark`, `spark+quad`, and `spark+six` choose masks consistently with the
  source footprint they reconstruct, or do upsampled candidate masks distort
  the scoring evidence?
- Do combined modes introduce colour bleed, transparent overpaint, unstable
  near-ties, or glyph-family discontinuities at small widths?
- Does a mode’s native cell geometry cause a different fit boundary or dropped
  detail than an equivalent constituent renderer, despite the shared geometry
  contract?
- Are `half/split` and `six+half` useful quality choices, or do they duplicate
  existing half/sextant behaviour with a measurable regression?
- Does smart selection choose a perceptually better family on small pixel art,
  or can #006’s metric work make a wrong choice look numerically acceptable?

Potential implementation scope, only where measurements identify a defect,
includes candidate-mask/scoring changes, ANSI/reconstruction parity, geometry
or transparency corrections, deterministic tie handling, or a narrowly scoped
smart-selection adjustment. Keep new glyph research in #009, shared
half/quad analysis in #013, and performance/GPU exploration in #017. Do not
silently redefine a mode’s user-facing geometry.

## 3. Acceptance Criteria

1. A reproducible width/fixture matrix covers every current sparkline-family
   mode and records the selected glyph and colours, geometry, reconstruction,
   and quality metrics, with controls for each suspected boundary.
2. The report distinguishes intrinsic cell-resolution and font-coverage limits
   from actionable scoring, fitting, transparency, or reconstruction defects.
3. Any proposed fix has a stated root-cause hypothesis supported by measured
   ANSI/image evidence, focused deterministic tests, and explicit non-goals.
4. Composite modes preserve serial/worker equivalence, smart-mode contracts,
   transparent coverage, and existing cross-mode geometry invariants.
5. Golden changes are predicted before regeneration, limited to justified
   files, carry descriptive PNG metadata, and follow
   `docs/RenderingBugPlaybook.md`; never regenerate goldens merely to clear a
   failure.

## 4. Verification Guidance

Run targeted sparkline and `cmd` tests, then `go test ./...`, `go vet ./...`,
`make preflight`, and the applicable golden suites. Verify raw ANSI and
reconstructed pixels for each original repro and near-miss. If implementation
changes result, update `docs/SparklinePixelArt.md` and this ticket in the same
commit; otherwise record the measured limits and leave goldens untouched.

## 5. Assessment and Resolution

The production width matrix covered widths 8–20 for cati, emojig, and four
geometric fixtures across `half/split`, `spark`, `spark+quad`, `six+half`, and
`spark+six`. All 390 serial/4-worker ANSI comparisons were byte-identical, and
all modes preserved requested widths and row boundaries. At 20 columns the
geometric spark and spark+six reconstructions were lossless; diagonal and
circle error at 10 columns matched the intrinsic cell-resolution limit.

Composite selection differed materially by fixture (spark versus spark+quad
differed at 10/13 checker widths), while an external RMSE control showed that
the larger spark+six candidate family can still score worse across different
cell geometries. This is evidence for future shared-metric/grid work, not a
proven defect in the current production metrics; that work remains out of
scope here.

A concrete defect was reproduced in `RenderToImage`: it discarded every
source-transparent pixel even when the selected ANSI glyph foreground or
background escape painted that region. The fix centralizes emitted-coverage
reconstruction for serial and worker paths and rejects background-only
non-space cells whose glyph pixels would depend on the terminal's unknown
default foreground. Tests cover foreground/background/unpainted coverage,
transparent selected regions, and background-only prevention. Existing opaque
goldens and JPEG decoder-family goldens are predicted unchanged, so none were
regenerated.

The reproducible smoke matrix is checked in as
`v1/sparkline/quality_assessment_test.go`; it exercises cati, four geometric
fixtures, and a transparent synthetic fixture across all five current
user-facing family modes at widths 8–20, asserting requested geometry plus
serial/worker ANSI and reconstruction parity. The emojig SVG remains covered
by the command/golden pipeline because this package-level matrix intentionally
avoids adding an SVG decoder dependency.
