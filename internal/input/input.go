// Package input parses and classifies terminal input events from spec/input.yaml.
package input

import (
	"io/fs"
	"strconv"
	"strings"
	"unicode/utf8"
)

// EventType classifies a decoded terminal input event.
type EventType string

const (
	EventKey     EventType = "key"
	EventMouse   EventType = "mouse"
	EventFocus   EventType = "focus"
	EventDefocus EventType = "defocus"
	EventResize  EventType = "resize"
	EventQuit    EventType = "quit"
	EventUnknown EventType = "unknown"
)

// MouseEvent holds decoded SGR 1006 mouse event data.
type MouseEvent struct {
	Btn     int
	Col     int
	Row     int
	Release bool
	Button  int
	Shift   bool
	Meta    bool
	Ctrl    bool
	Motion  bool
	Scroll  bool
}

func (m MouseEvent) IsScroll() bool { return m.Scroll }

// IsDrag reports a button-held drag: motion flag set, scroll flag clear, and a
// real button (0–2) held. Button==3 in SGR means no button held → that is a
// pure move, not a drag.
func (m MouseEvent) IsDrag() bool { return m.Motion && !m.Scroll && m.Button != 3 }

// IsMove reports a pure mouse move with no button held (SGR button field == 3
// with the motion flag set, emitted only in all-motion tracking mode).
func (m MouseEvent) IsMove() bool { return m.Motion && !m.Scroll && m.Button == 3 }

func (m MouseEvent) ScrollDir() int {
	if m.Btn&1 == 0 {
		return -1
	}
	return 1
}

// Event is a classified terminal input event.
type Event struct {
	Type  EventType
	Token string
	Mouse MouseEvent
}

type tokRule struct {
	name      string
	matchType string // "starts_with" | "any"
	prefix    string
	scanUntil string // terminator chars for scan_until mode
	scanClass string // "alpha_tilde"
	emit      EventType
}

// Spec holds the parsed input.yaml state machine configuration.
type Spec struct {
	keyAliases         map[string]string
	ctrlPrefix         string
	ctrlBaseChar       byte
	ctrlBaseCode       int
	mousePrefix        string
	mousePressS        string
	mouseReleaseS      string
	mouseBtnButtonMask int
	mouseBtnShiftMask  int
	mouseBtnMetaMask   int
	mouseBtnCtrlMask   int
	mouseBtnMotionFlag int
	mouseBtnScrollFlag int
	tokenizerRules     []tokRule
	terminalSeqs       map[string]EventType
}

// DefaultSpec returns a built-in Spec that matches the current hardcoded behaviour.
// It does not require spec/input.yaml to be present.
func DefaultSpec() *Spec {
	s := &Spec{
		keyAliases: map[string]string{
			"esc":      "\x1b",
			"escape":   "\x1b",
			"bs":       "\x7f",
			"backspace": "\x7f",
			"cr":       "\x0d",
			"enter":    "\x0d",
			"return":   "\x0d",
			"lf":       "\x0a",
			"nl":       "\x0a",
			"space":    " ",
			"tab":      "\t",
			"del":      "\x1b[3~",
			"delete":   "\x1b[3~",
			"up":       "\x1b[A",
			"down":     "\x1b[B",
			"right":    "\x1b[C",
			"left":     "\x1b[D",
			"pgup":     "\x1b[5~",
			"pageup":   "\x1b[5~",
			"pgdn":     "\x1b[6~",
			"pagedown": "\x1b[6~",
			"home":     "\x1b[H",
			"end":      "\x1b[F",
			"f1":  "\x1bOP", "f2": "\x1bOQ", "f3": "\x1bOR", "f4": "\x1bOS",
			"f5":  "\x1b[15~", "f6": "\x1b[17~", "f7": "\x1b[18~", "f8": "\x1b[19~",
			"f9":  "\x1b[20~", "f10": "\x1b[21~", "f11": "\x1b[23~", "f12": "\x1b[24~",
		},
		ctrlPrefix:         "c-",
		ctrlBaseChar:       'a',
		ctrlBaseCode:       1,
		mousePrefix:        "\x1b[<",
		mousePressS:        "M",
		mouseReleaseS:      "m",
		mouseBtnButtonMask: 0x03,
		mouseBtnShiftMask:  0x04,
		mouseBtnMetaMask:   0x08,
		mouseBtnCtrlMask:   0x10,
		mouseBtnMotionFlag: 0x20,
		mouseBtnScrollFlag: 0x40,
		terminalSeqs: map[string]EventType{
			"\x1b[I": EventFocus,
			"\x1b[O": EventDefocus,
		},
		tokenizerRules: []tokRule{
			{name: "sgr_mouse", matchType: "starts_with", prefix: "\x1b[<", scanUntil: "Mm", emit: EventMouse},
			{name: "csi_sequence", matchType: "starts_with", prefix: "\x1b[", scanClass: "alpha_tilde", emit: EventKey},
			{name: "bare_escape", matchType: "starts_with", prefix: "\x1b", emit: EventKey},
			{name: "utf8_multibyte", matchType: "utf8_lead", emit: EventKey},
			{name: "any_char", matchType: "any", emit: EventKey},
		},
	}
	return s
}

