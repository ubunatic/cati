# mediabrowse

A split-pane file and media browser example combining **Cobra CLI**, **Loom** (`Frame`, `Box`, `Choice`, and themes), and **Loom's `media.Widget`** backed by **Cati** rendering algorithms.

## Features

- **Cobra CLI flags**:
  - `mediabrowse [directory]`
  - `--theme, -t`: Select color theme (`mc`, `solarized-dark`, `monokai`, `dracula`, `nord`, `gruvbox`, etc.)
  - `--mode, -m`: Initial render mode (`halfblock`, `quadblock`, `sextant`)
  - `--fps`: Configurable video playback frame rate
  - `--images-only, -i`: Filter list to media files only
- **Interactive Split-Pane**:
  - **Files Pane**: Directory browsing, `/` gated search, and instant selection tracking.
  - **Media Preview Pane**: Cati-rendered still images & live streaming video playback using `loom/media.Widget`.
- **Keyboard & Mouse Controls**:
  - `Tab`: Switch focus between file list and media preview
  - `m`: Cycle render mode (`halfblock` → `quadblock` → `sextant`)
  - `p` / `P`: Toggle video playback / pause
  - `f`: Toggle fullscreen preview
  - `i`: Toggle media-only filter in file list
  - `F9`: Live cycle color themes
  - `F10` or `q`: Quit

## Running the Example

```sh
go run ./examples/mediabrowse [directory]
```
