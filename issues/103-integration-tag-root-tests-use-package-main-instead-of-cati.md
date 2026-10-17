# 103 — Integration-tag root tests use package main instead of cati

**Status**: Closed — Changed package declaration to cati_test and removed duplicate var in tui_integration_test.go (commit c720887)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [024](024-split-cli-player-browser-binaries.md), [Testing](../docs/Testing.md)

---

## 1. Problem & Motivation
The optional terminal integration suite cannot compile after the library/CLI
split. Default tests pass, so this only appears with the integration build tag.

## 2. Technical Specification / Findings
`GOWORK=off GOTOOLCHAIN=go1.26.0 go vet -tags=integration .` reports:

```text
found packages main (tui_integration_test.go) and cati (version.go)
```

`tui_integration_test.go` declares `package main`; the root's ordinary source
declares `package cati`. `make test-integration` targets this root package.
Found while adding media prerequisites in ticket 100. No rendering changes
or golden updates are needed to fix this package mismatch.

## 3. Implementation & Verification Plan
Move the test to the appropriate package (or external `cati_test`), check its
build helper still points at `./cmd/cati`, then run `make test-integration`.
Investigate any remaining CLI migration failures separately; do not weaken
terminal geometry assertions to make the suite pass.