// Load reads and parses spec/input.yaml from fsys. Falls back to DefaultSpec on error.
func Load(fsys fs.FS) (*Spec, error) {
	data, err := fs.ReadFile(fsys, "input.yaml")
	if err != nil {
		return DefaultSpec(), err
	}
	return parse(string(data))
}

// resolveEscapes replaces \xNN and \t notation in a string value read from YAML.
func resolveEscapes(s string) string {
	s = strings.ReplaceAll(s, `\t`, "\t")
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+3 < len(s) && s[i+1] == 'x' {
			hex := s[i+2 : i+4]
			if v, err := strconv.ParseUint(hex, 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 4
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func stripYAMLValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	} else if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		v = v[1 : len(v)-1]
	}
	return resolveEscapes(v)
}

func indentLevel(line string) int {
	n := 0
	for _, c := range line {
		if c == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

func parse(data string) (*Spec, error) {
	s := DefaultSpec()
	s.keyAliases = map[string]string{}
	s.terminalSeqs = map[string]EventType{}
	s.tokenizerRules = nil

	lines := strings.Split(data, "\n")

	section := ""       // top-level key under "input:"
	subSection := ""    // second-level key
	inInput := false

	// For list items in signals / terminal_sequences / tokenizer.rules
	type listItem struct {
		fields map[string]string
	}
	var curItem *listItem

	commitItem := func() {
		if curItem == nil {
			return
		}
		switch section {
		case "signals":
			// nothing stored in Spec directly (signals are handled by OS signal package)
		case "terminal_sequences":
			seq := resolveEscapes(curItem.fields["seq"])
			ev := curItem.fields["event"]
			if seq != "" && ev != "" {
				s.terminalSeqs[seq] = EventType(ev)
			}
		case "tokenizer":
			if subSection == "rules" {
				r := tokRule{
					name:      curItem.fields["name"],
					matchType: curItem.fields["match"],
					prefix:    resolveEscapes(curItem.fields["prefix"]),
					scanUntil: curItem.fields["scan_until"],
					scanClass: curItem.fields["scan_class"],
					emit:      EventType(curItem.fields["emit"]),
				}
				if r.name != "" {
					s.tokenizerRules = append(s.tokenizerRules, r)
				}
			}
		}
		curItem = nil
	}

	for _, rawLine := range lines {
		line := rawLine
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "$schema") {
			continue
		}

		indent := indentLevel(line)

		// Top-level: "input:"
		if trimmed == "input:" {
			inInput = true
			continue
		}
		if !inInput {
			continue
		}

		// Second level sections (indent=2)
		if indent == 2 && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			commitItem()
			section = strings.TrimSuffix(trimmed, ":")
			subSection = ""
			continue
		}

		// Third level sections (indent=4) — used by tokenizer: rules:
		if indent == 4 && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			commitItem()
			subSection = strings.TrimSuffix(trimmed, ":")
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			// Check for list item start "- name: val" or "- name:"
			if strings.HasPrefix(trimmed, "- ") {
				commitItem()
				curItem = &listItem{fields: map[string]string{}}
				rest := strings.TrimPrefix(trimmed, "- ")
				kv := strings.SplitN(rest, ":", 2)
				if len(kv) == 2 {
					curItem.fields[strings.TrimSpace(kv[0])] = stripYAMLValue(kv[1])
				}
			}
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := stripYAMLValue(parts[1])

		// List item field continuation
		if curItem != nil && indent >= 6 {
			curItem.fields[key] = val
			continue
		}

		switch section {
		case "key_aliases":
			if key != "" && val != "" {
				s.keyAliases[key] = val
			}
		case "ctrl_pattern":
			switch key {
			case "prefix":
				s.ctrlPrefix = val
			case "base_char":
				if len(val) > 0 {
					s.ctrlBaseChar = val[0]
				}
			case "base_code":
				if n, err := strconv.Atoi(val); err == nil {
					s.ctrlBaseCode = n
				}
			}
		case "mouse":
			switch key {
			case "prefix":
				s.mousePrefix = val
			case "press_suffix":
				s.mousePressS = val
			case "release_suffix":
				s.mouseReleaseS = val
			case "btn_button_mask":
				if n, err := strconv.Atoi(val); err == nil {
					s.mouseBtnButtonMask = n
				}
			case "btn_shift_mask":
				if n, err := strconv.Atoi(val); err == nil {
					s.mouseBtnShiftMask = n
				}
			case "btn_meta_mask":
				if n, err := strconv.Atoi(val); err == nil {
					s.mouseBtnMetaMask = n
				}
			case "btn_ctrl_mask":
				if n, err := strconv.Atoi(val); err == nil {
					s.mouseBtnCtrlMask = n
				}
			case "btn_motion_flag":
				if n, err := strconv.Atoi(val); err == nil {
					s.mouseBtnMotionFlag = n
				}
			case "btn_scroll_flag":
				if n, err := strconv.Atoi(val); err == nil {
					s.mouseBtnScrollFlag = n
				}
			}
		}
	}
	commitItem()

	// If no rules were parsed (malformed file), fall back to defaults.
	if len(s.tokenizerRules) == 0 {
		s.tokenizerRules = DefaultSpec().tokenizerRules
	}

	return s, nil
}

