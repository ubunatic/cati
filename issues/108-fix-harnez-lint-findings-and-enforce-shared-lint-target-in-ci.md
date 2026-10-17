# 108 — Fix harnez lint findings and enforce shared lint target in CI

**Status**: Closed — Implemented in 3645673 and 0edb756; pinned lint and full tagged container CI pass
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

## 4. Results

The sixteen shell errors are fixed. `make lint` checks tracked files with
gofmt, vet and Harnez. GitHub CI and the container pin Harnez and its Go 1.26.5
build toolchain while renderer tests retain Go 1.26.0. Both install FFmpeg and
librsvg, and run player/browser/integration tags in fast, reference and race
modes. Containers receive the tracked-file manifest through the output mount.

The pinned linter checks 349 files without findings. The newer host linter
reports no errors and 81 size/parameter warnings, retained under issue 008.
Preflight and scripted demo navigation (1 → 2 → 3 → 2) pass.

Rootless Podman cannot create namespaces inside the quota runner; its default
socket under `/run` is also hidden. A temporary Podman API service with a socket
in the ignored output directory supports the remote engine option without
disabling quota enforcement. Harnez feedback entry `94eb4d3c` records the runner
limitation; no Harnez repository files were changed.

`make container-test` passed through that API service, including lint, build,
all tagged fast/reference suites and the full race suite. Container `cmd`
times were 26.23s fast, 26.63s reference and 168.09s with race detection.
The temporary service was stopped after completion; follow-up feedback
`9c2fabba` records the successful quota-preserving configuration.
