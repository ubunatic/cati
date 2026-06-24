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

The browser supports opening selected items directly in the full-screen interactive view. Because the interactive viewer manages its own terminal setup and restores original cooked mode on exit, Cati implements raw mode swapping:

```
  +------------------+
  |   Grid Browser   | (Raw Mode Active, Mouse Tracking On)
  +------------------+
           │
           ▼ (Item Clicked / Enter Pressed)
  1. Restore original terminal state (cooked mode)
  2. Disable mouse tracking & show cursor
  3. Invoke interactive(selectedPath, ...)
           │
           ▼ (Interactive View Runs & Cleans Up on Quit)
  4. Restore raw mode for Grid Browser
  5. Hide cursor & enable mouse tracking
  6. Call redraw() to reconstruct the grid
           │
           ▼
  +------------------+
  |   Grid Browser   | (Restored)
  +------------------+
```

This guarantees that sub-views do not conflict with parent raw inputs or leave the terminal in a broken state if the application crashes.

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
