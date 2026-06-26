# 008 — Spaghetti Code Analysis

**Status:** 🔴 Open  
**Refs:** AGENTS.md (Spec System rules), [Design.md](../docs/Design.md)

---

## Summary

Static analysis of the 24 non-test Go source files (8,423 lines total) reveals systemic spaghetti code: **12 hand-written YAML parsers**, **~1,080 lines of duplicated code**, and **god functions exceeding 1,200 lines**. The worst offender is `cmd/browser.go` which alone accounts for 2,689 lines (32% of the codebase) and contains a single function spanning ~1,217 lines with 11 levels of nesting.

---

## A. Manual YAML Parsing — 12 ad-hoc parsers, zero YAML libraries

Every YAML file in `spec/` is parsed by hand. None use `gopkg.in/yaml.v3` or any YAML library. Each parser follows the same pattern: `specRead()`, `strings.Split(data, "\n")`, `TrimSpace`, skip `#`/empty lines, `SplitN(line, ":", 2)`, switch/case on key. This is error-prone, duplicates logic, and violates the project's own spec-system rules (AGENTS.md: "Spec is always readable").

**File: `cmd/browser.go` — 11 hand-written parsers**

| # | Function | Lines | Spec File | Description |
|---|----------|-------|-----------|-------------|
| 1 | `loadStyle()` | 137 | `style.yaml` | Reads theme tokens (bg, fg, borders, scrollbar) |
| 2 | `loadLabels()` | 33 | `labels.yaml` | Reads customizable labels |
| 3 | `loadButtonActions()` | 32 | `buttons.yaml` | Reads action: field from buttons |
| 4 | `loadAltButtonActions()` | 31 | `buttons.yaml` | Reads alt_action: field (mirrors #3) |
| 5 | `loadButtonKeyDefs()` | 47 | `buttons.yaml` | Reads keys:/action: pairs (mirrors #3) |
| 6 | `loadButtons()` | 37 | `buttons.yaml` | Reads text: field for rendering (mirrors #3) |
| 7 | `parseYamlView()` | 56 | `about.yaml` | Reads type/name/title/content/controls |
| 8 | `loadSpecConfigDefaults()` | 52 | `config.yaml` | Reads default settings from spec |
| 9 | `loadConfig()` | 49 | user `config` | Reads user config (same format as #8) |
| 10 | `loadControls()` | 68 | `controls.yaml` | Reads control min/max/values |
| 11 | `loadViewRowYaml()` | 91 | `views.yaml` | Reads view row templates + hidden_keys |

**Sub-issue:** `buttons.yaml` is parsed 4 times (#3, #4, #5, #6). Each parser independently implements the same `inButtons`/`currentKey` state machine. A single pass returning a structured struct would replace all four.

**File: `internal/input/input.go` — 1 hand-written parser**

| # | Function | Lines | Spec File | Description |
|---|----------|-------|-----------|-------------|
| 12 | `parse()` | 167 | `input.yaml` | Reads key aliases, mouse config, tokenizer rules, terminal sequences |

The `parse()` function is the most sophisticated (indent tracking, list items, section nesting), yet is unrelated to the other 11 parsers — no shared abstraction exists.

---

## B. Duplicate / Mirrored Code — ~1,080 lines

### B1. 5 ANSI helpers duplicated between halfblock and quadblock

The files `internal/halfblock/render.go` and `internal/quadblock/render.go` both define:

| Function | halfblock | quadblock |
|----------|-----------|-----------|
| `fgRGB()` | render.go:31-33 | render.go:33-35 |
| `bgRGB()` | render.go:36-38 | render.go:37-39 |
| `toRGBA()` | render.go:43-54 | render.go:43-49 |
| `isTransparent()` | render.go:57 | render.go:51 |
| `eqRGB()` | render.go:60-62 | render.go:53 |
| `ansiReset` | render.go:27 | render.go:27 |
| `ansiLinePrefix` | render.go:29 | render.go:30 |

**Fix:** Extract to a shared `internal/colorutil` package.

### B2. Terminal boilerplate triplicated

The stdin-reader goroutine + `MakeRaw` + `HideCursor` + signal.Notify pattern appears in:

| File | Lines | Lines of Code |
|------|-------|---------------|
| `cmd/browser.go:1278-1327` | 50 | ~25 |
| `cmd/interactive.go:172-222` (interactiveWithChan) | 51 | ~25 |
| `cmd/interactive.go:624-729` (interactiveVideo) | 106 | ~50 |

`interactiveVideo` adds channel splitting but follows the same skeleton.

### B3. Identical action handlers repeated in mouse + keyboard dispatch

In `cmd/interactive.go`:

| Action | Mouse handler (lines) | Keyboard handler (lines) | Lines duplicated |
|--------|-----------------------|-------------------------|-----------------|
| `toggle_play_pause` | 802-809 | 848-855 | 8 |
| `copy_viewport` | 811-818 | 857-864 | 8 |
| `cycle_render` | 819-825 | 865-871 | 7 |
| `cycle_render_prev` | 826-832 | 872-878 | 7 |
| `go_back` / `quit` | 833-834 | 846-847 | 2 |

**Total duplicated across these:** ~32 lines, each pair identical except wrapping context.

### B4. Inline image blitting duplicated

`cmd/browser.go` lines 1518-1526 and 1550-1558 contain identical image blitting loops:
```go
for ty := 0; ty < scaledH; ty++ {
    for tx := 0; tx < scaledW; tx++ {
        dx := destX + tx
        dy := destY + ty
        if dx >= 0 && dx < compW && dy >= 0 && dy < compH {
            compImg.Set(dx, dy, previewImg.At(tx, ty))
        }
    }
}
```

### B5. Thumbnail list rendering loops (3 variants)

`cmd/browser.go` renders file lists in three ways (preview mode, dense list, grid) — lines 1587-1613, 1614-1636, 1637-1663. Each is a variation on the same pattern with slightly different layout math.

### B6. quadruple render-mode command handling

`cycleRenderCfg` and `cycleRenderCfgPrev` (`cmd/interactive.go:89-108`) differ only by `(i+1)%n` vs `(i+n-1)%n`. The render-mode dispatch in `cmd/browser.go` mouse handler (lines 2063-2068) duplicates the `inc_zoom`/`dec_zoom` zoom pattern from `cmd/interactive.go`.

### B7. ffprobe command skeleton repeated 4 times

`internal/halfblock/video.go` builds the same `exec.Command("ffprobe", "-v", "quiet", ...)` pattern for:
- `ProbeVideoDimensions` (lines 36-45) — 10 lines
- `ProbeVideoFPS` (lines 66-75) — 10 lines (mirror of dimensions)
- `ProbeVideoDuration` (lines 96-103) — 8 lines (mirror of above)
- `ProbeMediaMeta` (lines 278-284) — 7 lines (JSON variant)

### B8. compileCell algorithm boilerplate

`internal/quadblock/algos.go` has duplicate opaque-pixel extraction and switch-on-count logic at lines 13-26 and 53-66, and duplicate diameter-finding loops at lines 29-38 and 69-78. `compileCellDiameter` and `compileCellKMeans` share an initialisation sequence that could be factored out.

---

## C. God Functions — Excessive Size and Nesting

### C1. `cmd/browser.go:browser()` — ~1,217 lines

The single `browser` function spans line 1170 through line 2381 (~1,212 lines). It contains two massive closures:

- **`redraw()`** (line 1367): ~412 lines. Renders the entire page: computes layout, loads thumbnails, draws grid/preview/list, scrollbar, header, bottom menu, hint bar. Contains 5+ nested conditionals.
- **`processInput()`** (line 1905): ~443 lines. Handles all keyboard + mouse input: scroll, click, drag, button hover, settings navigation, file opening, view mode switching. Replicates the nesting structure of `redraw()`.

**Nesting depth:** 11 levels. Example path:
```
browser() → select { case <-inputs: } → processInput() →
ParseMouse().ok → mouse handler → IsScroll() case →
scrollbar draq → inner if → totalRows/gridRows check →
selectedIdx assignment
```

### C2. `cmd/interactive.go:interactiveVideo()` — ~368 lines

Spans lines 621-989. Contains:
- `processToken()` closure (lines 788-883): ~95 lines with nested switch-inside-switch
- Event loop at lines 889-978: ticker, key input, mouse drain, redraw with buttons + hint bar

**Nesting depth:** 9 levels.

### C3. `cmd/interactive.go:interactiveWithChan()` — ~320 lines

Spans lines 145-465.
- `redraw()` closure (lines 237-262): ~25 lines
- `processInput()` closure (lines 279-437): ~158 lines — 6-level nesting

### C4. `cmd/browser.go:drawBottomMenu()` — ~100 lines

Lines 2586-2682. Parses view row templates, resolves `if(cond, a, b)` expressions, renders buttons with ANSI styling, returns hit-test rectangles. Does string parsing that should be pre-computed.

### C5. `internal/input/input.go:parse()` — ~167 lines

Lines 201-367. Hand-written YAML parser with indent-level tracking, section/subsection state machine, list-item commit pattern.

---

## D. Metrics Summary

### File-level metrics

| File | Lines | funcs | `if` | `switch` | Max Depth | Lines ≥5 levels | % File |
|------|-------|-------|------|----------|-----------|-----------------|--------|
| `cmd/browser.go` | 2,689 | 58 | 278 | 18 | 11 | 531 | 20% |
| `cmd/interactive.go` | 978 | 23 | 168 | 15 | 9 | 262 | 26% |
| `internal/quadblock/render.go` | 911 | 18 | 47 | 1 | 6 | 30 | 3% |
| `internal/input/input.go` | 678 | 16 | 53 | 4 | 7 | 56 | 8% |
| `internal/halfblock/video.go` | 348 | 6 | 34 | 2 | 5 | 12 | 3% |
| `internal/quadblock/algos.go` | 223 | 5 | 38 | 2 | 4 | 6 | 3% |
| `internal/pixelart/pixelart.go` | 220 | 7 | 24 | 0 | 4 | 8 | 4% |
| `cmd/ssim.go` | 241 | 12 | 25 | 0 | 4 | 4 | 2% |
| `cmd/thumbqueue.go` | 162 | 6 | 21 | 0 | 4 | 2 | 1% |
| `cmd/root.go` | 267 | 8 | 19 | 2 | 3 | 0 | 0% |
| `cmd/play.go` | 245 | 4 | 13 | 2 | 4 | 8 | 3% |
| `cmd/input_tester.go` | 216 | 3 | 17 | 4 | 5 | 22 | 10% |
| Others (12 files) | <200 ea | — | — | — | ≤3 | 0 | 0% |
| **Total** | **8,423** | **262** | **801** | **58** | **11** | **941** | **11%** |

### Duplication metrics

| Category | Instances | Est. Lines | Primary Locations |
|----------|-----------|------------|-------------------|
| Hand-written YAML parsers | 12 | ~800 | browser.go (11), input.go (1) |
| Color/ANSI helpers (halfblock vs quadblock) | 7 | ~26 | render.go ×2 |
| Input-handling boilerplate | 6 | ~150 | browser.go, interactive.go |
| Copy-pasted command handlers | 5 | ~50 | interactive.go |
| Other (cycle, ffprobe, diameter, blitting) | 5 | ~55 | Various |
| **Total** | **35** | **~1,080** | —

---

## E. Spec System Violations

The spec system rules (AGENTS.md) mandate:
- "No Go fallbacks — do not maintain hardcoded copies of spec content in Go"
- "Update spec and Go together"
- "All keys must be specced"

### E1. Hardcoded fallback views in `loadViewRowYaml()` (line 2485-2513)

When `views.yaml` is unreadable or missing, the function falls back to hardcoded strings:
```go
visibleDefaults := map[string]string{
    "browser":      "{ prev } { next } { back } | { settings } { mode } { about } | { quit }",
    "settings":     "{ save } { cancel } | { quit }",
    "about":        "{ back } { website } | { quit }",
    "image_viewer": "{ zoom_in } { zoom_out } { toggle_pan } { copy_viewport } { back } { quit }",
    "video_player": "{ zoom_in } { zoom_out } { if(playing, pause, play) } { copy_viewport } { back } { quit }",
}
```

Same for `hiddenDefaults`:
```go
hiddenDefaults := map[string]string{
    "browser": "{ nav_up } { nav_down } { page_prev } { page_next }",
}
```

This directly violates "No Go fallbacks". If the spec file goes missing, the app shows hardcoded defaults instead of the raw key names.

### E2. Hardcoded button labels in `loadLabels()` (lines 512-521)

```go
labels := map[string]string{
    "app_name":       "Cati Browser",
    "header":         "{app_name} [{dir}] — Page {page}/{pages} ({start}-{end} of {total})",
    "folder_icon":    "📁",
    "file_icon":      "📄",
    "hint_browser":   "[Enter/Click] View/Enter ...",
    "hint_settings":  "[▲/▼] Adjust ...",
    "hint_about":     "[q/Esc] Back",
    "hint_viewer":    "[q/Esc] Back [+/-] Zoom",
}
```

Fallback labels per spec rule should show raw key names, not hardcoded strings.

### E3. Hardcoded settings defaults in `loadControls()` (lines 1047-1054)

```go
specs := []ControlSpec{
    {Key: "preview_height", Type: "int", Min: 10, Max: 200},
    {Key: "view_mode",      Type: "enum", Values: []string{"grid", "preview"}},
    ...
}
```

Controls are hardcoded in Go and only optionally overridden by `controls.yaml`.

### E4. Hardcoded config defaults in `loadSpecConfigDefaults()` (line 924)

```go
cfg := Settings{MaxPreviewHeight: 20, ViewMode: "grid", PreviewVideos: true, ...}
```

---

## F. Architectual Issues

### F1. `cmd/` package is a god package

All 12 `.go` files in `cmd/` share the same package `cmd`. This means:
- `browser.go` can call any unexported function from `interactive.go`, `ssim.go`, `thumbqueue.go`, etc.
- The compiler never catches misplaced dependencies because everything is internal
- No clear module boundaries: settings, clipboard, rendering, and video playback are all in one bucket

### F2. `cmd/browser.go` imports everything, knows everything

Imports include: `internal/halfblock`, `internal/input`, `spec`, `x/term`. The file directly:
- Sets up raw terminal mode (line 1284)
- Runs ffmpeg subprocesses (via thumbqueue)
- Parses YAML spec files (11 parsers)
- Renders ANSI output directly to stdout
- Handles mouse/keyboard input dispatch
- Manages thumbnail cache, metadata cache, animation state

This violates the Single Responsibility Principle at every level.

### F3. State is passed through long parameter chains

`interactiveVideo` receives 10 parameters (line 621):
```go
func interactiveVideo(path string, initWidth, initHeight int, rc renderCfg,
    sharedInputs chan string, style *StyleConfig, labels map[string]string,
    viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string,
    inputSpec *input.Spec) error
```

And `interactiveWithChan` also receives 10 parameters (line 145). Both are called from browser.go with `nil` for all but the first 5, then the function re-loads them from scratch if nil. This creates confusing conditional initialisation paths.

### F4. `renderCfg` struct has unexported fields with overlapping semantics

```go
type renderCfg struct {
    id       int
    useQuad  bool
    quadOpts quadblock.Options
    preScale func(image.Image) image.Image
}
```

- `useQuad` is redundant with `quadOpts != zero value`
- `id` must match the slice index in `renderModes` (enforced by a comment, never by the compiler)
- The zero value of `id` means halfblock, but zero could also mean "not set"

---

## G. Remediation Priority

| Priority | Area | Effort | Impact | Approach |
|----------|------|--------|--------|----------|
| **P0** | 12 hand-written YAML parsers → single `yaml.Unmarshal` | 2-3 days | Eliminates 35+ functions, corrects all spec violations | Add `gopkg.in/yaml.v3`, define Go structs matching spec schemas, delete all manual parsers |
| **P1** | Break up `browser()` god function | 3-5 days | Reduces max function size from 1,217 to <200 lines | Extract `redrawGrid`, `redrawPreview`, `processMouse`, `processKeyboard`, `handleDirNav` |
| **P1** | Factor out duplicated terminal boilerplate | 0.5 day | Removes ~80 lines of duplication | Create `enterVisualMode()` / `exitVisualMode()` helpers in `halfblock` |
| **P2** | Merge `cycle_render`/`copy_viewport`/`toggle_play_pause` duplicate handlers | 0.5 day | Removes ~50 lines, ensures consistency | Extract handler functions in `interactive.go` |
| **P2** | Extract shared color/ANSI helpers | 0.5 day | Removes ~26 lines of duplication, single source of truth | New `internal/colorutil` package |
| **P3** | Split `cmd/` package into sub-packages | 2 days | Enforces module boundaries, prevents cross-contamination | `cmdbrowser/`, `cmdviewer/`, `cmdconfig/` |
| **P3** | Consolidate ffprobe invocation patterns | 0.5 day | Removes ~4 duplicate command builders | `halfblock.probeJSON()` or similar |
| **P4** | `renderCfg` cleanup | 0.5 day | Eliminates redundant `useQuad` field | Compare `quadOpts` to zero value, validate `id` in init |

---

## H. Conclusion

The cati codebase suffers from three systemic problems:

1. **No YAML library** — 12 hand-written parsers create ~800 lines of fragile, duplicate spec-reading code that directly violates project conventions. This is the single highest-impact fix.

2. **God functions in `cmd/browser.go`** — the `browser()` function exceeds 1,200 lines and handles every concern: input, rendering, navigation, caching, settings, animation. It needs to be decomposed into focused sub-functions.

3. **Triplicated infrastructure** — the stdin-reader goroutine, raw-terminal entry, signal handling, and event-draining loop are copied-and-pasted across three functions instead of being extracted once.

The duplication analysis found ~1,080 lines (13% of the codebase) that could be eliminated through extraction and consolidation. The nesting analysis found 941 lines (11% of the codebase) at 5+ levels of indentation, concentrated in `browser.go` and `interactive.go`.

A systematic refactoring following the P0→P4 priority table above would cut the codebase by 10-15% while dramatically improving maintainability, testability, and spec compliance.
