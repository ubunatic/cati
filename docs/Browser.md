---
title: Interactive Grid Browser
parent: System.md
weight: 3
---

# Interactive Image & Video Browser


This document describes the design and implementation of the multi-image interactive grid browser in Cati.

---

## 1. Screen Layout & Boundaries

The browser occupies the entire terminal window. It dynamically partitions terminal lines into three zones based on terminal rows and columns resolved via `TIOCGWINSZ`.

```
+-------------------------------------------------------------+ -- row 1
|  Title & Page Indicator (e.g. Page 1/3 (1-6 of 15))         |
+-------------------------------------------------------------+ -- row 2
|                                                             |
|   +-------------------+     +-------------------+           |
|   |                   |     |                   |           |
|   |     Thumbnail     |     |     Thumbnail     |           |
|   |                   |     |                   |           |
|   +-------------------+     +-------------------+           |
|   | [ filename.png ]  |     |   filename.jpg    |           |
|   +-------------------+     +-------------------+           |
|                                                             |
+-------------------------------------------------------------+ -- row (termRows-2)
|  [◀ Prev]  [Next ▶]  [ℹ About]  [✖ Quit]                    | -- row (termRows-1) (Buttons)
+-------------------------------------------------------------+ -- row (termRows) (Status)
|  Quick Help / Key Bindings Bar                              |
+-------------------------------------------------------------+
```

### Dynamic Grid Sizing
*   **Columns**: Automatically drops from `3` to `2` or `1` columns if the terminal width is too narrow (`< 60` or `< 40` characters).
*   **Rows**: Automatically shifts from `2` rows to `1` row if the terminal height is limited (`< 14` lines).
*   **Cell Width & Height**:
    ```go
    cellW = (termCols - (gridCols-1)*gapX) / gridCols
    cellH = (gridRowsLimit - (gridRows-1)*gapY) / gridRows
    ```
    Where `gridRowsLimit = termRows - marginTop - marginBottom`.

---

## 2. Thumbnail Composition & Anti-Flicker Rendering

Rendering individual grid cells using multiple cursor jumps and partial redraws causes screen tearing and massive blinking. Cati resolves this by rendering on a single composite canvas.

```
       +---------------------------------------------+
       |             Composite Canvas                |
       |               (image.RGBA)                  |
       |  +-----------+  +-----------+  +-----------+  |
       |  |  Thumb 1  |  |  Thumb 2  |  |  Thumb 3  |  |
       |  +-----------+  +-----------+  +-----------+  |
       |  +-----------+  +-----------+  +-----------+  |
       |  |  Thumb 4  |  |  Thumb 5  |  |  Thumb 6  |  |
       |  +-----------+  +-----------+  +-----------+  |
       +---------------------------------------------+
                             │
                             ▼
                    halfblock.Render()
                             │
                             ▼
                    stdout (Single Write)
```

### Thumbnail Caching
To keep panning, window resizing, and page flipping instant, Cati caches scaled thumbnails using a compound key:
```go
type thumbKey struct {
	path string
	w, h int
}
```
If the terminal is resized, new thumbnail dimensions are calculated, and the cache scales the original images to the new target sizes on demand.

### Composite Image Painting
1.  Initialize a blank `image.RGBA` with dimensions `termCols` wide and `gridRowsLimit * 2` high.
2.  Retrieve or build the thumbnail for each page item, scaled to fit `cellW` columns and `(cellH - 1) * 2` pixel rows.
3.  Draw each thumbnail onto the composite canvas at its computed pixel offset `(left, top * 2)`.
4.  Move the cursor to `(1, marginTop + 1)` and run `halfblock.Render` on the composite canvas.
5.  Render the filename labels and page title directly to terminal stdout via standard character positioning.

---

## 3. Double-Buffered Raw Mode

The browser supports opening selected items directly in the full-screen interactive view. Each viewer owns its own terminal mode — the browser restores cooked mode before handing off, and re-enters raw mode after:

```
  +------------------+
  |   Grid Browser   | (Raw Mode Active, Mouse Tracking On)
  +------------------+
           │
           ▼ (Item Clicked / Enter Pressed)
  1. term.Restore → cooked mode
  2. Disable mouse tracking & show cursor
  3. Invoke interactiveWithChan() or interactiveVideo()
     └─ Both call term.MakeRaw internally (raw mode during viewing)
     └─ Both restore terminal state via defer on exit
           │
           ▼ (Viewer exits — q/ESC/^C/video-end)
  4. Drain browser's sigs channel (propagate any SIGINT received during viewing)
  5. term.MakeRaw → raw mode for Grid Browser
  6. Hide cursor & enable mouse tracking
  7. Call redraw() to reconstruct the grid
           │
           ▼
  +------------------+
  |   Grid Browser   | (Restored)
  +------------------+
```

### Critical invariant: every viewer must call term.MakeRaw itself

The browser calls `term.Restore` (cooked mode) before invoking any viewer, so the shared stdin
goroutine is in cooked-mode blocking (waits for a full line before `Read` returns). If a viewer
does **not** call `term.MakeRaw`, single-key presses like `q` and `ESC` appear non-functional
because they are buffered by the line-discipline and never forwarded to the goroutine.

