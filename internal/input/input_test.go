package input_test

import (
	"os"
	"testing"

	"ubunatic.com/cati/internal/input"
)

func TestDefaultSpec(t *testing.T) {
	s := input.DefaultSpec()
	if s == nil {
		t.Fatal("DefaultSpec returned nil")
	}
}

func TestResolveKeyAlias(t *testing.T) {
	s := input.DefaultSpec()
	cases := []struct{ alias, want string }{
		{"<esc>", "\x1b"},
		{"<ESC>", "\x1b"},
		{"<cr>", "\x0d"},
		{"<enter>", "\x0d"},
		{"<space>", " "},
		{"<tab>", "\t"},
		{"<bs>", "\x7f"},
		{"<up>", "\x1b[A"},
		{"<down>", "\x1b[B"},
		{"<right>", "\x1b[C"},
		{"<left>", "\x1b[D"},
		{"<pgup>", "\x1b[5~"},
		{"<pgdn>", "\x1b[6~"},
		{"<home>", "\x1b[H"},
		{"<end>", "\x1b[F"},
		{"<del>", "\x1b[3~"},
		{"<c-c>", "\x03"},
		{"<c-a>", "\x01"},
		{"<c-z>", "\x1a"},
		{"q", "q"},              // plain char passes through
		{"<unknown>", "<unknown>"}, // unknown alias passes through
	}
	for _, tc := range cases {
		got := s.ResolveKeyAlias(tc.alias)
		if got != tc.want {
			t.Errorf("ResolveKeyAlias(%q) = %q, want %q", tc.alias, got, tc.want)
		}
	}
}

func TestTokenizeBasic(t *testing.T) {
	s := input.DefaultSpec()
	cases := []struct {
		raw  string
		want []string
	}{
		{"q", []string{"q"}},
		{"ab", []string{"a", "b"}},
		{"\x1b", []string{"\x1b"}},
		{"\x1b[A", []string{"\x1b[A"}},
		{"\x1b[B", []string{"\x1b[B"}},
		{"\x1b[5~", []string{"\x1b[5~"}},
		{"\x1b[<0;42;15M", []string{"\x1b[<0;42;15M"}},
		{"\x1b[<64;10;5m", []string{"\x1b[<64;10;5m"}},
		{"\x1b[I", []string{"\x1b[I"}},
		{"\x1b[O", []string{"\x1b[O"}},
	}
	for _, tc := range cases {
		got := s.Tokenize(tc.raw)
		if len(got) != len(tc.want) {
			t.Errorf("Tokenize(%q) = %v (len=%d), want %v (len=%d)", tc.raw, got, len(got), tc.want, len(tc.want))
			continue
		}
		for i, g := range got {
			if g != tc.want[i] {
				t.Errorf("Tokenize(%q)[%d] = %q, want %q", tc.raw, i, g, tc.want[i])
			}
		}
	}
}

