---
title: Video & Audio Pipeline
parent: System.md
weight: 2
---

# Video & Audio Pipeline


This document describes the video probing, decoding, streaming, and audio playback pipeline in Cati.

---

## 1. Video Detection & Probing

Cati detects video files by checking file extensions against a fixed set:

```go
var VideoExts = map[string]bool{
    ".mp4": true, ".webm": true, ".mkv": true, ".mov": true, ".avi": true,
}
```

Three ffprobe helpers live in `internal/halfblock/video.go`:

| Function | ffprobe query | Returns |
|---|---|---|
| `ProbeVideoFPS` | `stream=r_frame_rate` | Native FPS as `float64` (parses `num/den`) |
| `ProbeVideoDuration` | `format=duration` | Duration in seconds |
| `ProbeVideoDimensions` | `stream=width,height` | `(w, h int)` for rawvideo frame sizing |

FPS probing happens at stream-open time. If ffprobe is unavailable or fails, playback falls back to 15 fps.

---

## 2. Streaming Architecture

Frames are decoded by a background goroutine and sent over a buffered channel so the main loop never blocks on ffmpeg I/O.

```
OpenVideoStream(ctx, path, displayFPS)
  │
  ├─ ProbeVideoDimensions → (w, h)
  │
  ├─ ffmpeg -v quiet -i path
  │          [-vf fps=N -threads 4]   ← rate limit + thread cap
  │          -f rawvideo -pix_fmt rgba pipe:1
  │
  └─ goroutine: io.ReadFull(stdout, buf[w*h*4])
                → image.NewRGBA, copy(img.Pix, buf)
                → ch <- img
```

### Why rawvideo instead of PNG pipe

The original pipeline used `-f image2pipe -vcodec png pipe:1` and `png.Decode` in Go. PNG encoding (ffmpeg side) and decoding (Go side) both consumed significant CPU — ffmpeg routinely spawned 50+ threads at 600%+ CPU on a home video.

Rawvideo (`-f rawvideo -pix_fmt rgba`) eliminates all compression/decompression: ffmpeg copies pixels directly to the pipe, Go reads a fixed `w*h*4` byte block per frame with `io.ReadFull`. Per-frame memory is the same (both approaches yield an uncompressed `image.Image`); the difference is CPU.

**Current caveat:** frames are piped at source resolution (e.g. 1920×1080 = 8.3 MB/frame). At 30 fps that is ~250 MB/s through the pipe, which Linux handles comfortably (loopback pipe bandwidth ≫ 1 GB/s), but is wasteful. The planned fix is ffmpeg-side scaling (`-vf scale=W:H`) so frames arrive pre-scaled to terminal dimensions. See [issue 005](../issues/005-video-ffmpeg-scaling.md).

### FPS rate limiting

`-vf fps=N` tells ffmpeg's fps filter to select the nearest source frame for each output timestamp. A 30 fps source at `displayFPS=15` emits every 2nd frame in the same real time — natural playback speed is preserved. `-threads 4` caps the decoder thread pool.

Without rate limiting ffmpeg decodes at full CPU speed regardless of the consumer's tick rate, causing the stale-frame accumulation described in §3.

---

## 3. Playback Loop — One Frame Per Tick

Both `playVideos` and `interactiveVideo` use a ticker at `displayFPS`. The key rule: **consume exactly one frame per ticker tick**.

```go
case <-ticker.C:
    select {
    case img, ok := <-frames:
        if !ok { /* handle end */ }
        lastFrame = halfblock.ScaleToFit(img, cols, rows)
    default:
        // no frame yet — keep showing lastFrame
    }
    if lastFrame != nil {
        halfblock.Render(os.Stdout, lastFrame)
    }
```

### Why this matters

