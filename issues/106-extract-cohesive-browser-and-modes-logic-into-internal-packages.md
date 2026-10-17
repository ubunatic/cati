# 106 — Extract cohesive browser and modes logic into internal packages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: [008](008-spaghetti-code-analysis.md), [System](../docs/System.md)

## 1. Problem & Motivation
Browser, modes and interactive command files mix reusable presentation logic with CLI orchestration and are difficult to test independently.

## 2. Technical Specification / Findings
Extract cohesive presentation and layout logic into internal packages with their own tests. Keep stable public APIs and CLI behaviour unchanged. Leave stateful orchestration in cmd for bounded, reviewable steps.

## 3. Implementation & Verification Plan
Move pure components and their tests without changing algorithms. Document package boundaries, run go test ./... after each step, and preserve all PNG goldens.
