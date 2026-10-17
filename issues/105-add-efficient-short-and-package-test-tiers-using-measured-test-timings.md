# 105 — Add efficient short and package test tiers using measured test timings

**Status**: Closed — Implemented in 3645673 and 0edb756; warm short tier 9.25s, full/package-race/container checks pass
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: [Testing](../docs/Testing.md)

## 1. Problem & Motivation
The cmd test package takes about 35 seconds and consumes substantial CPU. Developers need a fast unit tier and isolated race-enabled package checks.

## 2. Technical Specification / Findings
Measure uncached per-test and package timings with go test -json. Keep full test coverage while skipping media, PTY, integration, and expensive sweeps in short mode. Share expensive fixtures and parallelize isolated sweeps safely.

## 3. Implementation & Verification Plan
Add test-short and test-pkg targets. Record timings and test tiers in Testing.md. Verify full tests, the short tier (target below ten seconds with warm build caches), isolated package checks, and container-test.

## 4. Results

The JSON baseline measured 43.59s in `cmd`. The final warm short tier took
9.25s overall (4.64s in `cmd`), retaining spec, focused geometry and PNG golden
coverage. Media, external SVG, verbose showcases and exhaustive sweeps skip
short mode. Vet remains in lint, full tests and preflight.

Golden cases share decoded source fixtures; read-only golden and isolated
mode sweeps run concurrently. The slow-mode benchmark advances a fake clock;
output-only benchmark cases no longer spend hundreds of milliseconds sampling.
Full host fast/reference and tagged suites, plus `test-pkg PKG=./internal/...`
with race detection, pass. `scripts/test_timings.go` reproduces the timing report.
