# 061 — Loom view truncates ANSI files

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `examples/mediabrowse/mockup/mediabrowse.ansi`

---

## 1. Problem & Motivation

Running `loom view examples/mediabrowse/mockup/mediabrowse.ansi` displayed output that appeared capped/truncated. The screenshot shows the mediabrowse mockup cut off at the right edge, with the panel contents and bottom hint bar incomplete. The exact cap mechanism and whether it depends on terminal dimensions are not yet known.

## 2. Technical Specification / Findings

Reproduce with the command above and determine where the output is being limited (ANSI file, Loom viewer, or terminal layout). Record any terminal-size dependency.

## 3. Implementation & Verification Plan

/goal Identify and fix the cause so `loom view` displays the complete ANSI mockup at supported terminal sizes, and verify the command; stop and report if a user decision or denied permission blocks progress.

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Describe the problem and why it matters.

## 2. Technical Specification / Findings
Record relevant technical details and findings.

## 3. Implementation & Verification Plan
Describe the implementation and how it will be verified.
