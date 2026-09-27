# 063 — Show only known Loom themes in mediabrowse

**Status**: In Progress
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

## 3. Implementation & Verification Plan
- Build the browser's cycle from Loom's available theme names and keep its displayed name aligned with the applied theme.
- Ensure an unknown `--theme` value is handled clearly and update the CLI help and example README to reflect supported themes.
- Add or update tests for valid cycling, unknown input, and theme listing; verify with the mediabrowse tests and required Go checks.