// ResolveKeyAlias converts a named alias like "<esc>" or "<c-c>" to its terminal sequence.
// Plain chars and unrecognised aliases pass through unchanged.
func (s *Spec) ResolveKeyAlias(name string) string {
	if !strings.HasPrefix(name, "<") || !strings.HasSuffix(name, ">") {
		return name
	}
	inner := strings.ToLower(name[1 : len(name)-1])
	if seq, ok := s.keyAliases[inner]; ok {
		return seq
	}
	// ctrl pattern: c-X
	if strings.HasPrefix(inner, s.ctrlPrefix) && len(inner) == len(s.ctrlPrefix)+1 {
		ch := rune(inner[len(s.ctrlPrefix)])
		if ch >= rune(s.ctrlBaseChar) && ch <= rune(s.ctrlBaseChar)+25 {
			return string(rune(s.ctrlBaseCode) + ch - rune(s.ctrlBaseChar))
		}
	}
	return name
}

// Tokenize splits a raw byte buffer into discrete input tokens.
func (s *Spec) Tokenize(raw string) []string {
	var tokens []string
	i := 0
	for i < len(raw) {
		matched := false
		for _, rule := range s.tokenizerRules {
			switch rule.matchType {
			case "starts_with":
				if !strings.HasPrefix(raw[i:], rule.prefix) {
					continue
				}
				start := i + len(rule.prefix)
				switch {
				case rule.scanUntil != "":
					// Scan forward for any terminator byte.
					end := -1
					for j := start; j < len(raw); j++ {
						if strings.ContainsRune(rule.scanUntil, rune(raw[j])) {
							end = j
							break
						}
					}
					if end == -1 {
						// No terminator found — emit just the prefix as fallback key.
						tokens = append(tokens, raw[i:i+len(rule.prefix)])
						i += len(rule.prefix)
					} else {
						tokens = append(tokens, raw[i:end+1])
						i = end + 1
					}
					matched = true
				case rule.scanClass == "alpha_tilde":
					end := -1
					for j := start; j < len(raw); j++ {
						c := raw[j]
						if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '~' {
							end = j
							break
						}
					}
					if end == -1 {
						// Incomplete CSI — emit prefix.
						tokens = append(tokens, raw[i:i+len(rule.prefix)])
						i += len(rule.prefix)
					} else {
						tokens = append(tokens, raw[i:end+1])
						i = end + 1
					}
					matched = true
				default:
					// prefix-only match: emit prefix as token.
					tokens = append(tokens, raw[i:i+len(rule.prefix)])
					i += len(rule.prefix)
					matched = true
				}
			case "utf8_lead":
				// Consume a complete UTF-8 codepoint so multi-byte chars (ö, €, 日…)
				// arrive as one token instead of several separate bytes.
				if raw[i] >= 0x80 {
					if r, size := utf8.DecodeRuneInString(raw[i:]); r != utf8.RuneError && size > 1 {
						tokens = append(tokens, raw[i:i+size])
						i += size
						matched = true
					}
				}
			case "any":
				tokens = append(tokens, string(raw[i]))
				i++
				matched = true
			}
			if matched {
				break
			}
		}
		if !matched {
			// Safety: consume one byte.
			tokens = append(tokens, string(raw[i]))
			i++
		}
	}
	return tokens
}

