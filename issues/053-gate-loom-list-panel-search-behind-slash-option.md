# 053 — Gate Loom List Panel Search Behind Option ("Type to Search" vs "[/] Search")

**Status**: Closed (Implemented GatedSearch in loom Choice and integrated into cati imgbrowser)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `codeberg.org/ubunatic/loom` (`choice.go`), `examples/imgbrowser/browser.go`, `examples/filebrowser/filebrowser/browser.go`

---

## 1. Problem & Motivation

In split-pane and dashboard TUI applications (such as `imgbrowser` and `filebrowser`), the left file list uses `loom.Choice` alongside several application-level single-key hotkeys:
- `p` (play/pause video)
- `i` (toggle images-only)
- `m` / `Shift-M` (cycle render mode)
- `f` (toggle fullscreen)
- `#` (toggle info/stats view)
- `+` / `-` / `0` (zoom controls)

Currently, `loom.Choice` eagerly consumes all printable text characters into its live filter query (`default: if e.Text != "" { c.query += e.Text; c.refilter() }`). This creates fundamental keybinding conflicts:
1. **Shortcut swallowing**: When an application attempts to intercept single-key shortcuts, typing filename characters that match hotkeys (e.g. `p`, `m`, `f`, `i`) becomes impossible or buggy.
2. **Greedy text input**: When the file list is focused, the user cannot intuitively use navigational hotkeys without accidentally appending characters to the active search filter.
3. **Shadow filter state tracking**: Applications currently maintain awkward shadow variables (`filterLen`) to guess whether `Choice`'s private `query` is empty before deciding to handle hotkeys.

## 2. Technical Specification

### 2.1 Loom Choice Gated Search Configuration

`loom.Choice` should provide an explicit option to configure search trigger behavior:

```go
type Choice struct {
    ...
    // GatedSearch requires an explicit trigger key (default '/') before accepting search input.
    // When false (default / "type to search"), any printable character immediately filters the list.
    // When true ("[/] search"), printable characters do not filter unless search mode is active.
    GatedSearch bool

    // SearchTrigger defines the key that activates search mode when GatedSearch is true (default "/").
    SearchTrigger string
}
```

### 2.2 Interaction Modes

1. **Normal Navigation Mode (`GatedSearch: true`, inactive)**:
   - Arrow keys (`Up`/`Down`/`PgUp`/`PgDn`/`Home`/`End`) navigate items.
   - Printable characters are **not** consumed by `Choice`, allowing parent widgets and frames to handle hotkeys (`p`, `i`, `m`, `f`, `#`, etc.).
   - The prompt row renders a hint (e.g. `[/] search` or placeholder text).
   - Pressing `/` activates Search Input Mode.

2. **Search Input Mode (`GatedSearch: true`, active)**:
   - Visual indicator: active cursor and prompt (e.g. `filter> `).
   - Printable characters append to `c.query` and trigger `c.refilter()`.
   - `Backspace` removes characters. Backspace on empty query exits Search Input Mode.
   - `Esc` or `Enter` exits Search Input Mode (retaining or clearing the filter according to config).

### 2.3 Application Integration (`imgbrowser`)

In `examples/imgbrowser/browser.go`:
- Enable `list.GatedSearch = true` (and `list.Placeholder = "[/] search"`).
- Remove fragile shadow `filterLen` tracking.
- Hotkeys (`p`, `i`, `m`, `f`, `#`, zoom) work cleanly during normal browsing without colliding with filename search.

---

## 3. Implementation & Verification Plan

1. **Loom Upstream (`choice.go`)**:
   - Add `GatedSearch bool` and search active state to `loom.Choice`.
   - Gate printable character filtering behind active search mode when `GatedSearch == true`.
   - Support `/` activation and `Esc`/`Backspace` deactivation.
   - Add unit tests in `choice_test.go` verifying hotkey pass-through vs. active filter mode.

2. **Cati Browser / Examples (`examples/imgbrowser`)**:
   - Configure `loom.Choice` with `GatedSearch: true`.
   - Update status bar and placeholder text to display `[/] search`.
   - Verify all media hotkeys (`p`, `i`, `m`, `f`, `#`) function without interfering with search.
