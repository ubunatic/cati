package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestModesCommandDemo(t *testing.T) {
	var out bytes.Buffer
	if err := runModesDemo(&out, 12, false); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "cati logo | emojig logo") {
		t.Fatalf("demo header missing from %q", text)
	}
	for _, entry := range renderModes {
		if !strings.Contains(text, entry.name) {
			t.Errorf("demo missing mode %q", entry.name)
		}
	}
}

func TestPadANSILine(t *testing.T) {
	line := "\x1b[38;2;1;2;3m██\x1b[0m"
	got := padANSILine(line, 5)
	if width := ansiLineWidth(got); width != 5 {
		t.Fatalf("padded ANSI line width = %d, want 5", width)
	}
	if width := ansiLineWidth(padANSILine("", 5)); width != 5 {
		t.Fatalf("empty padded ANSI line width = %d, want 5", width)
	}
}
