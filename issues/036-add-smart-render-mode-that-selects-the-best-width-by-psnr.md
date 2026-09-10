# 036 — Add smart render mode that selects the best width by PSNR

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [#006](006-quality-metrics-and-pixel-art-scaling.md), [#008](008-spaghetti-code-analysis.md), [#021](021-golden-storage-resolution-all-algos.md), [#025](025-spec-driven-render-modes.md), `cmd/metrics.go`, `cmd/ssim.go`, `internal/imgutil`, `internal/viewgeom`, `cmd/golden_render_test.go`

---

## 1. Problem & Motivation

The renderer currently treats the requested terminal width as the width at which
the selected algorithm must produce its image. For some source images and
algorithms, a nearby width produces a visibly better reconstruction: cell
boundaries, aspect-ratio rounding, and the algorithm's discrete glyph geometry
can make `w-1ch` or another nearby width preserve more source detail than the
nominal `w`-column result. Users currently have to discover and choose such a
width themselves, while still needing the final layout to occupy the requested
number of columns.

Add a spec-driven `smart` render mode (or an equivalent spec-owned mode
property) that wraps the chosen render algorithm. In the first implementation,
given target width `W` in terminal columns, it should render the source at `W`,
then render candidates at `W-1`, `W-2`, and so on, score every candidate with
PSNR against the same reference, and select the highest-scoring candidate. The
selected image must be placed on a `W`-column canvas by filling the unused
columns with spaces and centering the image. This preserves the caller's width
contract while allowing the renderer to use a more favorable nearby geometry.

The initial search unit is a full terminal-character column. The algorithms
already support sub-character rendering steps, so a later extension may search
at the selected algorithm's native cell/sub-cell step instead of raw full-
character columns; that finer search is explicitly out of scope for the first
version.

This complements #006's broader quality-metric/auto-selection work: this ticket
is specifically a bounded width search for one already chosen algorithm, not a
new algorithm-ranking policy across all render modes.

## 2. Technical Specification / Findings

### Candidate search

- **Initial scope: full-character steps.** Candidate widths are integer terminal
  character columns (`W0`, `W0-1`, ...). Do not introduce sub-character
  stepping in the first implementation.
- **Future extension: algorithm steps.** Once the full-character behavior is
  validated, the search may use the selected algorithm's native cell/sub-cell
  width step. That step must come from the authoritative geometry/spec rather
  than from a hardcoded constant, and the final output must still honor the
  requested terminal-column canvas width.
- Treat the requested width as `W0`; the `W0` render is always evaluated and is
  the fallback if no smaller candidate can be scored.
- Evaluate integer widths `W0, W0-1, ...` down to the inclusive lower bound
  `ceil(0.90 * W0)`, described as a maximum reduction of 10% of the final
  `W0`-image width. Clamp the lower bound to at least one usable render column.
- For the initial implementation, candidate widths are terminal columns, not
  raw pixels. Each candidate must
  pass through the selected mode's existing spec-defined cell geometry,
  aspect-ratio, crop/fit, and sizing path; do not duplicate those constants in
  Go.
- The same source/reference definition and PSNR color/alpha treatment must be
  used for every candidate. The comparison must measure rendered reconstruction
  quality, not merely the resized viewport, so a candidate cannot win because
  it was compared at a different pixel resolution.

### Selection and layout

- Select the maximum PSNR. Define a deterministic tie-break, preferably the
  largest width (closest to the user's request), and test it.
- If `W0` is zero/unspecified, retain the existing width derivation before
  entering smart mode. If the requested width is one column, evaluate only
  `W0`; never produce a zero- or negative-width candidate.
- Preserve the selected candidate's rendered height and existing half-cell or
  transparent-row semantics. This feature changes horizontal selection and
  horizontal padding only; it must not stretch the winning image vertically to
  fill the canvas.
- Let `imageWidth` be the actual visible width of the selected render after
  mode alignment. Add exactly `W0-imageWidth` terminal columns of spaces, with
  left padding `floor((W0-imageWidth)/2)` and the remainder on the right.
  Thus odd leftover space has a deterministic one-column bias and every output
  row occupies exactly `W0` columns.
- Center the selected image in the target canvas, including ANSI/transparent
  output paths as appropriate. Padding must not alter PSNR, source colors, or
  the selected candidate's glyph content.

### Spec and API boundaries

- Add the mode/behavior to the authoritative render-mode spec and schema, with
  a clear way to identify the wrapped base algorithm and the smart-search
  policy. Avoid hardcoded mode names, aliases, width bounds, or labels in Go.
- Decide and document how users select the base algorithm (for example, a
  smart wrapper around the currently selected mode versus one spec entry per
  algorithm). The implementation must not silently change the base algorithm.
- Keep existing non-smart modes byte-for-byte and geometrically unchanged.
- Reuse or factor the existing PSNR/SSIM quality pipeline in `cmd` and the
  existing fit/viewport geometry helpers. If PSNR is not yet exposed as a
  reusable scorer for this path, add the smallest reusable pure helper and
  document its reference normalization, peak value, and behavior for identical
  and zero-error images.
- Bound the extra work to at most 11 candidate renders per frame. Interactive
  and video paths need an explicit performance policy: either cache/reuse
  candidates where safe or make smart mode opt-in and skip/reduce its search
  while video is playing, consistent with the existing quality-metric skip
  behavior.

## 3. Implementation & Verification Plan

- Add spec and schema entries plus loader/integrity coverage proving the smart
  mode references a defined base mode/policy and has valid search parameters.
- Implement a candidate generator/scorer that covers `W0`, the 10% lower
  bound, widths below one cell, odd/even target widths, and cases where mode
  alignment means the actual image width differs from the requested candidate.
- Add focused geometry/layout tests asserting exact target width, centered
  padding, deterministic odd-space bias, preserved height, and no zero-width
  candidates.
- Add PSNR selection tests using deterministic fixtures where the winning
  candidate is neither the first nor the last candidate, where `W0` wins, and
  where scores tie. Include a regression that proves all candidates are scored
  against the same normalized reference and that the selected image is not
  rescored after padding.
- Extend render/golden coverage for at least one representative base algorithm
  and source fixture. Store the requested width, candidate width, selected
  width, PSNR, and smart-policy parameters as descriptive PNG `tEXt` metadata
  (or the repository's equivalent test metadata), while keeping comparisons
  insensitive to non-rendering metadata per #035. Follow
  `docs/RenderingBugPlaybook.md`; do not regenerate goldens without explaining
  the predicted pixel changes.
- Verify existing modes and all current render sizing/geometry tests remain
  unchanged. Run targeted smart-mode tests, `go test ./...`, `go vet ./...`,
  `make preflight`, and `make install` when implementation is complete.

### Acceptance criteria

1. A user can select the spec-defined smart mode and retain an explicit,
   documented base algorithm.
2. For target width `W0`, exactly the bounded candidate set is considered,
   including `W0`, with no candidate below `ceil(0.90*W0)` except the required
   one-column clamp.
3. Every candidate is rendered through the normal mode geometry and scored by
   the same PSNR reference; the highest score is selected with deterministic
   tie behavior.
4. Output always occupies exactly `W0` terminal columns, with the selected
   image centered and remaining columns filled with spaces. Height and
   transparency/half-cell behavior remain correct.
5. Tiny widths, odd leftover padding, equal scores, invalid/empty sources, and
   video/interactive performance behavior are covered by tests or explicitly
   rejected with a documented error.
6. Existing modes have no output or golden regressions, and the verification
   commands above pass.
