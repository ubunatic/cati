# 004 — Spec System: Loose Ends & Unresolved Stubs

**Status:** 🔴 Open  
**Refs:** [Design.md §3](../docs/Design.md#3-spec-system-spec)

---

## A. `website` button — undefined in `buttons.yaml`

✅ **Fixed** — `website` button added to `buttons.yaml` with `action: open_website`. URL stored in `labels.yaml` as `website_url`. `openWebsite(url)` calls `xdg-open`/`open`/`start` depending on OS. Wired in the about-view click handler and in the spec-driven grid keyboard dispatch.

---

## B. `image_viewer` and `video_player` view modes — layouts declared but not inline

✅ **Fixed** — chose option 2 (keep external viewers, pass button context into them).

`interactiveWithChan` and `interactiveVideo` now accept `style`, `labels`, `viewBtnRows` and call `drawBottomMenu`/`drawHintBar` during their redraw. The image takes `termRows-2` rows; the bottom 2 rows are the button bar and hint bar.

`interactiveVideo` also:
- Enables mouse tracking on entry (so buttons are clickable)
- Adds `paused bool` state; Space bar toggles pause
- Passes `conditions["playing"] = !paused` to `drawBottomMenu` so `{ if(playing, pause, play) }` resolves correctly

---

## C. `theme.yaml` style tokens — stored but not applied

`buttons.yaml` has `style: danger` / `style: primary` etc. referencing `spec/theme.yaml` tokens. `loadButtons` parses the `text:` field but ignores `style:`. The theme tokens are never applied to button rendering.

**Status:** 🔴 Still open. Fix: in `loadButtons` (or `drawBottomMenu`), resolve `style` → theme token → `fg`/`bg` overrides for that button.

---

## D. `controls.yaml` — declared but not read

✅ **Fixed** — `loadControls()` reads `spec/controls.yaml` and returns `[]ControlSpec`. The settings form (`drawSettingsPage`) is now driven by this slice: field labels derive from `settingsFieldLabel(key)`, tab cycling uses `len(controls)`, and inc/dec bounds come from `c.Min`/`c.Max`. `applySettingsDelta` dispatches on `c.Key` to update the right field in the `Settings` struct.

---

## E. Settings view has no `inc`/`dec` adjustment buttons

The old `{ inc } { dec } { save } { cancel }` layout was replaced with `{ save } { cancel } | { quit }`. The `inc`/`dec` mouse-clickable buttons are gone. Settings can only be adjusted via keyboard (`↑`/`↓`/`Tab`). This is intentional per spec (controls.yaml is the future path) but may surprise users expecting to click.

**Status:** 🔴 Decision deferred — keyboard-only for now is consistent with controls.yaml plan.
