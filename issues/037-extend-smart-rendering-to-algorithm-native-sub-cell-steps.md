# 037 — Extend smart rendering to algorithm-native sub-cell steps

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [#036](036-add-smart-render-mode-that-selects-the-best-width-by-psnr.md), [Spec](../docs/Spec.md), [Sparkline Pixel Art](../docs/SparklinePixelArt.md)

---

## 1. Problem & Motivation

Smart rendering currently searches only whole terminal-character widths. That is
the correct first implementation, but the render algorithms can place detail at
finer native cell/sub-cell steps. Restricting the search to raw terminal columns
can therefore miss a higher-quality reconstruction that still fits the same
requested terminal canvas.

Extend smart rendering so each selected base algorithm can search at its own
authoritative native step size. Preserve the existing user-facing width
contract: the winning image is centered on the requested terminal-column
canvas, and any remaining columns are filled with spaces.

## 2. Technical Specification / Findings

- Retain whole-character stepping as the fallback for algorithms without a
  declared native sub-cell step.
- Define the native step in the render-mode spec/geometry model; do not encode
  algorithm-specific increments or cell dimensions as Go-only constants.
- Generate candidates from the requested width down through the same bounded
  search window as #036 (at most a 10% reduction, with a minimum usable width),
  stepping by the base algorithm's native sub-cell increment.
- Render every candidate through the normal base-algorithm geometry and score
  every candidate against the same normalized reference with PSNR.
- Keep deterministic selection rules: highest PSNR wins, with the widest
  candidate winning ties. Padding and centering happen only after selection and
  must not affect the score.
- Preserve exact terminal-column output width, height, transparent-row
  behavior, and all non-smart mode output.
- Keep smart mode explicitly opt-in where candidate search is expensive; define
  caching or a reduced/disabled policy before enabling it for interactive or
  video playback.

## 3. Implementation & Verification Plan

- Extend the spec schema, loader, and integrity tests with native-step metadata
  and validation for every smart-capable base algorithm.
- Refactor candidate generation so full-column and native-step searches share
  the same lower-bound, clamping, tie-break, and layout logic.
- Add focused tests for half-cell, quad-cell, sextant, and sparkline geometry,
  including fractional/native-step candidates, tiny widths, odd padding, and
  candidates whose aligned visible width differs from the requested candidate.
- Add a representative smart golden with metadata describing base algorithm,
  requested width, candidate step, selected candidate, PSNR, and policy. Keep
  metadata-only differences ignored as established by #035.
- Verify existing modes remain unchanged and run targeted tests, `go test ./...`,
  `go vet ./...`, `make preflight`, and `make install`.

### Acceptance criteria

1. Each supported smart-capable algorithm declares and uses its native
   sub-cell step from the spec/geometry model.
2. Candidate search remains bounded, never emits an unusable width, and falls
   back deterministically to whole-character stepping when no native step is
   available.
3. Candidates are compared using the same PSNR reference, with widest-candidate
   tie-breaking and no scoring contamination from final padding.
4. Output always occupies exactly the requested terminal width and preserves
   existing height, transparency, and non-smart behavior.
5. Tests and at least one metadata-rich golden demonstrate that a native-step
   candidate can win over the whole-column candidates.
