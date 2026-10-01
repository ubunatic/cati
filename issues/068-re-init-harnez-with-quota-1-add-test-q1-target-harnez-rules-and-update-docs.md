# 068 — Re-init harnez with --quota-1: add test-q1 target, harnez rules and update docs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Chore
**Related**: loom issue 235

---

/goal Bring cati to the standard harnez setup (Quota-1 `make test-q1`, `.harnez/rules`, current managed docs),
or stop and report when blocked on a user decision.

## 1. Problem & Motivation
Agents in cati lack the standard harnez guardrails, so cross-repo sprints need per-repo special cases.

## 2. Technical Specification / Findings
Found 2026-10-01 during the loom module switch (loom issue 235): developer agents in this repo
found 5 rule files but no `make test-q1` target, so the one-run test budget could not be enforced and agents had to fall back to
`GOWORK=off go test ./...`. Projects with full setup (loom, settings, harnez, voxi) have 6-7 rule files and
`test-q1`.

## 3. Implementation & Verification Plan
- `harnez init -d . --quota-1` (check `harnez init --help` for docs to add, e.g. `--docs golang`).
- Review the generated `AGENTS.md`, `Makefile` and `docs/` changes; keep project-specific content.
- `make test` currently fails on the toolchain pin (issue 067); fix that first or together.
- Keep the double run (CATI_FASTPATH=1 and 0) inside `test-q1`.
- Verify: `harnez read .harnez/rules/...` lists the rules; `make test-q1` runs the full suite once.
