# 097 — 2x4 braille mode must be dot-only: no full block, no background color

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [#069](069-add-2x4-dot-only-braille-render-mode.md), [#070](070-add-2x4-braille-foreground-and-background-render-mode.md), [#025](025-spec-driven-render-modes.md)

---

## 1. Problem & Motivation
The `2x4` mode should draw braille dots only. Today it prints `█` and paints cell
backgrounds: in the mode comparison card `2x4 (0ms, w=12, ssim=0.76)` the image sits
on a filled blue rectangle, and the yellow face has solid `█` areas between the dots.
Braille art is meant to show the image as coloured dots on the terminal's own
background (#069).

## 2. Technical Specification / Findings
- `spec/render_modes.yaml` composes `2x4: [0, 8]`. Set 0 (`fulls`) adds `" "` and `█`;
  set 8 (`brailles`) is the generated `braille_2x4` set.
- The composition is rendered with fg/bg colouring, so every cell gets a background
  colour (the blue rectangle), and solid areas pick `█`.
- The dedicated dot-only renderer from #069 (`v1/braille`, `braille/dots`) was removed
  in da8da22 ("remove experimental braille renderer package"); `2x4` now goes through
  the generic composed renderer.
- Open question: whether the composed renderer has, or needs, a foreground-only colorer
  (dots in fg, no bg escape, undotted pixels count as the terminal background). The
  blank braille cell U+2800 and the full pattern `⣿` may already cover the empty and
  full cases, so set 0 may simply be dropped from the composition.

## 3. Implementation & Verification Plan
/goal Make `2x4` render braille dots only — no `█`, no background colour escapes — and
verify it, or stop and report when blocked on a user decision or denied permission.

Done when:
- `cati <image> -m 2x4` output contains no `█` and no `48;2;` background escapes
  (codified as a test on the property, not a pixel).
- Spec, docs (`composition_info`, `docs/SparklinePixelArt.md`) and any goldens follow
  the rendering-bug playbook: predict which goldens change before regenerating them.

Re-check the live spec and renderer before starting; the findings above date from
2026-10-07.
