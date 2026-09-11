package cmd

import (
	"bytes"
	"strings"
	"testing"

	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/sextant"
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

func TestModesCommandSmartZeroWidthListsWithoutRendering(t *testing.T) {
	cmd := modesCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--smart", "-w", "0", "half"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("modes --smart -w 0: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Available render modes:\nhalf\n") {
		t.Fatalf("list-only output = %q", text)
	}
	if strings.Contains(text, "cati logo") || strings.Contains(text, "emojig") || strings.Contains(text, "\x1b[") {
		t.Fatalf("list-only output rendered an image: %q", text)
	}
}

func TestModesCommandInfoFiltersAliasesAndListsShapes(t *testing.T) {
	var out bytes.Buffer
	if err := runModesDemoSelected(&out, 8, false, true, []string{"h"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Count(text, "\n  info:") != 1 {
		t.Fatalf("info output has wrong number of descriptions: %q", text)
	}
	if !strings.Contains(text, "  info: Maps each terminal cell") {
		t.Fatalf("description missing: %q", text)
	}
	if !strings.Contains(text, "  shapes: ␠ ▀ ▄ █") {
		t.Fatalf("half glyph inventory missing: %q", text)
	}
	if strings.Contains(text, "sparkline") {
		t.Fatalf("alias selection rendered another mode: %q", text)
	}
}

func TestModesCommandInfoAllModesAndSmartMetadataOnce(t *testing.T) {
	var out bytes.Buffer
	if err := runModesDemoSelected(&out, 8, true, true, nil); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, entry := range renderModes {
		if got := strings.Count(text, "\n  info: "+entry.definition.Description); got != 1 {
			t.Errorf("mode %q has %d info descriptions, want 1", entry.name, got)
		}
	}
	if !strings.Contains(text, "+smart") {
		t.Fatal("smart presentation missing")
	}
	if !strings.Contains(text, "shapes: ") {
		t.Fatal("shape metadata missing")
	}
}

func TestModesCommandInfoUsesCompleteGeneratedSextantInventory(t *testing.T) {
	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		t.Fatal(err)
	}
	entry := renderModes[0]
	for _, candidate := range renderModes {
		if candidate.name == "six" {
			entry = candidate
			break
		}
	}
	shapes := modeGlyphs(entry, modeSpec)
	if len(shapes) < len(sextant.Glyphs()) {
		t.Fatalf("six inventory has %d glyphs, want at least %d", len(shapes), len(sextant.Glyphs()))
	}
	for _, shape := range sextant.Glyphs() {
		found := false
		for _, got := range shapes {
			if got == shape {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("six inventory omits %q", shape)
		}
	}
}

func TestModesCommandRejectsUnknownSelection(t *testing.T) {
	if _, err := selectedRenderModes([]string{"not-a-mode"}); err == nil || !strings.Contains(err.Error(), `unknown render mode "not-a-mode"`) {
		t.Fatalf("unknown mode error = %v", err)
	}
}

func TestModesInfoListsRegistryCompositionsAndPreservesCase(t *testing.T) {
	var out bytes.Buffer
	cmd := modesCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--smart", "-w", "0", "Q", "q", "d1,6,9,44", "--info"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("modes registry info: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "quad+\n") || !strings.Contains(text, "quad\n") || !strings.Contains(text, "sets: [0 1 6 9 44]") || !strings.Contains(text, "geometry: 12x12") {
		t.Fatalf("registry info missing from %q", text)
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
