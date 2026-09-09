# 033 — Render() unconditionally emits erase-line+CR prefix, breaks composed layouts

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Moderate
**Category**: Bug
**Related**: `v1/halfblock/render.go`, `v1/quadblock/render.go`, `v1/sextant/render.go`

---

## 1. Problem & Motivation

Found while integrating `v1/halfblock`, `v1/quadblock`, and `v1/sextant` as a
terminal-image-rendering library into a sibling project (`dai`), which composes a
rendered image alongside other content on the same row (a side-by-side column layout).

Every line written by `halfblock.Render`, `quadblock.Render`, and `sextant.Render`
is unconditionally prefixed with `ansiLinePrefix` (`"\x1b[2K\r"` — erase-entire-line,
then carriage-return to column 0):

- `v1/halfblock/render.go:29-31` (`ansiEraseLine`, `ansiCarriageReturn`, `ansiLinePrefix`),
  written per-line in `Render` (confirmed by `TestRender_TransparentImage`'s comment:
  "ansiEraseLine (\x1b[2K) is always emitted per line — that's expected.")
- `v1/quadblock/render.go:31` — same constant, same unconditional per-line emission.
- `v1/sextant/render.go:20` — same constant, same unconditional per-line emission.

This is correct and desirable for a full-screen, standalone redraw (cati's own CLI/TUI
use case — clearing stale content from a previous frame at the same terminal position).
It actively corrupts output for any caller that writes a rendered image next to other
content on the same terminal row: the embedded `\r` resets the cursor to column 0
mid-line, and `\x1b[2K` erases the *entire* line (not just from the cursor onward),
wiping out whatever the caller already wrote on that row before the image.

Confirmed byte-for-byte via `cat -v` on real output from all three packages during the
`dai` integration — every rendered line starts with `^[[2K^M` (`cat -v` rendering of
`\x1b[2K\r`).

## 2. Technical Specification / Findings

`dai` worked around this on the caller side by stripping the fixed-length prefix from
every line before compositing (`internal/iconrender.Lines()`, constant `linePrefix`,
regression test `TestLinesStripsCatiLinePrefix` — read-only reference, not part of this
repo). That workaround is fragile: it silently breaks if `ansiLinePrefix`'s bytes change,
and every downstream caller doing composed-layout rendering has to reinvent it.

None of the three `Render(w io.Writer, img image.Image, cols int, opts Options) error`
signatures expose a way to opt out, and none of the package doc comments mention that
output assumes standalone, full-line use (e.g. `v1/quadblock`'s package doc describes
the cell encoding but says nothing about the erase/CR framing).

## 3. Implementation & Verification Plan

Two independent, non-exclusive fixes — pick one or both:

1. **Make the erase/CR prefix opt-in/opt-out via `Options`** (e.g. `Options.NoLinePrefix`
   or inverted `Options.StandaloneRedraw`), defaulting to today's behavior so the CLI
   binaries (`cati`, `catiplay`, `catibrowse`) are unaffected, and reserve the plain
   `Render` entry point (or a new sibling helper) for composed-layout callers who pass
   the opt-out.
2. **Document the assumption** in the package doc comments of `halfblock`, `quadblock`,
   and `sextant` (`Render`'s doc comment specifically) — state plainly that each output
   line assumes it owns the entire terminal row (via the erase+CR prefix) and is not
   safe to compose with other content on the same row without stripping it.

Verification: a small unit test per package asserting `Options{NoLinePrefix: true}` (or
equivalent) omits `ansiEraseLine`/`ansiCarriageReturn` from `Render`'s output, alongside
the existing `TestRender_TransparentImage`-style byte-content assertions. No golden
image impact expected — this only affects the ANSI control-sequence framing, not pixel
content.

This is a real, confirmed defect for any non-standalone caller, but has a known
one-line client-side workaround (strip the fixed prefix), hence low priority despite
moderate severity (visual corruption when triggered).
