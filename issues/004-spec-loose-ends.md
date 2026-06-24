# 004 — Spec System: Loose Ends & Unresolved Stubs

**Status:** 🔴 Open  
**Refs:** [Design.md §3](../docs/Design.md#3-spec-system-spec)

---

## A. `website` button — undefined in `buttons.yaml`

`spec/views.yaml` about view references `{ website }`:
```yaml
about:
  - row: "{ back } { website } | { quit }"
```
But `spec/buttons.yaml` has no `website` entry and `loadButtons` has no default for it.

**Effect at runtime:** button label falls back to the raw key string `"website"`, no action fires on click.

**Fix:** add to `buttons.yaml`:
```yaml
  website:
    text: "🌐 Website"
    style: secondary
    action: open_website
```
And wire `open_website` action in the browser click handler (opens `https://codeberg.org/ubunatic/cati`).

---

## B. `image_viewer` and `video_player` view modes — layouts declared but not inline

`spec/views.yaml` defines `image_viewer` and `video_player` layouts. The browser draws buttons for view mode `"image_viewer"` or `"video_player"` via `drawBottomMenu`. But the browser's `viewMode` variable only takes values `"grid"`, `"preview"`, `"about"`, `"settings"`. The image/video viewer runs as a blocking sub-process (via `cmd/interactive.go`), not as an inline view mode.

**Effect:** the `image_viewer`/`video_player` button rows are never rendered.

**Options:**
1. Convert viewer to inline view mode — set `viewMode = "image_viewer"` and render the viewer inside the browser redraw loop (major refactor).
2. Keep external viewer but pass a button row to it so it can render its own controls.
3. Leave as-is if the external viewer already handles its own key bindings.

---

## C. `theme.yaml` style tokens — stored but not applied

`buttons.yaml` has `style: danger` / `style: primary` etc. referencing `spec/theme.yaml` tokens. `loadButtons` parses the `text:` field but ignores `style:`. The theme tokens are never applied to button rendering.

**Fix:** in `loadButtons` (or `drawBottomMenu`), resolve `style` → theme token → `fg`/`bg` overrides for that button.

---

## D. `controls.yaml` — declared but not read

`spec/controls.yaml` declares runtime control bindings (`set`/`get` action names, `min`/`max`). Nothing reads this file. Settings are currently keyboard-only and hardcoded in `drawSettingsPage`.

**Fix:** load controls and use them to drive the settings form dynamically.

---

## E. Settings view has no `inc`/`dec` adjustment buttons

The old `{ inc } { dec } { save } { cancel }` layout was replaced with `{ save } { cancel } | { quit }`. The `inc`/`dec` mouse-clickable buttons are gone. Settings can only be adjusted via keyboard (`↑`/`↓`/`Tab`). This is intentional per spec (controls.yaml is the future path) but may surprise users expecting to click.

**Decision needed:** keep keyboard-only for now (consistent with controls.yaml plan), or re-add `inc`/`dec` as a temporary bridge.
