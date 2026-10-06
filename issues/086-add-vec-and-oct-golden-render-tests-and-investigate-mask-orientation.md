# 086 — Add vec and oct golden render tests and investigate mask orientation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#083](083-add-vector-render-mode-using-linear-cell-split-characters.md), [#084](084-add-octant-render-mode-using-unicode-1cd0-block-characters.md), [Rendering Bug & Golden-Change Playbook](../docs/RenderingBugPlaybook.md)

---

## 1. Problem & Motivation
Some vector (`vec`) masks appear to be applied in the wrong direction, as shown in the reported screenshot. The new `vec` and `oct` modes also need inspectable golden renders so mask-orientation bugs can be diagnosed and their expected output stabilized as the modes evolve.

## 2. Technical Specification / Findings
The specific glyph masks and orientations that are wrong are not yet identified. Establish a small, representative golden set for both modes that makes directional and asymmetric coverage easy to inspect; include descriptive algorithm and parameter metadata in the PNGs.

## 3. Implementation & Verification Plan
**Goal**: Add inspectable golden render tests for `vec` and `oct`, use them to identify and correct the reported vector mask-direction error, or stop and report if blocked on a user decision or denied permission. Done means the goldens cover representative orientations for both modes, mask application has focused regression coverage, and every golden change is justified by the rendering behavior it records.
