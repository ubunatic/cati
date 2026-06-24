# Cati Browser — Advanced Interactive Enhancements Design Doc

This document describes the design, style specifications, and architecture of the Cati Browser enhancements.

---

## 1. Config Key Update & Temporary Adjustments

*   **Config Key Rename**: The `height` configuration option in `~/.config/cati/config` is renamed to `max_preview_height`.
*   **Temporary Height Scaling**:
    *   Pressing `+` or `=` inside the browser temporarily increments the current preview height limit by `1` row (clamped to terminal rows).
    *   Pressing `-` temporarily decrements the current preview height limit by `1` row (clamped to a minimum of `10` rows).
    *   These changes are only held in memory and not persisted to `~/.config/cati/config` to allow flexible, transient layout adjustments without polluting the user's saved preferences.

---

## 2. Dynamic Grid Density (Dense Mode)

To optimize screen space and minimize paging, the grid layout adapts dynamically to its content.

*   **Trigger**: If a directory contains **no images or videos** (only subdirectories and files), or if the current page consists entirely of folders/directories (no media files are rendered).
*   **Dense Layout Configuration**:
    *   Grid columns (`gridCols`) are increased (e.g. dynamically calculated as `termCols / 20`, or fixed at `4` or `5`).
    *   Grid rows (`gridRows`) are set to the full available vertical lines (`gridRowsLimit`).
    *   Cell Height (`cellH`) is set to `1` line.
    *   Thumbnails are omitted. Folder items are drawn purely as text labels with a subtle prefix (e.g. `📁 name` or `[📁] name`).
*   **Benefit**: Increases single-page item density by up to 10× (e.g. displaying 60+ folders instead of 6 folders per page).

---

## 3. Style Specification (`spec/style.yaml`)

A generic style system is introduced to customize Cati's colors, borders, and buttons.

```yaml
# Cati Application Stylesheet
app:
  # Terminal background: "transparent" (null) or raw 24-bit color hex (e.g., "#0d0f14")
  bg: null
  # App border: "none", "box" (standard unicode characters), or "double" (double borders)
  border_style: "box"
  border_color: "#475569"

buttons:
  fg: "#94a3b8"
  bg: "#1e293b"
  border_color: "#334155"
  left_cap: "["
  right_cap: "]"
  # Subtle active/hover highlight colors
  active_fg: "#f8fafc"
  active_bg: "#475569"

preview:
  # Background color inside image cells
  bg: null

control_bar:
  # Background for the whole bottom area (button bar row + hint bar row)
  bg: "#0f172a"
  # Text colour for the shortcut hint bar (row below the button bar)
  fg: "#94a3b8"

scroll_bar:
  # Scrollbar character properties
  thumb_char: "█"
  rail_char: "▒"
  # Scrollbar width: 1 or 2 characters
  width: 1
  # Color options
  thumb_fg: "#64748b"
  rail_fg: "#334155"
  rail_bg: null
```

---

## 4. Draggable Scroll Bar & Navigation

A vertical scrollbar is drawn along the right edge of the grid/list area.

```
+--------------------------+
| Item 1                 █ | -- Scrollbar handle (thumb)
| Item 2                 ▒ | -- Rail (track)
| Item 3                 ▒ |
+--------------------------+
```

### Mechanics
*   **Scrollbar Dimensions**:
    *   Height corresponds to `gridRowsLimit` terminal lines.
    *   Width is configurable to `1` or `2` characters.
*   **Calculations**:
    *   Total rows: `totalRows = (len(items) + gridCols - 1) / gridCols`
    *   Visible rows: `visibleRows = gridRows`
    *   Handle size: `handleHeight = max(1, visibleRows * visibleRows / totalRows)`
    *   Handle position: `handleTop = (currentRow * (visibleRows - handleHeight)) / (totalRows - visibleRows)`
*   **Interaction**:
    *   **Keyboard**: `Up`/`Down` arrows move the selection row-by-row (no rollover at list boundaries).
    *   **Mouse Wheel**: Scrolls the visible list one row up/down.
    *   **Drag & Drop**: Click-press on the scrollbar column and dragging vertically shifts the viewport proportionally.

---

## 5. Preview Mode Split-Screen

Pressing `m` or `M` toggles between the Grid view and Split-Screen Preview Mode.

```
+-----------------------+-----------------------------+
| folder1               |                             |
| folder2               |        Selected Preview     |
| **file1.png**         |           (Scaled)          |
| file2.jpg             |                             |
| file3.png             |                             |
+-----------------------+-----------------------------+
```

### Layout
*   **Left Pane**: Text-based vertical list of folders and files (takes ~40% of terminal width).
*   **Right Pane**: Displays the currently selected item's thumbnail preview, scaled to fit the right pane dimensions (takes ~60% of width).
*   **Persistent Config**: Toggle state is held temporarily in memory, but can be saved to `~/.config/cati/config` via the Settings dialog as `view_mode=preview` or `view_mode=grid`.
