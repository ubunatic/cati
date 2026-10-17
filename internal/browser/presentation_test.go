package browser

import "testing"

func TestTruncateANSIPreservesEscapesAndWidth(t *testing.T) {
	got := TruncateANSI("\x1b[31mabcdef\x1b[m", 3)
	if got != "\x1b[31mabc" {
		t.Fatalf("truncateANSI = %q, want red abc prefix", got)
	}
}

func TestEllipsizeRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"fits", "cat.png", 8, "cat.png"},
		{"truncate with ellipsis", "very_long_name.png", 10, "very_lo..."},
		{"tight budget", "abcdef", 3, "abc"},
		{"empty budget", "abcdef", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := EllipsizeRunes(tc.in, tc.max); got != tc.want {
				t.Fatalf("EllipsizeRunes(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestTemplateRestoresBaseStyleAndResolvesLiterals(t *testing.T) {
	vars := map[string]string{"name": "猫.png"}
	got := RenderTemplate("{name | bold} {'ok'} {missing}", vars, "BASE")
	want := "\x1b[1m猫.png\x1b[mBASE ok missing"
	if got != want {
		t.Fatalf("template = %q, want %q", got, want)
	}
	if got := TemplateWidth("{name | bold} {'ok'} {missing}", vars); got != 16 {
		t.Fatalf("template rune width = %d, want 16", got)
	}
}

func TestColourTokensAndFallbacks(t *testing.T) {
	for _, tc := range []struct{ token, fg, bg string }{
		{"#f80", "\x1b[38;2;255;136;0m", "\x1b[48;2;255;136;0m"},
		{"RED", "\x1b[38;2;255;0;0m", "\x1b[48;2;255;0;0m"},
		{"dark", "\x1b[90m", "\x1b[100m"},
		{"light", "\x1b[97m", "\x1b[107m"},
		{"invalid", "fallback", "fallback"},
		{"", "fallback", "fallback"},
	} {
		t.Run(tc.token, func(t *testing.T) {
			if fg, bg := StyleFG(tc.token, "fallback"), StyleBG(tc.token, "fallback"); fg != tc.fg || bg != tc.bg {
				t.Fatalf("colour %q => %q, %q; want %q, %q", tc.token, fg, bg, tc.fg, tc.bg)
			}
		})
	}
	if _, ok := ParseColor("#ggg"); ok {
		t.Fatal("accepted invalid hex colour")
	}
}
