# Cati Browser — Spec System & Browser Design

Architecture and design decisions for the spec-driven style/layout system and the interactive grid browser.

---

## 1. Config Key & Temporary Height Adjustment

*   `height` in `~/.config/cati/config` is renamed to `max_preview_height`.
*   `+` / `=` increments `cfgHeight` by 1 row (clamped to `termRows`); `-` decrements (clamped to 10).
*   Changes are memory-only — not persisted — to avoid polluting saved preferences.

---

## 2. Dynamic Grid Density (Dense Mode)

*   **Trigger**: directory contains no images/videos, or the current page has only folders.
*   **Layout**: `gridCols = termCols/20`, `gridRows = gridRowsLimit`, `cellH = 1`, no thumbnails.
*   **Result**: up to 10× more items per page (e.g. 60+ folders vs. 6).

---

## 3. Spec System (`spec/`)

All user-facing configuration, styling, labelling, and layout lives in `spec/`. Every YAML file has a companion JSON Schema in `spec/schemas/` for editor validation and autocomplete.

### 3.1 File map

| File | Schema | Purpose |
|------|--------|---------|
| `spec/style.yaml` | `schemas/style.schema.json` | Colors, borders, grid style, header bar, scrollbar |
| `spec/labels.yaml` | `schemas/labels.schema.json` | Non-button strings: icons, hints, header template |
| `spec/buttons.yaml` | `schemas/buttons.schema.json` | Button text + action bindings (single source of truth) |
| `spec/views.yaml` | `schemas/views.schema.json` | Declarative button-row layouts per view |
| `spec/theme.yaml` | `schemas/theme.schema.json` | Semantic style tokens (primary, secondary, active, …) |
| `spec/controls.yaml` | `schemas/controls.schema.json` | Tunable runtime controls with get/set action names |
| `spec/config.yaml` | `schemas/config.schema.json` | App config defaults — read by `loadSpecConfigDefaults()` as the base layer before user config |
| `spec/about.yaml` | — | About page content (loaded by `spec.LoadYamlView()` via `parseYamlView`) |

All of these files are loaded through typed helpers in `spec/load.go`. The `cmd/` package keeps only thin adapters such as `loadViewButtonRows()` and `loadViewKeyRows()` so the browser logic still works with simple string templates, but the spec content itself is no longer line-parsed in Go.

### 3.2 Color values

All color fields accept:
- `null` — transparent / terminal default
- `#rrggbb` — 24-bit hex
- `#rgb` — 3-digit hex (expanded to `#rrggbb`)
- `dark` / `light` — ANSI 16-color palette entries (`\x1b[90m` / `\x1b[97m`); these **adapt to the user's terminal theme** (Solarized, Gruvbox, Nord, etc.) unlike fixed hex
- Named colors: `black`/`blk`, `white`/`wht`, `red`, `green`/`grn`, `blue`/`blu`, `yellow`/`yel`, `orange`/`org`, `purple`/`pur`, `pink`/`pnk`, `cyan`/`cyn`, `magenta`/`mag`, `brown`/`brn`, `gray`/`grey`/`gry`, `navy`/`nav`, `lime`, `aqua`, `teal`, `maroon`, `olive`, `silver`/`slv`

### 3.3 `spec/style.yaml` sections

```yaml
app:           # App window background and border
buttons:       # Button fg/bg/active colors; left_cap/right_cap applied at load time
preview:       # Image cell background
control_bar:   # Bottom area (button row + hint row) bg/fg
header_bar:    # Top status bar: fg, bg, bold
grid:          # item_fg/bg, selected_fg/bg/bold/marker, image_border
scroll_bar:    # thumb_char, rail_char, width, thumb_fg, rail_fg, rail_bg
```

No hex colors are hardwired in Go. `loadStyle()` only has structural defaults (chars, booleans). Every color comes from `spec/style.yaml`. The `page_title` section (fg/bold) styles the title line in `drawAboutPage` and `drawSettingsPage`.

