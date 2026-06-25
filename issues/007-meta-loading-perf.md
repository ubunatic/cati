# 007 — Media Metadata: ffprobe called for images on hover

**Status:** 🔴 Open  
**Refs:** [Input.md](../docs/Input.md), [Design.md §3.4](../docs/Design.md#34-speclabelsyaml--non-button-strings-only)

---

## Context

`cmd/media_meta.go:loadMediaMeta` calls `halfblock.ProbeMediaMeta(path)` for **every** file type
(images and videos) to populate `meta.*` template variables. `ProbeMediaMeta` spawns an `ffprobe`
subprocess and parses its JSON output.

Before this, ffprobe was only invoked for video files (to extract frames and duration).
Now it is also called for every image the user hovers over in the browser — even if the image
has no embedded metadata tags.

## Impact

- `ffprobe` latency: 100–300 ms per file on typical hardware.
- The load is async (goroutine + channel), so it does **not** block the UI or redraw.
- Results are cached per path (`metaCache map[string]*MediaMeta`) — ffprobe is called at most
  once per file per session.
- For sessions with many unique files, first-hover latency will trigger many concurrent ffprobe
  subprocesses.

## Options

1. **Skip ffprobe for non-video images** — call `image.DecodeConfig` only (already done for
   source dims) and leave audio/video/codec/tag fields empty for images. Less metadata but
   no ffprobe subprocess per image. EXIF tags (author, date, location, camera) would be lost
   for JPEG images.

2. **Lazy ffprobe for images** — only call `ProbeMediaMeta` for images if the `image.DecodeConfig`
   path returns successfully AND the file extension is JPEG/TIFF (common EXIF carriers). Skip for
   PNG, WebP, GIF.

3. **Current approach (keep as-is)** — async, cached, user only sees populated metadata after
   the first hover. Most users will browse at a pace slower than ffprobe latency.

4. **Use `exiftool` instead** — faster for images, but adds another runtime dependency.

## Recommendation

Option 2 is a low-risk middle ground: call ffprobe only for video and EXIF-capable image formats
(jpeg, tiff, heic). Implement a `shouldProbeRich(ext string) bool` helper in `media_meta.go`.
