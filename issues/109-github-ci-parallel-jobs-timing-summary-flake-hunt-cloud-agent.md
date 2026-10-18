# GitHub CI: parallel jobs, timing summary, flake hunt

Status: In Progress — owned by the Claude cloud agent (ccr branch)

Scope: .github/ only — same design as harnez issue 809 (no flake workflow:
the sprint scopes the flake hunt to harnez and loom).

## Changes
- `.github/gotest-report.go` (shared verbatim with harnez, loom, voxi):
  failures as summary and `::error` annotations, test time, slowest 10
  packages.
- `ci.yaml`: `make ci-test` split into parallel jobs running the same
  commands — `lint` (pinned harnez, `make lint`, build), `unit` (as
  `make test-short`), `full` (tagged suite, default and `fastpath0`), `race`
  (tagged), `reuse`; `report` writes the CI wall time to the summary and a
  `::notice`. GO_TEST_ENV (GOWORK=off, GOTOOLCHAIN=go1.26.0) and per-job
  module and build caches; actions off the deprecated Node 20 majors.

## Verification
- actionlint clean; workflow_dispatch run on the ccr branch.