### 3.4 `spec/labels.yaml` — non-button strings only

Button text **does not** live here. It lives in `spec/buttons.yaml`.

```yaml
app_name:           # used in header template as {app_name}
header:             # header bar template (supports { key | mod } expressions)
folder_icon:        # icon prefix for directory entries
file_icon:          # icon prefix for files in list/preview mode
hint_browser:       # hint bar text for browser/grid view
hint_settings:      # hint bar text for settings view
hint_about:         # hint bar text for about view
hint_viewer:        # hint bar text for image/video viewer
settings_title:     # header shown at top of settings page
settings_hint_tab:  # settings page Tab-to-cycle instruction
settings_hint_adjust: # settings page ↑/↓ instruction
settings_hint_save: # settings page Enter/Esc instruction
website_url:        # URL opened by the open_website action
```

**Template variables available per hint:**

| Variable | Available in | Description |
|----------|-------------|-------------|
| `active_file` | `hint_browser` | Filename of the currently selected item |
| `active_setting` | `hint_settings` | Name of the focused settings field |
| `preview_state` | `hint_browser` | Thumbnail status: `img`, `vid`, `Nf` (N frames), `…` (loading), `""` |
| `queue_size` | `hint_browser` | Pending thumb-load jobs: `↻N` or `""` when idle |
| `last_key` | all hints | Human-readable name of last input event (`"j"`, `"Up"`, `"Scroll Up"`, …) |
| `ssim` | `hint_viewer` | SSIM quality score as `"0.823"` |
| `render_mode` | `hint_viewer` | Current rendering mode name (`"halfblock"`, `"spark/quad"`, `"quad/splithalf"`, `"quad/edge-snap"`, …) |
| `zoom_level` | `hint_viewer` | Nearest ladder source pixels per rendered terminal cell, e.g. `"src px/cell=1.25"`; raw crop ratio is shown by the `Info` action |
| `meta.name` | browser + viewer | Base filename |
| `meta.name_short` | `hint_viewer` | Base filename shortened with `...` to fit the hint bar |
| `meta.ext` | browser + viewer | Lowercase extension without dot |
| `meta.size` | browser + viewer | Human-readable file size (`"3.2 MB"`) |
| `meta.modified` | browser + viewer | File modification date (`"2024-01-15"`) |
| `meta.src_w` | browser + viewer | Source pixel width (`"1920"`) |
| `meta.src_h` | browser + viewer | Source pixel height (`"1080"`) |
| `meta.src_res` | browser + viewer | `"1920×1080"` or `""` if unknown |
| `meta.disp_w` | browser + viewer | Display area width in chars |
| `meta.disp_h` | browser + viewer | Display area height in chars |
| `meta.disp_mode` | browser + viewer | `"half"`, `"quad"`, or `"spark"` |
| `meta.disp_res` | browser + viewer | `"80×24 half"` or `""` |
| `meta.duration` | browser + viewer | `"1:23"`, `"45s"`, or `""` |
| `meta.fps` | browser + viewer | Frame rate (`"29.97"`) or `""` |
| `meta.vcodec` | browser + viewer | Video codec (`"h264"`) or `""` |
| `meta.acodec` | browser + viewer | Audio codec (`"aac"`) or `""` |
| `meta.bitrate` | browser + viewer | `"5.2 Mbps"` or `""` |
| `meta.container` | browser + viewer | Container format (`"mp4"`) or `""` |
| `meta.title` | browser + viewer | Title tag from file metadata |
| `meta.author` | browser + viewer | Artist/author tag |
| `meta.date` | browser + viewer | Capture date from tags |
| `meta.location` | browser + viewer | GPS string from tags |
| `meta.camera` | browser + viewer | Device/camera model from tags |
| `meta.comment` | browser + viewer | Comment tag |

All `meta.*` keys are always present in the vars map (empty string when unknown), so templates never fall back to showing the raw key name. In the browser, `meta.*` values are loaded asynchronously per-path — the hint shows whatever is cached so far, updating on the next redraw after the load completes. In viewers, meta is loaded synchronously at file open.

