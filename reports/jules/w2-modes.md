# w2-modes

## Verdict
The `cmd/modes*.go` files are overcrowded and tightly coupled with shared types (like `renderCfg` and `renderModeEntry`), which complicates modularizing the monolithic `cmd` package (as required by Issue 112). Extracting modes logic requires decoupling these types or moving them into a foundational internal package.

## Evidence
- `cmd/modes.go:32`: `func modesCommand() *cobra.Command {` starts a massive CLI command handling file parsing, options and flags setup.
- `cmd/modes.go:223`: `func isImageFileOrPreset(arg string) bool {` couples CLI routing with asset types.
- `cmd/modes_metrics.go:28`: `func renderModePairRaw(leftSrc, rightSrc image.Image, width int, cfg renderCfg, smart bool, name string, termRows int) (modePairRawResult, error) {` demonstrates tight coupling to `renderCfg`.
- `cmd/interactive_modes.go:16`: `	cfg          renderCfg` shows that `renderCfg` is also deeply embedded in the interactive viewer.
- `cmd/modes_benchmark.go:22`: `func runModesBenchmarkScorecard(out io.Writer, width int, smart bool, entries []renderModeEntry, sortKey string) error {` relies on `renderModeEntry`.

## Smallest next step
1. Create a foundational internal package (e.g. `internal/clirender`) and move `renderCfg` and `renderModeEntry` (along with associated registry parsing logic) from `cmd/` into it.
2. Update all references in `cmd/interactive_modes.go`, `cmd/interactive_render.go`, `cmd/modes*.go`, `cmd/browser.go`, and others to use the shared package.
3. Extract `modes.go` logic (cards, metrics, presets, benchmarks) into `internal/climodes`.

## Ticket corrections
Issue 112 correctly captures that `modes.go` depends on `renderModeEntry` and `renderCfg` and states that extracting modes requires creating an `internal/clirender` or `internal/rendermodes` package first.
