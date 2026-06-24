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

## 3. Spec System (`spec/`)

All user-facing configuration, styling, labelling, and layout lives in `spec/`. Every YAML file in `spec/` has a companion JSON Schema in `spec/schemas/` for editor validation and autocomplete (VS Code with the YAML extension, JetBrains IDEs, etc.).

### 3.1 File map

| File | Schema | Purpose |
|------|--------|---------|
| `spec/style.yaml` | `schemas/style.schema.json` | Colors, borders, grid selection style, header bar, scrollbar |
| `spec/labels.yaml` | `schemas/labels.schema.json` | Button labels, icons (folder/file), hint bar text |
| `spec/theme.yaml` | `schemas/theme.schema.json` | Semantic style tokens (primary, secondary, active, muted, info, danger) |
| `spec/buttons.yaml` | `schemas/buttons.schema.json` | Button definitions with action bindings |
| `spec/views.yaml` | `schemas/views.schema.json` | Declarative screen layouts |
| `spec/controls.yaml` | `schemas/controls.schema.json` | Tunable runtime controls with get/set bindings |
| `spec/config.yaml` | `schemas/config.schema.json` | App config defaults |
| `spec/about.yaml` | — | About page content |

### 3.2 Color values

All color fields accept:
- `null` — transparent / terminal default
- `#rrggbb` — 24-bit hex
- `#rgb` — 3-digit hex (expanded to `#rrggbb`)
- Named colors: `black`/`blk`, `white`/`wht`, `red`, `green`/`grn`, `blue`/`blu`, `yellow`/`yel`, `orange`/`org`, `purple`/`pur`, `pink`/`pnk`, `cyan`/`cyn`, `magenta`/`mag`, `brown`/`brn`, `gray`/`grey`/`gry`, `navy`/`nav`, `lime`, `aqua`, `teal`, `maroon`, `olive`, `silver`/`slv`

### 3.3 `spec/style.yaml` sections

```yaml
app:           # App window background and border
buttons:       # Button fg/bg/active colors and cap characters [ ]
preview:       # Image cell background
control_bar:   # Bottom area (button row + hint row) bg/fg
header_bar:    # Top status bar: fg, bg, bold
grid:          # Grid/list item display: item_fg/bg, selected_fg/bg/bold/marker, image_border
scroll_bar:    # Scrollbar chars, width, colors
```

### 3.4 `spec/labels.yaml` keys

Non-button strings only. Button text lives in `spec/buttons.yaml`.

```yaml
app_name:        # Application name (used in header template)
header:          # Header bar template with { key | mod } expressions
folder_icon:     # Icon prefix for directories (default 📁)
file_icon:       # Icon prefix for files in list/preview mode (default 📄)
hint_browser:    # Hint bar text for the main browser view
hint_settings:   # Hint bar text for the settings view
hint_about:      # Hint bar text for the about view
hint_viewer:     # Hint bar text for the image/video viewer
```

### 3.5 `spec/theme.yaml` — semantic tokens

Defines reusable named styles referenced by `buttons.yaml` and view template expressions:

```yaml
primary:   { fg: wht, bold: true }   # main CTA
secondary: { fg: gry }               # standard action
active:    { fg: wht, bg: gry, bold: true }
muted:     { fg: "#475569" }
info:      { fg: cyn }
danger:    { fg: red }
```

### 3.6 `spec/buttons.yaml` — button definitions

Single source for button text. Cap characters come from `style.yaml buttons.left_cap`/`right_cap` and are applied at load time.

Button text supports `{ 'literal' | mod }` inline styling — same template engine as the header bar.

```yaml
buttons:
  quit:
    text: "✖ Quit"
    style: danger        # theme token
    action: quit         # registered Go handler name
  settings:
    text: "⚙ { 'S' | bold }ettings"  # inline bold on the keyboard shortcut letter
    style: secondary
    action: open_settings
```

### 3.7 `spec/views.yaml` — layout declarations

Each view is a list of rows/areas stacked top-to-bottom. Template strings use `{ key }` / `{ key | modifier... }` expressions.

```yaml
views:
  browser:
    - area: grid          # fills remaining vertical space
    - row: "{ prev } { next }  { settings } { about } { quit }"
    - row: "{ hint_browser }"

  settings:
    - area: settings_form
    - row: "{ inc } { dec }  { save } { cancel }"
    - row: "{ hint_settings }"
```

**Template syntax:**
- `{ key }` — resolves to a button widget or label string
- `{ key | mod1 mod2 }` — same with style overrides: color names (`red`, `cyn`…), `bold`, `dim`, `italic`, `underline`
- Literal spaces between `{ }` blocks are preserved as padding

### 3.8 `spec/controls.yaml` — runtime controls

```yaml
controls:
  preview_height:
    type: int
    min: 10
    max: 200
    default: 40
    set: set_preview_height   # Go action name
    get: get_preview_height
```

### 3.9 Placeholder assets (`assets/`)

`assets/` will hold static PNG files referenced by spec (e.g. folder placeholder, video placeholder). Currently the folder icon is generated in-memory by `createFolderIcon()` in `cmd/browser.go`.

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
