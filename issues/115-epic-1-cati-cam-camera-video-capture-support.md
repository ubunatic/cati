# Epic 1: cati cam - Camera & Video Capture Support

**Status:** Open
**Category:** Epic
**Priority:** High

## /goal

Design and implement Cobra CLI verbs (`cati cam view`, `cati cam record`, `cati cam devices`) allowing `cati` to access local cameras / video capture devices on the host OS (Linux v4l2, macOS AVFoundation, Windows MediaFoundation, or ffmpeg device capture backend).

Exit clause / circuit breaker: Implement camera enumeration and streaming frame pipeline or stop and report if video device drivers or platform dependencies prevent native frame capture without heavy external runtime dependencies.

## Out of Scope

- Full interactive TUI widgets or keyboard event loop (covered in Epic 2).
- Network streaming server / RTSP re-encoding (covered in Epic 3).

## Open Decisions

- Direct platform v4l2/AVFoundation native bindings vs. delegating raw frame ingestion to ffmpeg pipe input.

## Task Table

| Ticket | What | Sprint Group | Lead | Reviewer | Status |
|--------|------|--------------|------|----------|--------|
| TBD-1  | Video device discovery (`cati cam devices`) | SG1 | Developer | Architect | Open |
| TBD-2  | CLI verb routing for `cati cam view` and `cati cam record` | SG1 | Developer | Architect | Open |
| TBD-3  | Frame grabber engine & ASCII/quadblock pipeline integration | SG2 | Developer | Architect | Open |
| TBD-4  | `/evergreen` and `/wrap` final verification | SG2 | Orchestrator | Owner | Open |
