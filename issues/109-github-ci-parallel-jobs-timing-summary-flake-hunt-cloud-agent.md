# GitHub CI: parallel jobs, timing summary, flake hunt

Status: In Progress — owned by the Claude cloud agent (ccr branch)

Scope: .github/ only — same design as harnez issue 809 (no flake workflow:
the sprint scopes the flake hunt to harnez and loom).

## Changes
- `.github/gotest-report.go` (shared verbatim with harnez, loom, voxi):
  failures, test time, slowest 10 packages per job summary.
- `ci.yaml`: parallel `lint` (fmt-check, vet, build), `unit` (`go test -short`,
  default and `fastpath0`), `race`, `reuse`; `report` writes the CI wall time.
  Jobs use the Makefile's GO_TEST_ENV (GOWORK=off, GOTOOLCHAIN=go1.26.0) and
  per-job module and build caches. Actions moved off the deprecated Node 20
  majors.

## Verification
- actionlint clean; workflow_dispatch run on the ccr branch.
