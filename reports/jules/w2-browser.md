# w2-browser

## Verdict
The `cmd/browser*.go` files exhibit significant coupling and God-object characteristics, particularly within the 1000+ line `browser` function. The modularization gaps are primarily in UI logic separation and state management.

## Evidence

1. **Massive Monolithic Browser Function:**
   `cmd/browser.go:117`: `func browser(args []string, initWidth, initHeight int, rc renderCfg, fullComp bool, initialZoom string, jobs int) error {`
   The `browser` function spans over 1000 lines, handling everything from initialization, event loops, rendering, and state management.

2. **Inline UI Rendering in Event Loop:**
   `cmd/browser.go:618`: `redraw := func() {`
   The `redraw` closure inside `browser()` is massive and handles raw ANSI escape sequences directly, rather than delegating to view-specific renderers.

3. **Global-style controlHandlers logic:**
   `cmd/browser_config.go:36`: `var controlHandlers = func() map[string]controlHandler {`
   The `controlHandlers` initialization hardcodes settings retrieval and updates (e.g. `preview_height`, `view_mode`). This couples config keys directly to internal struct manipulations.

4. **Event Loop Event Coalescing logic:**
   `cmd/browser.go:1322`: `// Coalesce / drain consecutive events`
   Input processing and event coalescing logic are manually implemented within the massive `browser` event loop instead of being abstracted into a separate input handler or controller.

5. **Settings application side-effects inline:**
   `cmd/browser.go:1169`: `case "save_settings":`
   When applying settings, the `browser` function directly mutates multiple closure variables (`cfgHeight`, `previewVideos`, `nVideoFrames`) and manually calls `saveConfig()`.

## Smallest next step

1. Extract the `redraw` closure from `browser()` into a standalone `func (s *BrowserState) Draw(w io.Writer)` to decouple rendering from the main event loop.
2. Abstract the keyboard event loop dispatch logic (e.g., `case "save_settings"`) into a separate `Controller` or `InputHandler` struct to simplify `browser()`.
3. Move `controlHandlers` out of `cmd/` and into `internal/browser/config.go` where state management belongs, leaving only CLI parsing in `cmd/`.

## Ticket corrections

Relates to `issues/112-modularize-cmd-package-extract-browser-player-and-modes-into-internal-sub-packages.md`. No new ticket needed.
