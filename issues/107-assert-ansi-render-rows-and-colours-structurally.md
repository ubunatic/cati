# 107 — Assert ANSI render rows and colours structurally

**Status**: Open
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