### 3.5 `spec/buttons.yaml` — button definitions (single source)

Cap characters come from `style.yaml buttons.left_cap`/`right_cap` and are applied at load time by `loadButtons(leftCap, rightCap)`. Button text supports the full template engine syntax including inline `{ 'literal' | mod }` styling.

Each button also declares `prio` for narrow terminals. Lower-priority buttons collapse to compact labels first, then are hidden first if the compact row still does not fit. Key maps are built from the full view row before responsive layout, so hotkeys remain active even when a visual button is hidden.

```yaml
buttons:
  quit:
    text: "{ 'Q' | bold | light }uit"
    style: danger        # theme token (not yet wired to rendering)
    action: quit         # Go action name matched in button click handler
    keys: ["q", "Q", "\x03"]  # keyboard shortcuts that fire this action
  settings:
    text: "{ 'S' | bold | light }ettings"
    style: secondary
    action: open_settings
    keys: ["s", "S"]
```

The `keys:` field lists key sequences (escape sequences as Go string literals) that trigger the same action as clicking the button. `loadKeyActions()` builds a `map[string]string` (key → action name) from this field. In the grid keyboard handler these drive a spec-dispatched `default:` case, replacing the previously hardcoded character switch arms.

**Context-specific keys** (Escape for go_back/quit depending on view, Enter to open files, Space, arrow keys, Tab) are not in `keys:` — they stay hardcoded in Go because they change meaning with view context.

The flow: `loadButtons` → merged into `labels` map at startup → `drawBottomMenu` reads from `labels[key]`.

### 3.6 `spec/theme.yaml` — semantic tokens

Defines reusable named styles referenced by `buttons.yaml`. **Not yet wired** into button rendering — currently only the `style:` field is stored, not applied.

```yaml
primary:   { fg: wht, bold: true }
secondary: { fg: gry }
active:    { fg: wht, bg: gry, bold: true }
danger:    { fg: red }
```

### 3.7 `spec/views.yaml` — layout declarations

Each view is a list of stacked rows. The first non-hint `row:` per view drives `drawBottomMenu`; hint rows are rendered by `drawHintBar`.

```yaml
views:
  browser:
    - area: grid
    - row: "{ prev } { next } { back } | { settings } { mode } { about } | { quit }"
    - row: "{ hint_browser }"

  video_player:
    - area: canvas
    - row: "{ halfblock } { gray } { zoom_in } { zoom_out } { if(playing, pause, play) } { info } { copy_viewport } { render } { back } { quit }"
    - row: "{ hint_viewer }"
```

**Template syntax:**
- `{ key }` — resolves to button widget or label string
- `{ key | mod1 | mod2 }` — with style modifiers: color names, `bold`, `dim`, `italic`, `underline`
- `{ 'literal' | mod }` — quoted literal string with styling (not a label lookup)
- `{ if(cond, trueKey, falseKey) }` — conditionally picks a button key at render time
- Literal text between `{ }` blocks (including `|` separators) is rendered with `control_bar` styling

### 3.8 Template engine (`renderTpl`)

Used for headers, hint bars, and button text. Lives in `cmd/browser.go`.

```
renderTpl(tpl, vars, baseAnsi) string
tplWidth(tpl, vars) int          — visual width without ANSI escapes
tplResolve(key, vars) string     — resolves key: quoted literal, vars map, or fallback
```

**`if()` conditional** — resolved in `drawBottomMenu` before label lookup:
```
{ if(playing, pause, play) }
  → looks up conditions["playing"]
  → if true: renders labels["pause"], if false: renders labels["play"]
```

**Hint bar vars** — passed by the redraw function. See §3.4 for the full table per view.

### 3.9 `spec/controls.yaml` — runtime controls