func TestTokenizeMixed(t *testing.T) {
	s := input.DefaultSpec()
	// Multiple events packed into one read buffer.
	raw := "q\x1b[A\x1b[<0;10;5Mab"
	got := s.Tokenize(raw)
	want := []string{"q", "\x1b[A", "\x1b[<0;10;5M", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("Tokenize mixed: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestParseMouse(t *testing.T) {
	s := input.DefaultSpec()
	cases := []struct {
		tok     string
		wantOK  bool
		btn     int
		col     int
		row     int
		release bool
		scroll  bool
		motion  bool
		button  int
	}{
		{"\x1b[<0;10;5M", true, 0, 10, 5, false, false, false, 0},    // left press
		{"\x1b[<0;10;5m", true, 0, 10, 5, true, false, false, 0},     // left release
		{"\x1b[<64;10;5M", true, 64, 10, 5, false, true, false, 0},   // scroll up
		{"\x1b[<65;10;5M", true, 65, 10, 5, false, true, false, 1},   // scroll down
		{"\x1b[<32;10;5M", true, 32, 10, 5, false, false, true, 0},   // left drag  (button=0 + motion)
		{"\x1b[<35;10;5M", true, 35, 10, 5, false, false, true, 3},   // pure move  (button=3 + motion)
		{"\x1b[<1;10;5M", true, 1, 10, 5, false, false, false, 1},    // middle press
		{"q", false, 0, 0, 0, false, false, false, 0},                 // not mouse
	}
	for _, tc := range cases {
		m, ok := s.ParseMouse(tc.tok)
		if ok != tc.wantOK {
			t.Errorf("ParseMouse(%q) ok=%v, want %v", tc.tok, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if m.Btn != tc.btn || m.Col != tc.col || m.Row != tc.row {
			t.Errorf("ParseMouse(%q): btn=%d col=%d row=%d, want btn=%d col=%d row=%d",
				tc.tok, m.Btn, m.Col, m.Row, tc.btn, tc.col, tc.row)
		}
		if m.Release != tc.release {
			t.Errorf("ParseMouse(%q): Release=%v, want %v", tc.tok, m.Release, tc.release)
		}
		if m.Scroll != tc.scroll {
			t.Errorf("ParseMouse(%q): Scroll=%v, want %v", tc.tok, m.Scroll, tc.scroll)
		}
		if m.Motion != tc.motion {
			t.Errorf("ParseMouse(%q): Motion=%v, want %v", tc.tok, m.Motion, tc.motion)
		}
		if m.Button != tc.button {
			t.Errorf("ParseMouse(%q): Button=%d, want %d", tc.tok, m.Button, tc.button)
		}
	}
}

func TestClassify(t *testing.T) {
	s := input.DefaultSpec()
	cases := []struct {
		tok  string
		want input.EventType
	}{
		{"q", input.EventKey},
		{"\x1b[A", input.EventKey},
		{"\x1b", input.EventKey},
		{"\x1b[<0;10;5M", input.EventMouse},
		{"\x1b[<64;1;1M", input.EventMouse},
		{"\x1b[I", input.EventFocus},
		{"\x1b[O", input.EventDefocus},
	}
	for _, tc := range cases {
		ev := s.Classify(tc.tok)
		if ev.Type != tc.want {
			t.Errorf("Classify(%q) = %v, want %v", tc.tok, ev.Type, tc.want)
		}
	}
}

func TestKeyNameAliasBeforeCtrl(t *testing.T) {
	s := input.DefaultSpec()
	// Tab is \x09 — falls in ctrl range (1–26) but must show as "Tab" not "Ctrl-I".
	if got := s.KeyName("\x09"); got != "Tab" {
		t.Errorf("KeyName(tab) = %q, want \"Tab\"", got)
	}
	// Enter is \x0d — must show as "Enter" not "Ctrl-M".
	if got := s.KeyName("\x0d"); got != "Enter" {
		t.Errorf("KeyName(enter) = %q, want \"Enter\"", got)
	}
	// Esc is \x1b — not in ctrl range but must show as "Esc".
	if got := s.KeyName("\x1b"); got != "Esc" {
		t.Errorf("KeyName(esc) = %q, want \"Esc\"", got)
	}
	// Ctrl-C has no alias entry beyond <c-c> — must fall through to ctrl heuristic.
	if got := s.KeyName("\x03"); got != "Ctrl-C" {
		t.Errorf("KeyName(ctrl-c) = %q, want \"Ctrl-C\"", got)
	}
}

func TestKeyNameUTF8(t *testing.T) {
	s := input.DefaultSpec()
	cases := []struct{ seq, want string }{
		{"\xc3\xb6", "ö"},   // ö U+00F6
		{"\xe2\x82\xac", "€"}, // € U+20AC
		{"a", "a"},            // plain ASCII unchanged
	}
	for _, tc := range cases {
		if got := s.KeyName(tc.seq); got != tc.want {
			t.Errorf("KeyName(%q) = %q, want %q", tc.seq, got, tc.want)
		}
	}
}

func TestTokenizeUTF8(t *testing.T) {
	s := input.DefaultSpec()
	// ö is \xc3\xb6 — must arrive as one token, not two.
	tokens := s.Tokenize("\xc3\xb6")
	if len(tokens) != 1 {
		t.Fatalf("Tokenize(ö) got %d tokens, want 1: %q", len(tokens), tokens)
	}
	if tokens[0] != "\xc3\xb6" {
		t.Errorf("Tokenize(ö)[0] = %q, want %q", tokens[0], "\xc3\xb6")
	}
	// Mixed: "aö" → two tokens "a" and "ö".
	tokens = s.Tokenize("a\xc3\xb6")
	if len(tokens) != 2 || tokens[0] != "a" || tokens[1] != "\xc3\xb6" {
		t.Errorf("Tokenize(aö) = %q, want [\"a\", \"ö\"]", tokens)
	}
}

func TestMouseMoveVsDrag(t *testing.T) {
	s := input.DefaultSpec()
	// btn=35 (0x23): motion flag (0x20) + button bits 0b11 (3) = pure move
	move, ok := s.ParseMouse("\x1b[<35;20;10M")
	if !ok {
		t.Fatal("parse move failed")
	}
	if !move.IsMove() {
		t.Error("IsMove() = false for btn=35, want true")
	}
	if move.IsDrag() {
		t.Error("IsDrag() = true for btn=35 (pure move), want false")
	}
	// btn=32 (0x20): motion flag + button=0 (left held) = drag
	drag, ok := s.ParseMouse("\x1b[<32;20;10M")
	if !ok {
		t.Fatal("parse drag failed")
	}
	if drag.IsMove() {
		t.Error("IsMove() = true for btn=32 (left drag), want false")
	}
	if !drag.IsDrag() {
		t.Error("IsDrag() = false for btn=32, want true")
	}
	// Name check
	if n := input.MouseName(move); n != "Move" {
		t.Errorf("MouseName(move) = %q, want \"Move\"", n)
	}
	if n := input.MouseName(drag); n != "Drag Left" {
		t.Errorf("MouseName(drag) = %q, want \"Drag Left\"", n)
	}
}

func TestScrollDir(t *testing.T) {
	s := input.DefaultSpec()
	up, okUp := s.ParseMouse("\x1b[<64;1;1M")
	dn, okDn := s.ParseMouse("\x1b[<65;1;1M")
	if !okUp || !okDn {
		t.Fatal("scroll parse failed")
	}
	if up.ScrollDir() != -1 {
		t.Errorf("scroll up dir = %d, want -1", up.ScrollDir())
	}
	if dn.ScrollDir() != 1 {
		t.Errorf("scroll down dir = %d, want 1", dn.ScrollDir())
	}
}

func TestLoadSpec(t *testing.T) {
	fsys := os.DirFS("../../spec")
	s, err := input.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s == nil {
		t.Fatal("Load returned nil")
	}
	// Verify a few key aliases survive the parse round-trip.
	cases := []struct{ alias, want string }{
		{"<esc>", "\x1b"},
		{"<up>", "\x1b[A"},
		{"<c-c>", "\x03"},
	}
	for _, tc := range cases {
		got := s.ResolveKeyAlias(tc.alias)
		if got != tc.want {
			t.Errorf("loaded spec ResolveKeyAlias(%q) = %q, want %q", tc.alias, got, tc.want)
		}
	}
}