ffmpeg does not pace its output in real time — it decodes and pipes frames as fast as the CPU allows, then blocks when the channel buffer (size 8) fills. A separate `frames` case in the outer select would drain the buffer between ticks, advancing `lastFrame` 8+ frames per tick period and causing apparent fast-forward. The old "stale drain" loop (reading `len(frames)-1` extras after each render) had the same effect.

The non-blocking inner select gives the ticker exclusive control over frame advancement. ffmpeg's buffer fills, it blocks, and consumption naturally paces to `displayFPS`.

---

## 4. Play-Once vs Loop

`cati -p video.mp4` plays each video in the argument list exactly once, then exits. There is no implicit looping. With multiple files, they play sequentially; `videoIdx` advances without wrap-around.

`cati -i video.mp4` (interactive mode) loops by default — when the frame channel closes (`!ok`), `restartStream()` reopens the stream. But if the video ends while paused, the stream is set to `nil` (disabling that select case), the last frame is held, and `videoEnded = true` is set. The next play action (space or play button) calls `restartStream()`.

---

## 5. Input Resilience in Interactive Video

Mouse events flood the `inputs` channel (cap 32, ~800 bytes). Two safeguards:

1. **Full drain at loop top** — a labeled `for`/`select` empties the entire buffer before entering the blocking select. A single-token drain allowed `frames` to starve `inputs` on a burst.
2. **Buffer-full abort** — if `len(inputs) == cap(inputs)` at the top of the loop, the function returns an error. This means the goroutine was blocked (sending tokens with nowhere to put them) long enough to fill 32 slots — a genuine hang, not a burst.

---

## 6. Audio Playback

Audio is handled by the `internal/audio` package.

### Probing

```go
audio.HasAudio(path) // ffprobe -select_streams a:0 -show_entries stream=...
```

Returns `true` if the file contains at least one audio stream.

### Playback backend: ffplay

Audio is played via `ffplay -v quiet -nodisp -vn -autoexit path`.

**Why ffplay, not ffmpeg→aplay:** when cati holds the terminal in raw mode, the process runs without a controlling TTY. `aplay` (and similar ALSA tools) fail silently in this context. `ffplay` manages its own audio session and works correctly as a subprocess of a raw-terminal process.

### Lifecycle in `playVideos`

```
openAudio(path)  →  audio.Open(ctx, path)  →  ffplay subprocess
stopAudio(p)     →  p.Stop()               →  Kill + wait

Video advances → stopAudio(current), openAudio(next)
```

Audio is not yet wired into `interactiveVideo` (`cati -i video.mp4`).

---

## 7. Render-Pipeline Optimizations for Playback

### Throttled invariant checks (`renderCheckGate`)

`renderChecked` validates every rendered frame by walking the full ANSI output string to count cell widths — an O(output-length) operation that is unnecessary after the first frame passes with stable dimensions.

`renderCheckGate` (in `cmd/render_output.go`) tracks the last check time and the last frame dimensions. `renderCheckedGated` calls the ANSI walk and `validateRenderSize` only when:
- The gate has never fired (first frame always checked), or
- The rendered cell dimensions changed (resize or mode switch), or
- More than `gate.interval` (1 s) has elapsed since the last check.

Both `playImages` and `playVideos` create a gate with `interval = time.Second`.
`interactiveVideo` uses `renderValidatedGated` (which also gates `validateRenderSize`) via the same mechanism.

### Skipping quality metrics while playing (`skipQuality`)

`viewerCore.skipQuality` disables the expensive per-frame quality pipeline in `interactiveVideo`:
- `buildRef` (pyramid downscale of the source region)
- `computeQuality` (render-to-image + SSIM + Sobel + blockiness)

`vc.skipQuality` is `true` while the video is playing. On pause, `setPaused(true)` immediately runs a single quality computation so the hint bar shows accurate SSIM/blockiness values. While playing the hint bar displays the last computed value (frozen), which is acceptable because quality metrics are not meaningful at video frame rates.

`vc.skipQuality` is reset to `false` when the video ends or when the user toggles pause.
