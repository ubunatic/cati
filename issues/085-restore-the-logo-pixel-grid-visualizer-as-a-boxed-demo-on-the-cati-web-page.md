# 085 — Restore the logo pixel-grid visualizer as a boxed demo on the cati web page

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: ubunatic.com issue 071 (Step 1 open note), ubunatic.com commit `e2cf495^:cati/index.html` (old page)

---

## 1. Problem & Motivation

The old cati page on ubunatic.com had a small animated visualizer: the 24×14 cati logo drawn as raw
pixels, with a highlight sweeping through row pairs to show how cati packs two pixel rows into one
terminal cell (▀ ▄ █). The new generated page (from `website/page.yaml`, ubunatic.com issue 071)
dropped it. It explains the core rendering idea better than text.

## 2. Technical Specification / Findings

- Old markup and script: `git -C ~/projects/ubunatic.com show e2cf495^:cati/index.html`, the
  `#pixel-grid` element (~line 848), the `.pixel-demo` CSS (~line 196) and the script after
  `// Pixel grid` (~line 1089). The colors were inlined between `PIXELS_START`/`PIXELS_END`
  by the since-removed `scripts/generate_pixels.go`.
- The site supports boxed demos: a `demo` block in `page.yaml` renders
  `<iframe sandbox="allow-scripts">`. The box cannot fetch files, so data and scripts must be
  inline or classic `<script src>` files inside the demo dir (see emojig's `website/demo/`).
- The demo must respect `prefers-reduced-motion` and both themes.

## 3. Implementation & Verification Plan

/goal The cati page on ubunatic.com shows the animated logo pixel-grid in a boxed demo, sourced from
cati's `website/`, or stop and report when blocked on an owner decision or denied permission.

Check the live cati `website/` and the site's demo support first; this ticket may be stale. Add
`website/demo/pixel-grid/` (generated colors from `cati_0001.png`, kept in sync by a check), add a
`demo` block to `website/page.yaml`, then `uman website sync cati` (hooks run `make gen-tools`
and `make check`). Verify with screenshots in light and dark theme at 1200 and 390 px, and with
reduced motion.
