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

Rechecked during the 2026-10-03 aspect session: the mandated six-file
`harnez read` still reports `.harnez/rules/Quota.md` missing. Other required
rules load, and optional Local.md is absent. The current Makefile already pins
Go 1.26.0 and the aspect session's full fast/reference tests passed, so the old
Go 1.25 toolchain mismatch below is no longer an observed blocker. Reconcile
the required rule list with generated files when this setup issue is addressed;
the aspect work did not modify AGENTS.md or re-run harnez init.

## 3. Implementation & Verification Plan
- `harnez init -d . --quota-1` (check `harnez init --help` for docs to add, e.g. `--docs golang`).
- Review the generated `AGENTS.md`, `Makefile` and `docs/` changes; keep project-specific content.
- Verify the current toolchain pin (Go 1.26.0 as of 2026-10-03); the original issue 067 pin mismatch is no longer reproduced by the aspect session's test run.
- Keep the double run (CATI_FASTPATH=1 and 0) inside `test-q1`.
- Verify: `harnez read .harnez/rules/...` lists the rules; `make test-q1` runs the full suite once.
