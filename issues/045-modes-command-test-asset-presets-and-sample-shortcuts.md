# 045 — Modes Command Test Asset Presets and Sample Shortcuts

**Status**: Closed (Implemented in `cmd/modes_presets.go` and `cmd/modes.go`)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [`issues/044-modes-command-custom-image-input-support.md`](file:///home/uwe/projects/cati/issues/044-modes-command-custom-image-input-support.md), [`cmd/modes.go`](file:///home/uwe/projects/cati/cmd/modes.go)

---

## 1. Problem & Motivation
The `testdata/` directory contains rich synthetic patterns (circles, checkerboards, crosses, gradients) and real-world photographic test assets (`sample_soldering_practice`, `sample_summer_vacation`, `sample_darth_daughter`). Having built-in presets allows rapid testing without specifying long relative file paths.

## 2. Technical Specification
- Add `--sample` / `-p` / `--preset` flag to `cati modes`.
- Support standard named presets:
  - `circle`, `checker`, `cross`, `diag`, `horiz`, `verti` (from `testdata/demo_*_20x20/source.png`)
  - `soldering`, `summer`, `darth` (from `testdata/sample_*`)
  - `gradient`, `emojig`, `logo`
- Automatically map preset names to underlying asset paths.

## 3. Implementation Plan
1. Add preset registry mapping in `cmd/modes.go`.
2. Add `--sample` flag in `modesCommand()`.
3. Support listing available presets via `cati modes --sample help` or `list`.
4. Add unit test verifying preset resolution and rendering.
