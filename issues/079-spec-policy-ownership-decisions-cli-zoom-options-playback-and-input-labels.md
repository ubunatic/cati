# 079 — Spec policy ownership decisions: CLI, zoom, options, playback, and input labels

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: [076 — spec ownership audit](076-audit-spec-driven-approach-violations-and-classify-candidates.md), [Spec System](../docs/Spec.md), [Terminal Input](../docs/Input.md)

---

## 1. Problem & Motivation
Issue 076's audit found several stable policy and interface values currently declared in Go without an agreed spec owner. These are ownership choices, not confirmed bugs; deciding their canonical home and intended contract should precede implementation.

**Goal:** Decide whether each item below belongs in a YAML spec or remains code-owned, record the rationale and boundary, and file bounded implementation follow-ups for accepted migrations. Stop and report when a decision requires product input that is unavailable.

## 2. Technical Specification / Findings
Resolve ownership for these candidates from issue 076 §B:

- CLI flag declarations and metadata (defaults, help, hidden flags, and completion).
  **Defaults decided and done (2026-10-07):** spec-owned in `spec/cli.yaml` `defaults:`, see
  [Design.md §3.10](../docs/Design.md). Help text, hidden flags and completion remain open.
- Zoom thresholds and step policy.
- Option values and their declarative ownership.
- Playback fallback frame rate (15 vs. 24 FPS).
- Input display names.

Keep behavior and parsing mechanisms in Go where appropriate; reuse existing render-mode metadata rather than duplicating it. Record decisions and any accepted spec changes in the relevant design docs.

## 3. Implementation & Verification Plan
Inspect the live code and related specs before deciding. For each candidate, document the chosen owner and rationale; verify any accepted contract with spec integrity and behavior tests. File implementation issues for accepted changes, or close with the documented decision if no migration is warranted.

## 4. Gokay ownership triage — 2026-10-26

At `f178232`, `gokay assess --json` reports valid schemas, 17 orphan-key candidates, four unspecced constants and eight numeric shadow matches. These are heuristic findings requiring ownership decisions, not permission to migrate arbitrary matching values.

Candidates for this existing ownership review:

- `cmd/browser.go:741`: 300ms animation ticker; decide whether UI timing is spec-owned.
- `cmd/input_tester.go:66`: `maxLines = 20`; decide whether diagnostic history length is a UI policy or code-owned bound.
- `cmd/interactive.go:579,597`: 3s transient status expiry. The reported match to `spec/cli.yaml` bench-budget is unrelated behavior and is not evidence that status expiry should load the benchmark budget.
- `internal/metrics/metrics.go:303,317`: `maxExcess = 0.15`, `threshold = 0.05`; decide whether metric calibration belongs in an application spec or remains part of the metric definition, with tests and documentation for any accepted migration.

Rejected automatic-shadow conclusions: 10ms formatting threshold in `cmd/modes_benchmark.go:177` matched glyph order 10; 50ms example timing matched button priority 50; 40ms example timing matched preview height 40. The dimensions and meanings differ. Retain the distinct semantics; assess code ownership separately if needed.

The assessor does not enumerate the 17 orphan keys in its JSON, so no exact unused spec object was established and none was deleted. Schema validation and the passing spec tests in `gokay check` are distinct evidence; the check's own spec step says `skipped-empty`. Require exact paths and consumption evidence before acting on orphan counts. No change to YAML or loaders was made.

Cross-package lint findings are separately tracked by [Epic 120](120-epic-reconcile-unused-go-declarations-across-cli-and-render-packages.md); the architecture hotspot remains #112 and the broader test roadmap #111. This update leaves ownership decisions Open.
