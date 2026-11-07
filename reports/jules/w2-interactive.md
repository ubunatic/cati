# w2-interactive

## Target: `cmd/interactive*.go` and `cmd/viewer_core*.go`

### 1. Extract unified setup into `viewerCore`
- **Context:** Both `interactiveWithChan` (images) and `interactiveVideo` (video) duplicate boilerplate to init `inputSpec`, `style`, `labels`, `viewBtnRows`, etc.
- **Citation:** `cmd/interactive.go:361` (in `interactiveVideo`): `func interactiveVideo(path string, initWidth, initHeight int, rc renderCfg, tr TimeRange, sharedInputs chan string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string, inputSpec *input.Spec, fullComp bool, initialZoom string) error {`
- **Proposal:** Add a setup helper to centralize default styles, specs, and labels.
- **Smallest Next Step:** Create `setupViewerEnv()` to DRY up the setup phases.

### 2. Move video state out of `interactiveVideo`
- **Context:** `interactiveVideo` is massive and mixes UI events, terminal IO, audio, and video frames.
- **Citation:** `cmd/interactive.go:501` (in `interactiveVideo`): `setPaused := func(p bool) {`
- **Proposal:** Encapsulate video state into a struct (e.g., `videoController`).
- **Smallest Next Step:** Move `setPaused` and audio functions into a dedicated struct.

### 3. Consolidate repetitive cfg copying
- **Context:** `cycleRenderCfg` and `cycleRenderCfgPrev` manually copy state fields.
- **Citation:** `cmd/interactive_modes.go:157` (in `cycleRenderCfg`): `next.cfg.prescaler = rc.prescaler`
- **Proposal:** Use a helper method for carrying over observer state when cycling modes.
- **Smallest Next Step:** Extract a `preserveState(base, current renderCfg) renderCfg` function.

### 4. Move view sizing logic into `viewgeom.Spec`
- **Context:** `renderedCellSizeForPixels` checks `rc.mode` manually to compute size, which belongs in `viewgeom`.
- **Citation:** `cmd/interactive_render.go:278` (in `renderedCellSizeForPixels`): `func renderedCellSizeForPixels(w, h int, rc renderCfg) renderCells {`
- **Proposal:** Delegate calculation entirely to `rc.viewSpec()`.
- **Smallest Next Step:** Add `CellsForPixels(w,h)` to `viewgeom.Spec`.

### 5. Modularize token routing
- **Context:** `viewerCore.handleKey` relies on parsing terminal sequences deep in the rendering core.
- **Citation:** `cmd/viewer_core.go:193` (in `handleKey`): `func (vc *viewerCore) handleKey(tok string) (quit, changed bool, unhandledAction string) {`
- **Proposal:** `handleKey` should consume structured `input.Event` instead of strings to decouple raw ANSI parsing.
- **Smallest Next Step:** Pass `input.Event` instead of `tok string`.
