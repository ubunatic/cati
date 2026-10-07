# 096 — Tune vec3 and vec4 vector glyph tips with hand-drawn masks

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: #090

---

## 1. Problem & Motivation
At odd cell sizes the 50% coverage rule drops thin diagonal tips: edge pixels that are
33–49% covered stay off, so glyphs look shorter than they are. For `vec5` (3x5) this was
fixed on 2026-10-07 with hand-drawn `masks: { 3x5: [...] }` in `spec/render_modes.yaml`.
`vec3` (2x3) and `vec4` (3x4) likely have the same problem. The user parked this ("later").

## 2. Technical Specification / Findings
- The 3x5 rule, approved by the user: turn on every pixel at least about 1/3 covered.
- Exceptions at 3x5:
  - The corner triangles 🬼🭇🭗🭢 stay dots.
  - The middle-third sextants 🬃🬇🬋 stay one row.
  - The partners 🭚🭜🭥 are pinned to their computed masks.
- Coverage probe: rasterize with `vectorShapes` at 20× the size and average 20×20 blocks;
  list pixels at 33–49% coverage that are off.

## 3. Implementation & Verification Plan
- Run the probe at 2x3 and 3x4, propose drawings in ASCII, and get the user's approval.
- Add `masks:` entries; `TestVectorDrawnMasksAreUsed` covers them.
