# 022 — SVG: fixed 2048px rasterization ceiling, no target-size scaling

**Status:** 🔴 Open
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