Both `interactiveWithChan` and `interactiveVideo` call `term.MakeRaw` at their top and restore
via `defer term.Restore` — this must be maintained for any future viewer added.

### SIGINT propagation

Go's `signal.Notify` delivers a signal to *all* registered channels. Both `browser()` and each
viewer register for SIGINT. When the user presses `^C` inside a viewer, the viewer's channel
fires and it returns — but the browser's `sigs` channel also buffered the signal. To ensure
`^C` always exits the app, the Enter handler explicitly drains `sigs` after the viewer returns:

```go
select {
case <-sigs:
    shouldQuit = true
    return
default:
}
```

---

## 4. Input & Coordinate Mappings

### SGR Mouse Coordinate Decoding
Clicking buttons or grid cells maps terminal-coordinate hits directly to actions:
*   **Buttons (Row `termRows-1`)**: Checks if clicked column `col` is within `btn.col` and `btn.col + btn.width`.
*   **Cells**: Checks if clicked cursor coordinates `(c, r)` reside within a cell's bounding box:
    ```go
    c >= left && c < left+cellW && r >= top && r < top+cellH
    ```
    If true, the index `itemIdx` is selected and immediately opened.

### Keyboard fallbacks
All mouse-driven actions have full keyboard equivalents to support headless/keyboard-only operations:
*   Arrow keys navigate the selected cell highlight.
*   `Enter`/`Space` opens the selected item.
*   `[` / `]` / `Page Up` / `Page Down` trigger page transitions.
*   `a` toggles the About page overlay.

---

## 5. Async Thumbnail Loading & Priority Queue

Thumbnails (images and video preview frames) are loaded asynchronously so the browser grid
renders immediately with placeholders and fills in progressively.

### Architecture

```
  Browser goroutine                 thumbQueue            Worker goroutines (N = CPU/2)
  ─────────────────                 ──────────            ─────────────────────────────
  redraw()
   └─ getThumbnail(item, w, h)
       ├─ cache hit → return frame
       └─ cache miss → tq.submit()──→ [job, job, job …]──→ thumbWorker
                                                            ├─ LoadImage / LoadVideoFrameAt
                                                            └─ results chan ──→ browser select
                                                                               └─ cache + redraw
```

### Priority Re-ordering

When the user scrolls, newly visible items move to the **front** of the job queue without
interrupting in-progress workers:

```go
tq.prioritize(currentVisibleKeys)  // called inside redraw()
```

The queue is protected by a `sync.Mutex` + `sync.Cond`; workers block on `cond.Wait` and are
woken by `cond.Signal` on each new submission.

### Video Preview Frames

For video items, `loadVideoThumbs` uses `ffprobe` to measure duration, then extracts `N`
evenly-spaced frames via `ffmpeg -ss <offset>`. The frames are stored as a `[]image.Image`
slice in the cache and cycled as a one-shot animation.

### Settings

| Config key | Default | Description |
|---|---|---|
| `preview_videos` | `true` | Whether to extract video preview frames |
| `max_jobs` | `CPU/2` | Parallel thumbnail worker count (0 = auto; overridden by `-j/--jobs`) |
| `video_frames` | `10` | Number of frames extracted per video thumbnail |

### One-Shot Animation

When a video thumbnail scrolls into view (or the cursor moves onto it), `startVisibleAnimations`
triggers a one-shot playback of its cached frames at 300 ms/frame. The animation stops at the
last frame and does not loop, keeping the UI calm when browsing.

---

## 6. Space-Pan Mode in the Image Viewer

The full-screen image viewer (`interactiveWithChan`) supports a **Space-pan** mode as an
alternative to left-button drag:

| Action | Effect |
|--------|--------|
| `Space` (first press) | Enter pan mode — status bar shows hint |
| Move mouse | Image follows cursor (grab-and-pull) |
| `Space` (second press) | Exit pan mode |
| Left-button drag | Always available regardless of pan mode |

### Implementation

Pressing `Space` toggles a `spacePan bool` flag and switches mouse tracking:

```
Space ON  → \x1b[?1003h\x1b[?1006h   (any-motion: reports bare mouse moves)
Space OFF → \x1b[?1002h\x1b[?1006h   (button-event: reports moves only while button held)
```

The first motion event after entering pan mode sets an anchor (`dragState`). Subsequent events
compute pan as an absolute delta from that anchor — identical math to left-button drag. The
translation from terminal-cell deltas to viewport pixels is owned by `internal/viewgeom`, so
halfblock, quad, and spark modes all pan the same source-space region through their own cell
footprint:

```go
state.panX, state.panY = geom.PanFromAnchor(anchor, col, row)
```

Any-motion tracking emits pure move events (`IsMove`) as well as drag events.
Space-pan must accept both; otherwise entering pan mode enables the terminal
protocol but ignores the bare mouse motion it asked for.

The pan values always describe the upper-left origin of the visible viewport in
the scaled image. Renderers receive a cropped image and must respect its
`Bounds().Min`; panning should not be reimplemented inside a renderer as
movement of an output frame or background.

The anchor resets each time pan mode is toggled on, so re-entering always anchors to the
current cursor position.

`ANSIMouseOff` includes `?1003l` so cleanup correctly disables any-motion tracking even if the
viewer exits while pan mode is active.