// Classify maps a raw token string to an Event.
func (s *Spec) Classify(tok string) Event {
	if et, ok := s.terminalSeqs[tok]; ok {
		return Event{Type: et, Token: tok}
	}
	if strings.HasPrefix(tok, s.mousePrefix) {
		if m, ok := s.ParseMouse(tok); ok {
			return Event{Type: EventMouse, Token: tok, Mouse: m}
		}
	}
	return Event{Type: EventKey, Token: tok}
}

// ParseMouse decodes an SGR 1006 mouse event token.
func (s *Spec) ParseMouse(tok string) (MouseEvent, bool) {
	if !strings.HasPrefix(tok, s.mousePrefix) {
		return MouseEvent{}, false
	}
	body := tok[len(s.mousePrefix):]
	var release bool
	switch {
	case strings.HasSuffix(body, s.mousePressS):
		body = body[:len(body)-len(s.mousePressS)]
	case strings.HasSuffix(body, s.mouseReleaseS):
		release = true
		body = body[:len(body)-len(s.mouseReleaseS)]
	default:
		return MouseEvent{}, false
	}
	parts := strings.SplitN(body, ";", 3)
	if len(parts) != 3 {
		return MouseEvent{}, false
	}
	btn, err1 := strconv.Atoi(parts[0])
	col, err2 := strconv.Atoi(parts[1])
	row, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return MouseEvent{}, false
	}
	m := MouseEvent{
		Btn:     btn,
		Col:     col,
		Row:     row,
		Release: release,
		Button:  btn & s.mouseBtnButtonMask,
		Shift:   btn&s.mouseBtnShiftMask != 0,
		Meta:    btn&s.mouseBtnMetaMask != 0,
		Ctrl:    btn&s.mouseBtnCtrlMask != 0,
		Motion:  btn&s.mouseBtnMotionFlag != 0,
		Scroll:  btn&s.mouseBtnScrollFlag != 0,
	}
	return m, true
}

// IsCovered reports whether a token is handled by any tokenizer rule and is a
// recognised event (not EventUnknown). Used by the input tester TUI.
func (s *Spec) IsCovered(tok string) bool {
	ev := s.Classify(tok)
	return ev.Type != EventUnknown
}

// canonicalNames maps alias name → preferred display name.
// Used to pick the most readable name when multiple aliases share a sequence.
var canonicalNames = map[string]string{
	"esc": "Esc", "bs": "Backspace", "cr": "Enter", "space": "Space",
	"tab": "Tab", "del": "Delete", "up": "Up", "down": "Down",
	"left": "Left", "right": "Right", "pageup": "Page Up", "pagedown": "Page Down",
	"home": "Home", "end": "End",
	"f1": "F1", "f2": "F2", "f3": "F3", "f4": "F4",
	"f5": "F5", "f6": "F6", "f7": "F7", "f8": "F8",
	"f9": "F9", "f10": "F10", "f11": "F11", "f12": "F12",
}

// preferredAlias is the lookup order for picking the canonical display name.
// Earlier entries win when multiple aliases resolve to the same sequence.
var preferredAlias = []string{
	"esc", "bs", "cr", "space", "tab", "del",
	"up", "down", "left", "right",
	"pageup", "pagedown", "home", "end",
	"f1", "f2", "f3", "f4", "f5", "f6",
	"f7", "f8", "f9", "f10", "f11", "f12",
}

