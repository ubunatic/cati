# 034 — quadblock and sextant have no image loader, must import halfblock.LoadImage

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `v1/halfblock/png.go`, `v1/halfblock/svg.go`, `v1/quadblock`, `v1/sextant`

---

## 1. Problem & Motivation

Found while integrating `v1/quadblock` and `v1/sextant` as terminal-image renderers
into a sibling project (`dai`). Image loading (`LoadImage`, `LoadImageWithTarget`,
`RasterizeSVG`, `RasterizeSVGWithTarget` — SVG-via-`rsvg-convert`) is exported only
from `v1/halfblock` (`v1/halfblock/png.go`, `v1/halfblock/svg.go`). Neither
`v1/quadblock` nor `v1/sextant` exports an equivalent loader.

Confirmed in-repo: `v1/quadblock/render.go` and its tests import `ubunatic.com/cati/v1/halfblock`
solely for image loading, and every internal caller (`cmd/root.go`, `cmd/play.go`,
`cmd/interactive.go`, `cmd/thumbqueue.go`, and their tests) calls `halfblock.LoadImage`
regardless of which render mode (`half`, `quad`, `six`, `spark`) is actually selected —
`v1/sextant` and `v1/quadblock` have no `LoadImage`/`RasterizeSVG` of their own.

This works correctly (loading is renderer-agnostic — decode once, hand the
`image.Image` to whichever `Render`/`RenderToGrid` the caller wants), but it is a
surprising cross-package dependency for a caller who wants only `v1/quadblock` or
`v1/sextant` without pulling in `v1/halfblock` as an implicit "core" package. Nothing
in `v1/quadblock`'s or `v1/sextant`'s package doc comments points a new caller at
`halfblock.LoadImage` — a reasonable first guess (`quadblock.LoadImage`) doesn't exist
and gives no hint where to look.

## 2. Technical Specification / Findings

This is very likely intentional (avoid duplicating decode/SVG-rasterization logic
across three renderer packages), not a design flaw. `v1/core` already exists as a
shared package (imported by `v1/sextant`) and could be a natural home if image loading
were ever moved, but that's a larger refactor with unclear payoff for a single function
family — not proposed here.

## 3. Implementation & Verification Plan

Low-effort fix: add a one-line doc comment to `v1/quadblock`'s and `v1/sextant`'s
package doc (or `Render`'s doc comment) pointing callers at `halfblock.LoadImage` /
`halfblock.RasterizeSVG` for image decoding, e.g.:

> Image loading is not duplicated in this package — use `halfblock.LoadImage` or
> `halfblock.RasterizeSVG` to decode a source image before calling Render.

No code or test changes required; verify by reading the rendered `go doc` output for
both packages after the doc comment change confirms the pointer is visible.
