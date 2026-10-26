# 121 — Reconcile legacy Archived statuses with harnez issues lint

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [Archived geometry ticket 010](archive/010-unified-zoom-geometry-and-ladder.md), [Archived viewport ticket 012](archive/012-viewport-geometry-mode-switch-regression.md)

---

## 1. Problem & Motivation
The session's extended self-test probes found `harnez issues lint` exits 1 on Cati's legacy Archived metadata, while `harnez index --check` reports all indexes current. This prevents a clean tracker lint result despite no index drift.

**Goal:** Reconcile Cati's legacy archival metadata with the supported tracker lifecycle, preserving historical content, links and consolidation decisions, and verify tracker lint plus index consistency; or stop and report if the intended lifecycle requires an unavailable owner decision. On a loop or no progress, record attempts and resume point rather than rewriting history blindly.

## 2. Technical Specification / Findings
Observed diagnostic:

```text
Error: issues lint: archive/010-unified-zoom-geometry-and-ladder.md has unknown status "🗄️ Archived — content merged into [008 — Spaghetti Code & Viewport Geometry Refactoring Plan](../008-spaghetti-code-analysis.md)" and cannot appear in the three lifecycle indexes
```

The check stops at its first finding; inspect other legacy archival entries before claiming this is the only incompatible status. No metadata was repaired during the session. This is separate from the 36 Go unused diagnostics in #120. Harnez's auto-archiving design ticket 844 concerns a different decision and is not assumed to resolve existing Cati metadata.

## 3. Implementation & Verification Plan
Re-read the live tracker contract, inspect the legacy tickets, and choose supported metadata without reviving work already consolidated into #008. If supported historical metadata is instead being rejected incorrectly, file the confirmed tooling defect in Harnez rather than weakening checks. Preserve existing source links and history. Verify `harnez issues lint`, `harnez index --check`, and any changed relative links. Keep the issue Open until both tracker checks pass.