Loaded by `loadControls()` → `[]ControlSpec`. Drives the settings form:
- Field labels come from `settingsFieldLabel(key)` (snake_case → Title Case)
- Tab cycles through `len(controls)` fields
- `↑`/`↓` call `applySettingsDelta(c, ±1, &tempCfg)` which uses `c.Min`/`c.Max` for int fields and `c.Values` for enum fields

```yaml
controls:
  preview_height:
    type: int
    min: 10
    max: 200
    default: 40
    set: set_preview_height    # not yet wired — action name for future use
    get: get_preview_height
```

Adding a new control to `controls.yaml` with a known `key` (one handled in `applySettingsDelta`) is enough to add it to the settings form.

---

## 4. Bottom Bar Rendering

The bottom two terminal rows are owned by the spec system:

```
row (effHeight-1):  button bar     — drawBottomMenu()
row (effHeight):    hint bar       — drawHintBar()
```

`drawBottomMenu(w, termRows, termCols, viewMode, activeAction, style, labels, viewBtnRows, conditions)`:
- Reads the button row template from `viewBtnRows[viewName]`
- Resolves `if()` conditionals using the `conditions` map
- Renders literal content between `{ }` blocks with `ctrlAnsi` styling
- Returns `[]menuButton` with `{label, action, col, width}` for click detection
- `termCols` is a fallback column budget used when `writerTermCols(w)` returns 0 (non-tty); production callers pass the live terminal width, test callers pass their test budget (e.g. 80)

`drawHintBar(w, termRow, termCols, label, vars, style)`:
- Calls `renderTpl(label, vars, ctrlAnsi)`
- `vars` provides runtime values like `active_file` and `active_setting`

---

## 5. Draggable Scroll Bar & Navigation

```
+--------------------------+
| Item 1                 █ | -- thumb
| Item 2                 ▒ | -- rail
| Item 3                 ▒ |
+--------------------------+
```

- `handleHeight = max(1, visibleRows² / totalRows)`
- `handleTop = (currentRow × (visibleRows − handleHeight)) / (totalRows − visibleRows)`
- Drag: click-press on scrollbar column, drag vertically to shift viewport proportionally.
- Configurable via `scroll_bar` section in `spec/style.yaml`.

---

## 6. Preview Mode Split-Screen

`m`/`M` toggles between Grid and Split-Screen Preview:

```
+-----------------------+-----------------------------+
| folder1               |                             |
| **file1.png**         |     Selected Preview        |
| file2.jpg             |        (Scaled)             |
+-----------------------+-----------------------------+
```

- Left pane: ~40% width, text list
- Right pane: ~60% width, scaled thumbnail of selected item
- State held in memory; saveable to `~/.config/cati/config` as `view_mode=preview|grid`

---

## 7. Interactive Viewers (`cmd/interactive.go`)

`interactiveWithChan` (image) and `interactiveVideo` accept `style`, `labels`, `viewBtnRows` from the browser:

- Image viewer: renders image in `viewRows = termRows - viewerChromeRows`; explicit CLI `--height` sets `viewRows`, not total terminal rows. The current chrome uses a button bar at `termRows-1` and a hint bar at `termRows`. Button actions: `zoom_in`, `zoom_out`, `back`, `quit`.
- Video viewer: same layout. Adds `paused bool`; Space bar toggles. `conditions["playing"] = !paused` passed to `drawBottomMenu` so `{ if(playing, pause, play) }` resolves at render time. Mouse tracking is enabled on entry (the browser disables it before calling the viewer).

Viewer quality values compare a reconstruction of the terminal glyph output
against a common quality-grid source reference. Quad uses
`quadblock.RenderToImage`; spark uses `sparkline.RenderToImage`; halfblock uses
the viewport image directly.

## 8. Audio (`internal/audio`)

New package for audio playback. `play_video` and `pause_video` button actions are stubbed in the browser event loop, ready to call into this package. The `conditions["playing"]` flag in `drawBottomMenu` already reflects the video player's `paused` state and will extend naturally to audio.
