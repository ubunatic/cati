# 107 — Assert ANSI render rows and colours structurally

**Status**: Closed — Implemented in 8aa4cb8; structural colour/row checks, PTY parity and container race suites pass
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: [104](104-update-tui-integration-test-go-for-post-split-cli-flags-and-spec-driven-mode-names.md), [Testing](../docs/Testing.md)

## 1. Problem & Motivation
Raw ANSI comparisons can fail on equivalent escape encodings and plain-text comparisons miss colour regressions.

## 2. Technical Specification / Findings
Parse rendered SGR into rows of styled cells. Assert terminal cell counts, glyphs and foreground/background colours. Test equivalent encodings and colour/reset differences.

## 3. Implementation & Verification Plan
Add a reusable test helper with focused package tests and migrate relevant render assertions. Keep exact byte comparisons where byte identity is itself the contract (fastpath parity).

## 4. Results

`internal/ansitest` parses single-cell renderer rows into comparable glyph,
foreground/background and attribute state. Tests cover equivalent SGR encodings,
colour and attribute resets, palette entries, RGB values, row erasure, carriage
return, malformed controls and non-painting viewer setup. Known mouse/cursor
toggles preserve cells; unknown operations and alternate-screen switching fail.

Half-block colour, alpha, background and row-boundary assertions now inspect
every cell. Interactive/static parity compares styled rows instead of stripping
colour. Exact fast/reference byte comparisons and PNG goldens remain unchanged.
The focused PTY comparison and full host integration suite pass.
