# Small-Width Render Quality Sprint

## Outcome

Issues #038 and #039 were resolved. Sextant now exhaustively selects the
lowest-SSE representable mask, and the user-facing quad mode applies a
conservative halfblock fallback to ambiguous 3+ colour cells.

## Flow learnings

- Read-only advisors quickly separated intrinsic sextant resolution limits from
  selector defects and isolated the SplitHalf fallback bypass.
- Golden regeneration must use the same Go toolchain as `make test`; JPEG
  decoder-family goldens are intentionally separate.
- Review caught that a width matrix needs quality assertions, not only dimension
  checks; the final tests compare partial-quad glyph counts and validate both
  renderers across widths 8–20.

## Verification

`go vet ./...`, `make install`, `make preflight`, and `make test` pass.
