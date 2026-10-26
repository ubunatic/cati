# 118 — Fix unchecked fmt.Sscanf in spec/glyph_sets_test.go

**Status**: Closed — checked error return from fmt.Sscanf in glyph_sets_test
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Quality
**Related**: Originally local issue 115; renumbered to 118 when integrating GitHub main to preserve camera epic 115. Fixed by commit `1e9445f`.

---

## 1. Problem & Motivation
`gokay check --all` flags an errcheck violation in `cati`:
`../home/uwe/projects/cati/spec/glyph_sets_test.go:418:14: Error return value of fmt.Sscanf is not checked (errcheck)`

## 2. Technical Specification / Findings
In `spec/glyph_sets_test.go:418`:
Check the error from `fmt.Sscanf(key, "%dx%d", &w, &h)` and fail the test via `t.Fatalf` if parsing fails.

## 3. Implementation & Verification Plan
- Update `spec/glyph_sets_test.go`.
- Run `go test ./spec` in `cati`.
- Re-run `gokay check --all` in `cati`.
