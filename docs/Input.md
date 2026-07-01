---
title: Terminal Input System
weight: 20
---

# Cati — Spec-Driven Terminal Input System


Architecture, design decisions, and pitfalls for the `internal/input` package and `spec/input.yaml`.

---

## 1. Motivation

Before this system, terminal input handling was scattered across three files:

- `cmd/browser.go` — `resolveKeyName()`, `tokenizeInput()`, ad-hoc escape parsing
- `cmd/interactive.go` — `parseSGRMouse()`, `sgrIsScroll()`, `sgrIsDrag()`, `sgrButton()`, `sgrScrollDir()`
- Hard-coded byte slices in switch statements

These duplicated the same state machine logic with subtle divergences. The spec-driven approach consolidates everything into:

```
spec/input.yaml          ← single source of truth for all terminal input rules
internal/input/          ← Go package that loads and executes those rules
cmd/input_tester.go      ← --input-test TUI for live verification
```

---

## 2. `spec/input.yaml` — Structure

```yaml
input:
  key_aliases:          # named key → terminal byte sequence
    esc: "\x1b"
    tab: "\x09"
    up:  "\x1b[A"
    <c-c>: "\x03"
    # ... 30+ aliases including F1-F12

  ctrl_pattern:         # how to compute Ctrl-A through Ctrl-Z
    prefix: "c-"
    base_char: "a"
    base_code: 1

  signals:              # OS signals and their event type
    - name: SIGWINCH
      event: resize

  terminal_sequences:   # fixed sequences → named events
    - seq: "\x1b[I"
      event: focus
    - seq: "\x1b[O"
      event: defocus

  mouse:                # SGR 1006 extended mouse protocol constants
    prefix: "\x1b[<"
    press_suffix: "M"
    release_suffix: "m"
    btn_button_mask: 3
    btn_motion_flag: 32
    btn_scroll_flag: 64
    btn_no_button: 3    # SGR btn field value when no button held during move

  tokenizer:
    rules:              # ordered decision tree — first match wins
      - name: sgr_mouse
        match: starts_with
        prefix: "\x1b[<"
        scan_until: "Mm"
        emit: mouse
      - name: csi_sequence
        match: starts_with
        prefix: "\x1b["
        scan_class: alpha_tilde
        emit: key
      - name: bare_escape
        match: starts_with
        prefix: "\x1b"
        emit: key
      - name: utf8_multibyte
        match: utf8_lead    # byte ≥ 0x80, consume full codepoint
        emit: key
      - name: any_char
        match: any
        emit: key
```

The `match` types in the tokenizer:
- `starts_with` — literal prefix match; scans to end-of-prefix or to a terminator set (`scan_until`)
- `utf8_lead` — byte ≥ 0x80: consume a complete UTF-8 codepoint via `utf8.DecodeRuneInString`
- `any` — fallback, consumes one byte

---

## 3. `internal/input` Package API

```go
// Load parses spec/input.yaml from the given FS. Falls back to DefaultSpec() on error.
func Load(fsys fs.FS) (*Spec, error)

// DefaultSpec returns a hardcoded baseline matching current terminal conventions.
// Used as fallback if the YAML cannot be read at runtime.
func DefaultSpec() *Spec

// Tokenize splits a raw byte buffer into terminal event tokens using the spec's
// ordered tokenizer rules. Each token is one complete event.
func (s *Spec) Tokenize(raw string) []string

// Classify returns the EventType and structured data for a token.
func (s *Spec) Classify(tok string) Event

// ParseMouse extracts SGR 1006 mouse fields from a token.
func (s *Spec) ParseMouse(tok string) (MouseEvent, bool)

// ResolveKeyAlias maps <esc>, <c-c>, <up>, etc. to terminal byte sequences.
func (s *Spec) ResolveKeyAlias(name string) string

// KeyName returns the human-readable name for a key sequence.
// Order: alias map → printable ASCII → Backspace → Ctrl- prefix → UTF-8 → hex.
func (s *Spec) KeyName(seq string) string

// MouseName returns "Scroll Up", "Press Left", "Drag Right", "Move", etc.
func MouseName(m MouseEvent) string

// EventName returns a single human-readable string for any Event.
func (s *Spec) EventName(ev Event) string
```

Event types: `EventKey`, `EventMouse`, `EventFocus`, `EventDefocus`, `EventResize`, `EventQuit`, `EventUnknown`.

---

## 4. Mouse: SGR 1006 Protocol

The SGR extended mouse protocol encodes all mouse events as:

```
ESC [ < btn ; col ; row M    (press or motion)
ESC [ < btn ; col ; row m    (release)
```

The `btn` field is a bitmask:

| Bit(s) | Mask | Meaning |
|--------|------|---------|
| 0–1 | 0x03 | Button (0=left, 1=middle, 2=right, 3=no-button) |
| 2 | 0x04 | Shift held |
| 3 | 0x08 | Meta/Alt held |
| 4 | 0x10 | Ctrl held |
| 5 | 0x20 | Motion event |
| 6 | 0x40 | Scroll event |

