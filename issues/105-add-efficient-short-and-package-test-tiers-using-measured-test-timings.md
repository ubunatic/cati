# 105 — Add efficient short and package test tiers using measured test timings

**Status**: Open
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
