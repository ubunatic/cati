# 003 — Style: hint bar used hardcoded reverse-video instead of theme palette

**Status:** ✅ Closed  
**Refs:** [Design.md](../docs/Design.md#3-style-specification-specstyle.yaml)

---

## Symptom

The browser grid has two bars at the bottom of the screen:

- **Row `termRows-1`** — clickable button bar (`[◀ Prev]  [Next ▶]  …`) rendered by
  `drawBottomMenu`, using dark slate palette from `style.yaml`.
- **Row `termRows`** — shortcut hint bar (`[Enter/Click] View/Enter …`) rendered inline with
  a hardcoded `\x1b[7m` (reverse-video) escape → **white/light background**, dark text —
  visually inconsistent with the styled bar above it.

## Fix Applied

1. Added `ControlBarFg string` to `StyleConfig` (default `#94a3b8`, same as `BtnFg`).
2. Added `fg` key parsing under `control_bar` in `loadStyle()`.
3. Added `fg: "#94a3b8"` to `spec/style.yaml` under `control_bar`.
4. Replaced `\x1b[7m...\x1b[m` with `styleBG(style.ControlBarBg) + styleFG(style.ControlBarFg)`.

Now both bars share the same dark-slate background and muted-slate text. Both can be recoloured
from a single `control_bar` section in `spec/style.yaml`.

## Design Note

The `control_bar` section in `style.yaml` now owns the palette for the entire bottom area:

```yaml
control_bar:
  bg: "#0f172a"   # background for both button bar row and hint bar row
  fg: "#94a3b8"   # text colour for the hint/shortcut bar
```

Individual button colours are still governed by the `buttons` section.