**Move vs Drag** — a critical distinction:

```
btn=35 (0x20 | 0x03):  motion=true, button=3 (no button held) → IsMove()
btn=32 (0x20 | 0x00):  motion=true, button=0 (left held)      → IsDrag()
```

Before this system, `IsDrag()` checked only the motion flag, misidentifying pure moves as drags. The fix: `IsDrag()` requires `Motion && !Scroll && Button != 3`. `IsMove()` requires `Motion && !Scroll && Button == 3`.

Enable sequences:
- `\x1b[?1002h\x1b[?1006h` — button events only
- `\x1b[?1003h\x1b[?1006h` — all mouse events including motion

---

## 5. UTF-8 and Alias Ordering — Two Fixed Bugs

### Bug 1: Tab shown as "Ctrl-I"

`Tab = \x09` falls in the ctrl range (bytes 1–26 → Ctrl-A through Ctrl-Z). `KeyName` was applying the ctrl heuristic before the alias map lookup.

**Fix**: alias map is consulted FIRST in `KeyName`:
```
alias map → printable ASCII (0x20–0x7e) → Backspace (0x7f) → Ctrl-X → UTF-8 → hex
```

This ensures Tab→"Tab", Enter→"Enter", Esc→"Esc" regardless of their byte values.

### Bug 2: Multi-byte chars (ö, €) split into two tokens

The `any_char` tokenizer rule consumed one byte at a time. UTF-8 sequences like `ö` = `\xc3\xb6` produced two separate tokens.

**Fix**: the `utf8_multibyte` tokenizer rule (placed before `any_char`) detects lead bytes ≥ 0x80 and consumes a full codepoint via `utf8.DecodeRuneInString`. The same logic applies in `KeyName` to display the character directly instead of hex.

---

## 6. Integration with Browser and Viewer

### Key resolution chain

```
spec/buttons.yaml keys: ["<up>", "q"]
  → ResolveKeyAlias("<up>") → "\x1b[A"
  → loadButtonKeyDefs(inputSpec) → map[buttonName]buttonKeyDef{action, keys}
  → buildViewKeyMaps(viewRows, defs) → per-view map[key]action
  → viewKeyAction(tok) → action string
  → Go switch handler
```

`resolveKeyName()` in `cmd/browser.go` was deleted. `loadButtonKeyDefs` now takes `*input.Spec` and calls `inputSpec.ResolveKeyAlias`.

### Tokenization

In the browser and both viewers, all stdin reads pass through `inputSpec.Tokenize(raw)`. Mouse events use `inputSpec.ParseMouse(tok)` returning a typed `input.MouseEvent` with `IsScroll()`, `IsDrag()`, `IsMove()`, `Button`, `Col`, `Row`, etc. The old ad-hoc string parsing was deleted.

### `last_key` template variable

Every input loop tracks:
```go
lastKey = inputSpec.EventName(inputSpec.Classify(tok))
```
This is passed as `"last_key"` in the vars map to `drawHintBar`. The `spec/labels.yaml` hint templates use `{ last_key | dim }` to show the last input event (e.g. `"j"`, `"Up"`, `"Scroll Up"`, `"Resize"`).

---

## 7. `--input-test` TUI

```sh
cati --input-test
```

Hidden flag. Opens a raw-terminal TUI that:
- Captures all input (keyboard, mouse, focus, resize)
- Shows event type, human-readable name, hex-escaped token sequence, and coverage status
- Marks tokens not matched by any spec rule as `← unexpected`
- Collects and prints a summary of unexpected tokens on exit
- Writes a log to `/tmp/cati-input-test-TIMESTAMP.log`
- Exits only on Ctrl-C (hardcoded safeguard independent of spec)

The `hexEscape` helper renders valid UTF-8 printable codepoints (ö, €) directly and escapes raw bytes as `\xNN`.

---

## 8. Pitfalls

- **`DefaultSpec()` must stay in sync with `spec/input.yaml`**: it's the fallback when the file cannot be read at runtime. Any new tokenizer rule or key alias added to the YAML should be reflected in `DefaultSpec()`.
- **`\x03` Ctrl-C is always hardcoded** in every keyboard handler as a last-resort quit safeguard, independent of the spec. Do not remove it even if the `quit` button's `<c-c>` key is loaded from spec.
- **Tokenizer rule order matters**: `utf8_multibyte` must precede `any_char` or multi-byte chars are split. `sgr_mouse` must precede `csi_sequence` or the `\x1b[<` prefix gets misidentified.
- **`scan_class: alpha_tilde`** for CSI sequences: scans until an alphabetic character or `~`. This correctly terminates `\x1b[5~` (PgUp) at `~` and `\x1b[A` at `A`.
