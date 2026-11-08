## Verdict
Several documentation files in `docs/` have drifted from the codebase: `Design.md` lists outdated signatures and locations, and several pixel art docs refer to `internal/` paths instead of `v1/`.

## Evidence
- `docs/Design.md:208`
  `Used for headers, hint bars, and button text. Lives in cmd/browser.go.`
  Actually partially moved to `internal/browser` via `browserui`. Example: `cmd/browser.go:1345` `func tplWidth(tpl string, vars map[string]string) int { return browserui.TemplateWidth(tpl, vars) }`.
- `docs/Design.md:286`
  `drawBottomMenu(w, termRows, termCols, viewMode, activeAction, style, labels, viewBtnRows, conditions):`
  The signature in `cmd/browser_menu.go:163` actually includes `btnActions` and `altBtnActions`: `func drawBottomMenu(w io.Writer, termRows, termCols int, viewMode string, activeAction string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, conditions map[string]bool, btnActions map[string]string, altBtnActions map[string]string) []menuButton`.
- `docs/SparklinePixelArt.md:375`
  `The testhelper package (internal/sparkline/testhelper/) provides automated`
  The package is actually at `v1/sparkline/testhelper/`, proved by `cmd/modes_presets.go:12`: `"ubunatic.com/cati/v1/sparkline/testhelper"`.
- `docs/QuadPixelArt.md:10`
  `for the internal/quadblock package`
  The package is actually `v1/quadblock`.
- `docs/RenderPipelines.md:37`
  `*   **Half-block**: [halfblock.ScaleToFit](https://codeberg.org/ubunatic/cati/src/branch/main/internal/halfblock/render.go#L82) & [halfblock.Render](https://codeberg.org/ubunatic/cati/src/branch/main/internal/halfblock/render.go#L213)`
  Link and text refer to `internal/halfblock` instead of `v1/halfblock`.
- `docs/Design.md:334`
  `## 7. Interactive Viewers (cmd/interactive.go)`
  Mentions `cmd/interactive.go` exclusively, but viewer core logic has been partially extracted to `cmd/viewer_core.go`.

## Ranked findings
1. Severity: Medium, Effort: S - Update `docs/SparklinePixelArt.md`, `docs/QuadPixelArt.md`, and `docs/RenderPipelines.md` to point to `v1/` instead of `internal/`.
2. Severity: Low, Effort: S - Update `docs/Design.md` with accurate file paths and signatures for template and viewer logic.

## Smallest next step
Update `docs/SparklinePixelArt.md:375`, `docs/QuadPixelArt.md:10`, and `docs/RenderPipelines.md:37-38` to point to `v1/` instead of `internal/`.

## Ticket corrections
None
