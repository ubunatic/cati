# 058 — cati --bench reports fast/simple output mismatches

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Testing
**Related**: 056 (`--bench`), 055 (known `extractLumaFlat` fast-path drift), `core.Fastpath`, `cmd/media_benchmark.go`, commit 22cd815 (halfblock fast paths, Refs PR #10)

## /goal

`cati --bench <mediafile>` already renders every mode with fast paths on and
off. It must also compare the two outputs per mode and report any difference,
so a fast path that is not output-identical is caught on real assets, not only
by unit parity tests.

Done when:

- Each bench row shows whether fast and simple output are identical for that
  asset (e.g. `ok` / `DIFF`), with a short summary of the difference (bytes or
  cells differing, first differing position).
- A mismatch is clearly flagged in the summary and yields a non-zero exit
  status (or a documented flag controls this).
- Videos compare per frame (at least a count of differing frames).
- A test covers both an identical and a forced-mismatch case.
- `docs/Testing.md` mentions the check.

## Notes

- Compare only fast vs simple within the same mode; different modes are
  expected to differ.
- Known drift (issue 055) should surface as a reported diff, not be hidden.
- Re-verify the bench code against current `main` before starting.

## Resolution

Delivered in 6f329f7: `Output parity` column (ok / DIFF with byte count + first diff position), per-frame SHA-256 comparison for video, non-zero exit on mismatch, image identical/mismatch tests, docs/Testing.md. Verified live: photo at 120x50 and a 10-frame testsrc video, all modes `ok`. Video forced-mismatch test added in 4044eb8.
