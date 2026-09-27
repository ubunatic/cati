# 059 — OpenVideoStream reader and cleanup both call cmd.Wait, stop hangs unless channel is drained

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: `v1/halfblock` `OpenVideoStream` (v0.2.6); loom issue 111 (`media/widget.go` workaround)

---

## 1. Problem & Motivation
Found by loom's media widget (loom 111, M4): if a consumer cancels the stream context and
calls the returned stop func without first draining the frame channel, stop can hang. The
frame reader goroutine and the cleanup path both call `cmd.Wait` on the same ffmpeg process.
Loom works around this by cancelling, then draining the channel (`for range frames {}`), and
only then calling stop.

## 2. Technical Specification / Findings
- `cmd.Wait` must be called exactly once. A second concurrent call races or blocks.
- The consumer contract (drain the channel or not) is undocumented.

## 3. Implementation & Verification Plan
Make stop idempotent and safe without draining: one goroutine owns `cmd.Wait`, and stop
cancels, kills the process and waits on a done channel. Test: open a stream, read one frame,
call stop without draining, and it must return within a timeout with no goroutine leak (skip
when ffmpeg is missing). Afterwards loom can remove its drain workaround.
