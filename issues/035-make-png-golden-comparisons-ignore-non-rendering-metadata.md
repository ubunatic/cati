# 035 — Make PNG golden comparisons ignore non-rendering metadata

**Status**: Closed (Verified with metadata isolation and negative difference tests in `cmd/golden_render_test.go`)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Infrastructure
**Related**: [#032](032-jpeg-golden-toolchain-drift.md), [RenderingBugPlaybook.md](../docs/RenderingBugPlaybook.md), `cmd/golden_render_test.go`

---

## 1. Problem & Motivation

The PNG golden render suite in `cmd/golden_render_test.go` should fail when the
rendered pixels regress, but it can also fail when a PNG changes only in
non-rendering file metadata. For example, changes to PNG `tEXt` chunks or other
ancillary chunks may be reported as golden drift even though the decoded image
is visually identical.

Recent `make test` failures exposed adjacent golden fragility: #032 traced
JPEG-sourced pixel drift to Go toolchain differences. That issue pins the
toolchain and updates affected images, but it does not establish whether the
comparison and loading path is unnecessarily sensitive to PNG metadata. This
ticket is to investigate and, if confirmed, remove that source of false
failures while retaining meaningful pixel and geometry regression detection.

## 2. Technical Specification / Findings

Determine precisely what `goldenLoad`, `testhelper.SavePNG`, and
`goldenEqual` compare today, including whether comparison is performed on
decoded `image.Image` values or on encoded PNG bytes and whether ancillary
chunks can affect the decoded bounds, color model, alpha values, or pixels.

Assess a comparison strategy that treats non-rendering metadata and ancillary
chunks as irrelevant but continues to detect differences in image dimensions,
pixel values, transparency, and any other properties that affect rendered
output. Do not mask actual decoder/toolchain pixel drift such as the one
documented in #032, and do not weaken the existing golden-change evidence
requirements in `docs/RenderingBugPlaybook.md`.

Record any uncertainties about PNG chunk handling, color models, or the
test-helper encoder rather than assuming that all metadata is harmless.

## 3. Implementation & Verification Plan

- Add focused tests proving that a golden with changed `tEXt` metadata and/or
  unrelated ancillary chunks compares equal when its decoded pixels are
  unchanged.
- Add focused negative tests proving that changed dimensions, alpha, and
  representative pixel values still fail; include a case that would catch
  one-channel/one-LSB pixel drift so #032-style changes remain visible.
- Implement the narrowest comparison or loading change in the golden test
  support, documenting why metadata is excluded and which image properties
  remain significant.
- Run the targeted golden tests and the full repository test suite. Confirm
  that no existing golden files need regeneration solely because metadata is
  being ignored, and follow the rendering playbook if any pixel golden does
  change.

The issue is P2: false failures create recurring maintenance cost and obscure
real rendering regressions, but the current suite still detects pixel changes
and #032 has a pinned-toolchain workaround.
