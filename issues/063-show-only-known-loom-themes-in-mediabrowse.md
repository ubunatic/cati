# 063 — Show only known Loom themes in mediabrowse

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `examples/mediabrowse/app.go`, `examples/mediabrowse/main.go`, `examples/mediabrowse/README.md`, Loom `spec/themes.yaml`

---

## 1. Problem & Motivation
`mediabrowse` hard-codes theme names such as `solarized-dark`, `monokai`, and `nord` for its F9 cycle and CLI help. These names are not present in the Loom version's embedded `spec/themes.yaml`; Loom silently resolves unknown names to `plain`. The browser can therefore display a theme name that does not match the colors in use.

Loom exposes its embedded theme map as `loom.SpeccedThemes`. In the current dependency version, the known names are `plain`, `mc`, `mc-classic`, `mc-dark`, and `julia256`.

## 2. Goal
`/goal Update mediabrowse so theme selection and cycling use only Loom-defined themes, with accurate user-facing theme help/docs and coverage; stop and report if the theme API cannot provide the available names.`

## 3. Implementation Milestones

### M1 (Theme Source & Deterministic Cycling)
- Use `loom.SpeccedThemes` as the authoritative source of available themes and colors.
- Resolve requested theme name: validate against `loom.SpeccedThemes`; fallback to `"mc"` (or `"plain"` if `"mc"` is missing) if unknown, ensuring `themeName` matches the colors actually applied.
- Build deterministic sorted list of theme names from `loom.SpeccedThemes` for F9 cycling.
- Ensure F9 cycle wraps around correctly and recovers cleanly from unknown/stale theme names.

### M2 (CLI Help, Documentation & Verification)
- Update CLI `--theme` flag description in `examples/mediabrowse/main.go` and `examples/mediabrowse/README.md` to list valid Loom themes (`plain`, `mc`, `mc-classic`, `mc-dark`, `julia256`).
- Update/add unit tests in `examples/mediabrowse/app_test.go` and `main_test.go` covering known theme selection, unknown theme fallback, deterministic cycling, and CLI flag defaults.
- Run full test suite (`go test ./...`), `go vet ./...`, and `make preflight`.

## 4. Outcome & Resolution
- Delivered in commit `c877b70`:
  - `themeNames()` dynamically queries and sorts keys from `loom.SpeccedThemes`.
  - `resolveTheme(name)` validates against `loom.SpeccedThemes` with deterministic fallback to `mc` (or `plain`).
  - `cycleTheme()` deterministically cycles through all specced themes.
  - CLI `--theme` flag usage and README documentation updated to reflect available Loom themes.
  - Comprehensive unit tests added covering theme resolution, fallback, deterministic cycling, and CLI flag usage.
- All tests and `make preflight` passing cleanly.
