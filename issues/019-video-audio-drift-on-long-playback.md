# 019 — Video audio drift on longer playback

**Status:** 🔴 Open  
**Refs:** [Video.md](../docs/Video.md), [005-video-ffmpeg-scaling.md](005-video-ffmpeg-scaling.md)

---

## Problem

On longer videos, the audio playback drifts away from the video. Short clips look
fine, but sustained playback eventually loses sync.

This is a playback correctness issue, not a scaling or codec-compatibility issue.

## Likely Areas

- `playVideos` audio lifecycle
- `interactiveVideo` / video tick pacing
- ffplay startup latency vs video frame start
- blocking behavior when frames are dropped or delayed
- any mismatch between decoded video timestamps and audio wall-clock time

## Investigation Goal

Determine whether drift is caused by:

1. starting audio and video at slightly different times,
2. video pacing being tied to frame delivery rather than presentation time,
3. audio running free while video stalls on rendering or buffering, or
4. a backend-level ffplay/ffmpeg synchronization issue.

## Notes

- Prefer proving the drift mechanism with measurements before changing the pipeline.
- If the cause is in the main loop, keep the fix minimal and local.
- If the cause is ffplay timing, document the limitation and evaluate a more explicit sync strategy.