// seqNames builds a reverse map from terminal sequence → display name.
func (s *Spec) seqNames() map[string]string {
	m := make(map[string]string, len(s.keyAliases))
	// Insert in reverse-preferred order so preferred entries overwrite.
	for alias, seq := range s.keyAliases {
		if _, exists := m[seq]; !exists {
			if dn, ok := canonicalNames[alias]; ok {
				m[seq] = dn
			} else {
				m[seq] = strings.ToUpper(alias[:1]) + alias[1:]
			}
		}
	}
	// Overwrite with preferred ordering (earlier = wins).
	for i := len(preferredAlias) - 1; i >= 0; i-- {
		alias := preferredAlias[i]
		if seq, ok := s.keyAliases[alias]; ok {
			if dn, ok2 := canonicalNames[alias]; ok2 {
				m[seq] = dn
			}
		}
	}
	return m
}

// KeyName returns a human-readable name for a raw key token.
// Examples: "\x1b[A"→"Up", "\x0d"→"Enter", "\x09"→"Tab", "a"→"a", "\x03"→"Ctrl-C", "ö"→"ö".
func (s *Spec) KeyName(seq string) string {
	// Named aliases take priority — covers Tab, Enter, Esc, Backspace, arrows, etc.
	// This must run before the ctrl heuristic so Tab (\x09) shows as "Tab" not "Ctrl-I".
	names := s.seqNames()
	if name, ok := names[seq]; ok {
		return name
	}
	if len(seq) == 1 {
		c := seq[0]
		// Printable ASCII.
		if c >= 0x20 && c < 0x7f {
			return string(c)
		}
		// Backspace (in case it wasn't aliased).
		if c == 0x7f {
			return "Backspace"
		}
		// Ctrl chord: \x01–\x1a → Ctrl-A … Ctrl-Z.
		if c >= 1 && c <= 26 {
			return "Ctrl-" + string(rune('A'+c-1))
		}
	}
	// Multi-byte UTF-8 printable character (e.g. ö, €, 日).
	if r, size := utf8.DecodeRuneInString(seq); size > 1 && size == len(seq) && r != utf8.RuneError {
		return string(r)
	}
	// Fallback: hex representation.
	var b strings.Builder
	for i := 0; i < len(seq); i++ {
		ch := seq[i]
		if ch >= 0x20 && ch < 0x7f {
			b.WriteByte(ch)
		} else {
			b.WriteString(`\x`)
			b.WriteByte("0123456789abcdef"[ch>>4])
			b.WriteByte("0123456789abcdef"[ch&0xf])
		}
	}
	return b.String()
}

// MouseName returns a human-readable name for a mouse event.
// Examples: "Scroll Up", "Press Left", "Drag Right", "Release Middle".
func MouseName(m MouseEvent) string {
	btnNames := [3]string{"Left", "Middle", "Right"}
	btn := "Left"
	if m.Button >= 0 && m.Button < 3 {
		btn = btnNames[m.Button]
	}
	switch {
	case m.IsScroll() && m.ScrollDir() < 0:
		return "Scroll Up"
	case m.IsScroll():
		return "Scroll Down"
	case m.IsMove():
		return "Move"
	case m.IsDrag():
		return "Drag " + btn
	case m.Release:
		return "Release " + btn
	default:
		return "Press " + btn
	}
}

// EventName returns a concise human-readable name for any event.
func (s *Spec) EventName(ev Event) string {
	switch ev.Type {
	case EventMouse:
		return MouseName(ev.Mouse)
	case EventFocus:
		return "Focus Gain"
	case EventDefocus:
		return "Focus Loss"
	case EventResize:
		return "Resize"
	case EventQuit:
		return "Quit Signal"
	case EventKey:
		return s.KeyName(ev.Token)
	default:
		return "Unknown"
	}
}

// MouseEnableButton returns the sequence to enable button tracking.
func (s *Spec) MouseEnableButton() string { return "\x1b[?1002h\x1b[?1006h" }

// MouseEnableMotion returns the sequence to enable full motion tracking.
func (s *Spec) MouseEnableMotion() string { return "\x1b[?1003h\x1b[?1006h" }

// MouseDisableButton returns the sequence to disable button tracking.
func (s *Spec) MouseDisableButton() string { return "\x1b[?1002l\x1b[?1006l" }

// MouseDisableMotion returns the sequence to disable full motion tracking.
func (s *Spec) MouseDisableMotion() string { return "\x1b[?1003l\x1b[?1006l" }
