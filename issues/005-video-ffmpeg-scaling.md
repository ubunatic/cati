# 005 — Video: ffmpeg-side scaling for rawvideo pipe

**Status:** 🔴 Open  
**Refs:** [Video.md §2](../docs/Video.md#2-streaming-architecture)

---

## Problem

`OpenVideoStream` pipes raw RGBA frames at the source video's native resolution.
For a 1920×1080 file at 15 fps that is `1920 × 1080 × 4 × 15 ≈ 124 MB/s` through the pipe.
The Go consumer then scales each frame to terminal dimensions with `halfblock.ScaleToFit`.

This is wasteful: the terminal might be 200×50 cells (200×100 pixels for halfblock), so
Go scales an 8.3 MB frame down to a 80 KB image on every tick.

## Proposed Fix

Pass the target pixel dimensions to `OpenVideoStream` and add a `-vf scale=W:H` filter:

```
ffmpeg … -vf fps=N,scale=W:H -f rawvideo -pix_fmt rgba pipe:1
```

Frame size drops from `srcW*srcH*4` to `W*H*4`. For a 200×100 terminal target:
- Before: 8.3 MB/frame × 15 fps = 124 MB/s
- After:  80 KB/frame × 15 fps = 1.2 MB/s

`ScaleToFit` on the consumer side can be removed for the streaming path.

## Complication

`scale=W:H` stretches to exactly W×H, ignoring aspect ratio.
Use `scale=W:H:force_original_aspect_ratio=decrease` (or `scale=W:-2`) and accept
that the caller must re-query frame dimensions from the channel item (it is an `*image.RGBA`
with bounds set to the actual output size).

Alternatively: compute aspect-correct pixel dimensions before calling `OpenVideoStream`
(replicating the `ScaleToFit` math) and pass exact W×H.

## API Change

```go
// Before
func OpenVideoStream(ctx context.Context, path string, displayFPS float64) (...)

// After (proposed)
func OpenVideoStream(ctx context.Context, path string, displayFPS float64, pixW, pixH int) (...)
// pixW/pixH == 0 → no scaling (source resolution)
```

Call sites must supply terminal pixel dimensions:
- `playVideos`: `pixW = cols, pixH = rows*2` (halfblock: 2 px per terminal row)
- `interactiveVideo`: `pixW = termCols, pixH = viewRows*2` where `viewRows = termRows - viewerChromeRows`

On terminal resize, the stream must be restarted (already the case in `interactiveVideo`).
