# Small-Width Render Quality Sprint

## Outcome

Issues #038 and #039 were resolved. Sextant now exhaustively selects the
lowest-SSE representable mask, and the user-facing quad mode applies a
conservative halfblock fallback to ambiguous 3+ colour cells.

Issues #040 and #041 then completed the same assessment for halfblock and the
sparkline composites. Halfblock had no general small-width defect, but its
image reconstruction disagreed with opaque ANSI true-color output for partial
alpha. Sparkline had the analogous transparent-coverage mismatch; composite
quality differences across cell geometries remain documented as follow-up
work rather than being treated as selector bugs.

## Flow learnings

- Read-only advisors quickly separated intrinsic sextant resolution limits from
  selector defects and isolated the SplitHalf fallback bypass.
- Golden regeneration must use the same Go toolchain as `make test`; JPEG
  decoder-family goldens are intentionally separate.
- Review caught that a width matrix needs quality assertions, not only dimension
  checks; the final tests compare partial-quad glyph counts and validate both
  renderers across widths 8–20.
- A reviewer caught an evidence mismatch when a claimed 390-case assessment was
  initially represented by a smaller synthetic smoke test. Keeping the matrix
  checked in, naming its exact fixtures, and separating measured metric results
  from smoke assertions makes the ticket reproducible and auditable.

## Verification

`go vet ./...`, `make install`, `make preflight`, and `make test` pass.
