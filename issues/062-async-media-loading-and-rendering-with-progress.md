# 062 — Async Media Loading and Rendering with Progress

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [Go Library API](../docs/GoLibrary.md), [Video & Audio Pipeline](../docs/Video.md)

---

## 1. Problem & Motivation

Loading preview videos can take 1–2 seconds, and high-resolution images or videos processed by expensive rendering algorithms can take much longer. Callers currently need a way to keep their UI responsive and show users what is happening during this work.

## 2. Technical Specification / Findings

Add an optional API/protocol for asynchronous media loading and rendering that reports progress. It should cover both loading media and rendering it, while preserving the existing synchronous usage for callers that do not need progress. The exact progress representation and cancellation/error semantics should be settled during implementation.

## 3. Implementation & Verification Plan

**Goal**: Provide an optional Go API for asynchronous media loading and rendering with useful progress updates, retaining the existing synchronous path. Done when the API is documented and tests verify progress delivery and completion/error behavior; if a required protocol or behavior decision is blocked on user input or denied permission, stop and report the decision needed.

Verify with focused Go tests for both asynchronous stages and the existing synchronous behavior, plus `go vet ./...` and `make install`.
