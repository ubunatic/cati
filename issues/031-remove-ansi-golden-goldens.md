# 031 — Remove `.ansi` golden tests (unverifiable byte diffs let a fix go stale)

**Status:** ✅ Closed
**Refs:** [RenderingBugPlaybook.md](../docs/RenderingBugPlaybook.md), `cmd/cli_render_test.go` (removed)

---

## Problem

`TestCLIRender` compared raw ANSI escape bytes against `testdata/**/cli_*.ansi`
golden files. Commit `ce5950f` ("fix(sextant): remove avgRGBA fallback for
opaque cells; scope gap check to opaque rows") correctly stopped painting a
background escape into cells that should be transparent in a trailing partial
row (the sextant counterpart of the still-open #023 transparency bug), but the
3 affected `.ansi` goldens (`demo_verti_20x20/cli_sextant_w5.ansi`,
`demo_vert_split_8x8/cli_sextant_w3.ansi`, `demo_vert_split_8x8/cli_sextant_w5.ansi`)
were never regenerated, so `TestCLIRender` started failing against
already-correct output.

## Root Cause

Byte-exact ANSI escape sequences can't be verified visually — there is no way
to eyeball a `\x1b[48;2;13;13;241m` diff and confirm which side is right. That
made the goldens easy to leave stale after a legitimate rendering fix, and
gave no signal about *which* side (code or golden) was wrong without manual
byte decoding (confirmed here via `cat -v` + diff, isolating the change to the
trailing partial row's edge cells in all 3 failing cases — exactly matching
the already-passing `TestAllRenderModesWidthOneThroughTwentyKeepAspectAndNoGaps`
invariant, which explicitly permits gaps outside `opaqueRows`).

## Fix

Removed `cmd/cli_render_test.go` and all 50 `testdata/**/cli_*.ansi` golden
files. ANSI-format correctness remains covered by two mechanisms that don't
depend on byte-exact snapshots:

- `TestAllRenderModesWidthOneThroughTwentyKeepAspectAndNoGaps` (`cmd/root_test.go`) —
  asserts no terminal-default-background gaps inside `opaqueRows`, across all
  modes and widths 1–20.
- `validateRenderedANSI` (`cmd/render_output.go`) — runs on every real render
  (not just in tests), checking cell-count/row structure.

`TestGoldenRenders` (PNG image goldens, which *are* visually verifiable) is
unaffected and remains the primary golden-image regression test.

## Golden Impact

Removed 50 `.ansi` files; no PNG goldens touched by this change.
