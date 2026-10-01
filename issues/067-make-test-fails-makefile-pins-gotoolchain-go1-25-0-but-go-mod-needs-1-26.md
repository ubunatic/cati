# 067 — make test fails: Makefile pins GOTOOLCHAIN=go1.25.0 but go.mod needs 1.26

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: loom issue 235 (found during the loom module switch)

---

/goal Make `make test` run cati's test suite again, or stop and report when blocked on a user decision
(e.g. which Go version to pin).

## 1. Problem & Motivation
`make test` fails before running any test: the Makefile sets
`GO_TEST_ENV := GOWORK=off GOTOOLCHAIN=go1.25.0` while `go.mod` declares `go 1.26.0` (since ea51211,
image decoders). The suite therefore only runs via plain `GOWORK=off go test ./...`.

## 2. Technical Specification / Findings
Found 2026-10-01 while switching cati to `ubunatic.com/loom`; `GOWORK=off go test ./...` passes.
Check why the pin exists (reproducible test toolchain?) before changing it; the same pin may be in
`test-player`, `test-browser` and `test-integration`.

## 3. Implementation & Verification Plan
- Align the pin with `go.mod` (or drop it), in one place.
- Verify: `make test` and `make test-all` run and pass.
