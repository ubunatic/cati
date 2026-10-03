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
- Zoom thresholds and step policy.
- Option values and their declarative ownership.
- Playback fallback frame rate (15 vs. 24 FPS).
- Input display names.

Keep behavior and parsing mechanisms in Go where appropriate; reuse existing render-mode metadata rather than duplicating it. Record decisions and any accepted spec changes in the relevant design docs.

## 3. Implementation & Verification Plan
Inspect the live code and related specs before deciding. For each candidate, document the chosen owner and rationale; verify any accepted contract with spec integrity and behavior tests. File implementation issues for accepted changes, or close with the documented decision if no migration is warranted.
