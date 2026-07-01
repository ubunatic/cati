# 022 — SVG: fixed 2048px rasterization ceiling, no target-size scaling

**Status:** ✅ Closed
**Refs:** [System.md — External Rasterization for Non-Raster Formats (SVG)](../docs/System.md#external-rasterization-for-non-raster-formats-svg), analogous to [#005](005-video-ffmpeg-scaling.md)

---

## Problem

`halfblock.RasterizeSVG` always rasterizes at a fixed `-w 2048` long-edge cap,
then relies on `LoadImage`'s callers (`scaleToFit`, browser thumbnails) to
downscale. This is the same class of problem #005 identified for the video
pipe: rasterizing/decoding at a resolution unrelated to the actual terminal
display target wastes CPU/memory on every load, and — unlike raster
PNG/JPEG stills, which have a fixed native resolution — SVG's "native"
resolution is a free parameter we chose without knowing the caller's target
size.

`LoadImage(path string) (image.Image, error)` has no width/height parameter,
so `RasterizeSVG` cannot currently ask `rsvg-convert` to render at the
caller's actual target size even if it wanted to.

## Impact

- A single large SVG loaded for a 40×20-cell terminal render still costs a
  full 2048px-edge rasterization.
- Conversely, an interactive-mode zoom past the point where 2048px source
  detail runs out will show blur/pixelation, same as any raster image zoomed
  past its native resolution — the vector source *could* avoid this by
  re-rasterizing at the zoomed size, but nothing in the current pipeline does
  that.

## Possible Fix

Mirror #005's proposed direction: thread target pixel dimensions through to
the rasterization call rather than decoding at a fixed size and scaling
after. Concretely this likely means giving `LoadImage`/`RasterizeSVG` (or a
new SVG-specific entry point) an optional target-size parameter that callers
who know their render dimensions (`cmd/thumbqueue.go`, `cmd/interactive.go`)
can supply, falling back to the current fixed cap for callers that don't
(e.g. static single-shot render before layout is known).

Not addressed now because `LoadImage`'s no-width/height signature is shared
by every caller (PNG/JPEG/video), and changing it is a larger, cross-cutting
change out of scope for adding baseline SVG support.

## Downstream impact (confirmed in mdview)

`mdview print` hit this directly: it sizes rendered images by scaling
`img.Bounds()` against the terminal's column budget, so any correctly-sized
small SVG (e.g. a 120px icon) still reported `img.Bounds()` as ~2048px wide
and always hit the max-column cap — every SVG rendered at full terminal
width regardless of its declared size or the caller's `--image-scale`.

Workaround applied on the mdview side (not a fix to cati): before scaling,
mdview now calls `halfblock.ProbeSVGDimensions(path)` and uses the SVG's own
declared `width`/`viewBox` as the "source width" for its column-budget math,
falling back to `img.Bounds()` only if the probe fails. This avoids the
wrong-size-hint problem for *scaling decisions*, but does nothing for the
underlying waste this issue describes (mdview still gets back a full
~2048px-edge raster for a 120px icon and immediately downscales it) — the
real fix here is still to thread a target size into rasterization, per
"Possible Fix" above. Once `LoadImage`/`RasterizeSVG` accepts a target size,
mdview's `ProbeSVGDimensions` workaround can likely be dropped in favor of
just asking cati to rasterize at the render size directly.

## Resolution

Implemented target-size SVG rasterization:

- Added `halfblock.RasterizeSVGWithTarget(path, maxWidth, maxHeight)`.
- Added `halfblock.LoadImageWithTarget(path, maxWidth, maxHeight)`.
- Kept `LoadImage(path)` and `RasterizeSVG(path)` backwards-compatible by
  preserving the 2048px long-edge fallback when no target box is supplied.
- Wired static `cati` rendering to compute the renderer pixel target from the
  SVG's declared dimensions before loading.
- Wired browser thumbnails to rasterize SVG thumbnails directly to their
  preview target.
- Updated SVG dimension probing to resolve absolute SVG/CSS units the way a
  browser does (`1in = 96px`, so `5cm ≈ 189px`) and to derive a missing
  dimension from `viewBox` aspect ratio.
- Added tests for target-size aspect preservation and, when `rsvg-convert` is
  available, end-to-end SVG raster bounds.

The remaining interactive zoom enhancement is intentionally separate:
`catiplay` still loads a single raster image at viewer open time. Re-rasterizing
SVGs dynamically after zoom changes would require preserving the source path in
viewer state and replacing the loaded image on zoom/resize events.
