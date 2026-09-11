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

func TestModesFilterFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantModes  []string
		avoidModes []string
	}{
		{
			name:       "default excludes legacy",
			args:       []string{"-l"},
			wantModes:  []string{"full", "half", "quad", "quad+", "bars", "bars+", "six", "2x3", "3x3", "all", "all+", "z", "z+"},
			avoidModes: []string{"spark", "spark+quad", "six+half", "spark+six", "half/split"},
		},
		{
			name:       "all modes includes legacy",
			args:       []string{"-l", "-a"},
			wantModes:  []string{"full", "half", "quad", "quad+", "bars", "bars+", "six", "2x3", "3x3", "all", "all+", "z", "z+", "half/split", "spark", "spark+quad", "six+half", "spark+six"},
		},
		{
			name:       "composed only",
			args:       []string{"-l", "-c"},
			wantModes:  []string{"full", "half", "quad", "quad+", "bars", "bars+", "six", "2x3", "3x3", "all", "all+", "z", "z+"},
			avoidModes: []string{"spark", "spark+quad", "six+half", "spark+six", "half/split"},
		},
		{
			name:       "legacy only",
			args:       []string{"-l", "-L"},
			wantModes:  []string{"half/split", "spark", "spark+quad", "six+half", "spark+six"},
			avoidModes: []string{"full", "quad", "bars", "six", "3x3", "all", "z"},
		},
		{
			name:       "exact only",
			args:       []string{"-l", "-c", "--exact"},
			wantModes:  []string{"full", "half", "quad", "bars", "bars+", "six", "2x3"},
			avoidModes: []string{"quad+", "3x3", "all", "all+", "z", "z+"},
		},
		{
			name:       "approx only",
			args:       []string{"-l", "-c", "--approx"},
			wantModes:  []string{"quad+", "3x3", "all", "all+", "z", "z+"},
			avoidModes: []string{"full", "half", "quad", "bars", "bars+", "six", "2x3"},
		},
		{
			name:       "set 6 sextants",
			args:       []string{"-l", "-s", "6"},
			wantModes:  []string{"six", "2x3", "3x3", "all", "all+", "z", "z+"},
			avoidModes: []string{"full", "half", "quad", "quad+", "bars", "bars+"},
		},
		{
			name:       "set 86 morebars",
			args:       []string{"-l", "-s", "86"},
			wantModes:  []string{"bars+", "z"},
			avoidModes: []string{"full", "half", "quad", "bars", "six", "2x3", "3x3", "all", "all+", "z+"},
		},
		{
			name:       "max-geo 2x2",
			args:       []string{"-l", "--max-geo", "2x2"},
			wantModes:  []string{"full", "half", "quad"},
			avoidModes: []string{"bars", "bars+", "six", "2x3", "3x3", "all", "all+", "z", "z+"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := modesCommand()
			cmd.SetOut(&out)
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("modes %v: %v", tt.args, err)
			}
			text := out.String()
			lines := strings.Split(text, "\n")
			hasLine := func(target string) bool {
				for _, line := range lines {
					if line == target {
						return true
					}
				}
				return false
			}
			for _, want := range tt.wantModes {
				if !hasLine(want) {
					t.Errorf("output missing %q in:\n%s", want, text)
				}
			}
			for _, avoid := range tt.avoidModes {
				if hasLine(avoid) {
					t.Errorf("output unexpectedly contained %q in:\n%s", avoid, text)
				}
			}
		})
	}
}

func TestModesListGlyphSets(t *testing.T) {
	var out bytes.Buffer
	cmd := modesCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--sets"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("modes --sets: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Registered glyph sets:") {
		t.Fatalf("glyph sets header missing: %q", text)
	}
	for _, want := range []string{"set 0", "set 1", "set 2", "set 4", "set 6", "set 9", "set 14", "set 15", "set 44", "set 45", "set 86", "set 88"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q in:\n%s", want, text)
		}
	}
}

func TestModesStatsInTitle(t *testing.T) {
	var out bytes.Buffer
	cmd := modesCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--smart", "-w", "12", "half", "spark+six"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("modes --smart: %v", err)
	}
	text := out.String()
	for _, pattern := range []string{"half (", "w=12", "err=", "+smart (", "spark+six ("} {
		if !strings.Contains(text, pattern) {
			t.Errorf("expected stats pattern %q in output:\n%s", pattern, text)
		}
	}
}

func TestModesSorting(t *testing.T) {
	t.Run("list sort by name", func(t *testing.T) {
		var out bytes.Buffer
		cmd := modesCommand()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"-l", "--sort", "name", "six", "half", "quad"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("modes -l --sort name: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		// Filter out header
		var modes []string
		for _, l := range lines {
			if l != "Available render modes:" {
				modes = append(modes, l)
			}
		}
		expected := []string{"half", "quad", "six"}
		if len(modes) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, modes)
		}
		for i := range expected {
			if modes[i] != expected[i] {
				t.Errorf("at index %d: expected %s, got %s", i, expected[i], modes[i])
			}
		}
	})

	t.Run("list sort by -name", func(t *testing.T) {
		var out bytes.Buffer
		cmd := modesCommand()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"-l", "--sort", "-name", "six", "half", "quad"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("modes -l --sort -name: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		var modes []string
		for _, l := range lines {
			if l != "Available render modes:" {
				modes = append(modes, l)
			}
		}
		expected := []string{"six", "quad", "half"}
		if len(modes) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, modes)
		}
		for i := range expected {
			if modes[i] != expected[i] {
				t.Errorf("at index %d: expected %s, got %s", i, expected[i], modes[i])
			}
		}
	})

	t.Run("demo sort by psnr and by-psnr flag", func(t *testing.T) {
		var out1, out2 bytes.Buffer
		cmd1 := modesCommand()
		cmd1.SetOut(&out1)
		cmd1.SetArgs([]string{"-w", "12", "--sort", "psnr", "half", "quad", "six"})
		if err := cmd1.Execute(); err != nil {
			t.Fatalf("modes --sort psnr: %v", err)
		}

		cmd2 := modesCommand()
		cmd2.SetOut(&out2)
		cmd2.SetArgs([]string{"-w", "12", "--by-psnr", "half", "quad", "six"})
		if err := cmd2.Execute(); err != nil {
			t.Fatalf("modes --by-psnr: %v", err)
		}

		if out1.String() != out2.String() {
			t.Errorf("--sort psnr and --by-psnr outputs differ:\n%s\nvs\n%s", out1.String(), out2.String())
		}
	})

	t.Run("demo sort by efficiency", func(t *testing.T) {
		var out bytes.Buffer
		cmd := modesCommand()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"-w", "12", "--sort", "eff", "half", "quad", "six"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("modes --sort eff: %v", err)
		}
		text := out.String()
		for _, want := range []string{"half (", "quad (", "six ("} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q in:\n%s", want, text)
			}
		}
	})

	t.Run("invalid sort flag", func(t *testing.T) {
		var out bytes.Buffer
		cmd := modesCommand()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"-l", "--sort", "invalid_sort_key"})
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected error for invalid sort key, got nil")
		}
	})
}

