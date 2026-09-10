# 040 — Assess halfblock rendering quality at small widths

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#006](006-quality-metrics-and-pixel-art-scaling.md), [#013](013-shared-analysis-grid-for-halfblock-and-quadblock.md), [#038](038-investigate-blocky-sextant-rendering-at-small-widths.md), [#039](039-improve-quad-quality-for-small-pixel-art-renders.md), [RenderingBugPlaybook](../docs/RenderingBugPlaybook.md)

---

## 1. Problem & Motivation

The `half` mode is the simplest and most widely usable image renderer, but it
has not received the exhaustive small-width assessment recently completed for
`six` in #038. At small logo, icon, checkerboard, and transparent-image sizes,
its deliberate `1x2` cell resolution may be the limiting factor, or geometry,
colour averaging, transparency handling, and reconstruction may introduce
avoidable degradation. The comparison with `quad` must distinguish an
inherent half-cell limit from a fixable implementation problem.

This ticket covers the user-facing `half` mode (`halfblock_exact`). It also
provides the halfblock control for comparisons with `quad` and the combined
families; it does not reopen the completed sextant or quad fixes.

## 2. Technical Specification / Assessment Plan

Use the production `v1/halfblock` path and the command render pipeline. Record
for widths 8–20 (including the nearest good and bad width for every suspected
boundary): source dimensions, fitted dimensions, cell dimensions, emitted
`▀`/`▄`/`█`/space decisions, foreground/background escapes, transparent
coverage, reconstructed image dimensions, and metrics from #006. Exercise at
least `assets/cati_0001.png`, `testdata/emojig-icon.svg`, the 20x20 geometric
fixtures, a hard two-colour split, antialiased edges, and transparent borders.

Compare `half` against `quad` and `half/split` at identical fitted terminal
sizes. Check serial and worker rendering, cropped/non-zero image bounds, odd
source dimensions, explicit zoom, and the shared half-cell fit invariant. For
each apparent defect, capture raw ANSI bytes and compare them with
`RenderToImage`; do not treat terminal font appearance as renderer output.

Likely root-cause questions (to answer with measurements, not assumptions):

- Does nearest-neighbour fitting or half-cell rounding discard a meaningful
  row/column at a particular width?
- Are top/bottom colour averages and alpha decisions consistent between ANSI
  output and image reconstruction, including partially transparent cells?
- Does the renderer preserve hard edges and thin features as well as its
  representational `1x2` limit permits, or do sampling footprints amplify
  errors at tiny sizes?
- Do serial and parallel paths make the same cell decisions, and do cropped
  bounds affect sampling?

Potential implementation scope, only if the evidence justifies it, includes a
geometry/sampling correction, transparency/reconstruction correction, or a
small-width-specific quality rule. Do not add a new algorithm merely to make
`half` resemble `quad`; coordinate any shared-footprint work with #013 and
metrics/selection work with #006.

## 3. Acceptance Criteria

1. A reproducible width matrix and fixture set records halfblock geometry,
   cell decisions, ANSI coverage, reconstruction, and quality metrics, with a
   control width for each reported anomaly.
2. The assessment explicitly separates intrinsic `1x2` resolution and terminal
   font effects from actionable renderer defects, and states whether a fix is
   warranted.
3. If a fix is warranted, its scope is limited to the proven root cause and
   includes focused tests for boundaries, transparency, hard edges, odd sizes,
   cropped bounds, and serial/worker equivalence as applicable.
4. Existing halfblock, cross-mode geometry, and golden expectations remain
   stable unless a changed pixel is named and justified.
5. Any changed golden has embedded descriptive metadata and a written
   prediction of the affected files; follow the gates in
   `docs/RenderingBugPlaybook.md` and never use `-update` as diagnosis.

## 4. Verification Guidance

Run targeted `v1/halfblock` and `cmd` tests first, then `go test ./...`,
`go vet ./...`, `make preflight`, and the relevant golden suites. Verify both
the original repro and its near-miss from raw output and reconstructed pixels.
Update the relevant evergreen rendering documentation and this ticket in the
same implementation commit if a fix is made.
