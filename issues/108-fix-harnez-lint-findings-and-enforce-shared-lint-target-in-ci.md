# 108 — Fix harnez lint findings and enforce shared lint target in CI

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Maintenance
**Related**: [Testing](../docs/Testing.md)

## 1. Problem & Motivation
The repository has sixteen harnez lint findings in two shell scripts and CI lacks a shared lint entry point.

## 2. Technical Specification / Findings
Use gofmt, go vet and harnez lint --check on tracked Go, shell, Markdown and Makefile files. Fix shell condition/loop layout without changing behaviour. Provide harnez in GitHub CI and the test container.

## 3. Implementation & Verification Plan
Add make lint, wire it into CI, validate shell syntax and exercise the demo navigation. Run lint and container-test before completion. Report harnez false positives upstream without changing other repositories.
