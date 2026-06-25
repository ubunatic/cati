package cmd

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"codeberg.org/ubunatic/cati/internal/input"
	spec "codeberg.org/ubunatic/cati/spec"
)

// TestMain chdirs to the project root so testdata/ is reachable.
// Spec files are loaded via the embedded spec.FS and are not affected by CWD.
func TestMain(m *testing.M) {
	if err := os.Chdir(".."); err != nil {
		panic("cannot chdir to project root: " + err.Error())
	}
	os.Exit(m.Run())
}

// TestSpecButtonsLoad verifies the button key-def loader returns a populated map
// with every entry having a non-empty action.
func TestSpecButtonsLoad(t *testing.T) {
	inputSpec, _ := input.Load(fs.FS(spec.FS))
	defs := loadButtonKeyDefs(inputSpec)
	if len(defs) == 0 {
		t.Fatal("loadButtonKeyDefs() returned empty map — spec/buttons.yaml not readable?")
	}
	for name, def := range defs {
		if def.action == "" {
			t.Errorf("button %q has empty action", name)
		}
	}
}

// TestSpecViewsLoad verifies that all expected views are present in both the
// render rows map and the key rows map.
func TestSpecViewsLoad(t *testing.T) {
	expected := []string{"browser", "image_viewer", "video_player", "settings", "about"}

	renderRows := loadViewButtonRows()
	keyRows := loadViewKeyRows()

	for _, view := range expected {
		if renderRows[view] == "" {
			t.Errorf("loadViewButtonRows(): missing or empty entry for view %q", view)
		}
		if keyRows[view] == "" {
			t.Errorf("loadViewKeyRows(): missing or empty entry for view %q", view)
		}
	}
}

// TestSpecHiddenKeysExpanded verifies that loadViewKeyRows() includes hidden_keys
// entries that are absent from loadViewButtonRows().
func TestSpecHiddenKeysExpanded(t *testing.T) {
	render := loadViewButtonRows()
	key := loadViewKeyRows()

	// browser has hidden_keys for nav_up and nav_down
	if !strings.Contains(key["browser"], "nav_up") {
		t.Error("loadViewKeyRows()[browser] missing nav_up from hidden_keys")
	}
	if strings.Contains(render["browser"], "nav_up") {
		t.Error("loadViewButtonRows()[browser] must NOT contain nav_up (it is hidden)")
	}
}

// TestSpecButtonsAllUsed verifies that every button defined in spec/buttons.yaml
// appears in at least one view row or hidden_keys entry in spec/views.yaml.
func TestSpecButtonsAllUsed(t *testing.T) {
	// Collect all button names defined in buttons.yaml.
	buttonNames := collectButtonNames(t)
	if len(buttonNames) == 0 {
		t.Skip("no button names found — spec/buttons.yaml not readable")
	}

	// Collect all button names referenced in any view row or hidden_keys.
	allKeyRows := loadViewKeyRows()
	used := map[string]bool{}
	for _, tpl := range allKeyRows {
		for _, name := range extractViewButtonNames(tpl) {
			used[name] = true
		}
	}

	for _, name := range buttonNames {
		if !used[name] {
			t.Errorf("button %q is defined in spec/buttons.yaml but not referenced in any view row or hidden_keys — add it to spec/views.yaml or remove it", name)
		}
	}
}

// TestSpecNoGoFallback verifies that loadButtons() does not return keys that are
// absent from spec/buttons.yaml (i.e., no hardcoded Go-only fallbacks).
func TestSpecNoGoFallback(t *testing.T) {
	buttonNames := collectButtonNames(t)
	if len(buttonNames) == 0 {
		t.Skip("no button names found — spec/buttons.yaml not readable")
	}
	specSet := map[string]bool{}
	for _, n := range buttonNames {
		specSet[n] = true
	}

	loaded := loadButtons("", "")
	for key := range loaded {
		if !specSet[key] {
			t.Errorf("loadButtons() returned key %q that is not in spec/buttons.yaml — remove Go-only fallback", key)
		}
	}
}

// TestSpecKeyResolve verifies that inputSpec.ResolveKeyAlias maps all documented
// aliases to the expected terminal byte sequences.
func TestSpecKeyResolve(t *testing.T) {
	inputSpec, _ := input.Load(fs.FS(spec.FS))
	cases := []struct{ alias, want string }{
		{"<esc>", "\x1b"},
		{"<bs>", "\x7f"},
		{"<c-c>", "\x03"},
		{"<cr>", "\x0d"},
		{"<space>", " "},
		{"<up>", "\x1b[A"},
		{"<down>", "\x1b[B"},
		{"<right>", "\x1b[C"},
		{"<left>", "\x1b[D"},
		{"<pgup>", "\x1b[5~"},
		{"<pgdn>", "\x1b[6~"},
		{"q", "q"},
		{"+", "+"},
	}
	for _, tc := range cases {
		got := inputSpec.ResolveKeyAlias(tc.alias)
		if got != tc.want {
			t.Errorf("ResolveKeyAlias(%q) = %q, want %q", tc.alias, got, tc.want)
		}
	}
}

// TestSpecViewKeyMaps verifies that buildViewKeyMaps produces non-empty maps for
// all views and that key→action entries are consistent with the spec.
func TestSpecViewKeyMaps(t *testing.T) {
	inputSpec, _ := input.Load(fs.FS(spec.FS))
	defs := loadButtonKeyDefs(inputSpec)
	keyRows := loadViewKeyRows()
	maps := buildViewKeyMaps(keyRows, defs)

	// quit action must be reachable in every view via some key.
	for view, km := range maps {
		hasQuit := false
		for _, action := range km {
			if action == "quit" {
				hasQuit = true
				break
			}
		}
		if !hasQuit {
			t.Errorf("view %q has no key bound to the quit action", view)
		}
	}

	// Space in image_viewer must trigger toggle_pan (not play/pause).
	if action, ok := maps["image_viewer"][" "]; ok {
		if action != "toggle_pan" {
			t.Errorf("image_viewer space key = %q, want toggle_pan", action)
		}
	} else {
		t.Error("image_viewer has no space key binding (expected toggle_pan)")
	}

	// Space in video_player must trigger toggle_play_pause.
	if action, ok := maps["video_player"][" "]; ok {
		if action != "toggle_play_pause" {
			t.Errorf("video_player space key = %q, want toggle_play_pause", action)
		}
	} else {
		t.Error("video_player has no space key binding (expected toggle_play_pause)")
	}
}

// collectButtonNames parses buttons.yaml and returns all button entry names.
func collectButtonNames(t *testing.T) []string {
	t.Helper()
	data, err := fs.ReadFile(spec.FS, "buttons.yaml")
	if err != nil {
		t.Skip("buttons.yaml not readable:", err)
		return nil
	}
	var names []string
	inButtons := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "buttons:" {
			inButtons = true
			continue
		}
		if !inButtons {
			continue
		}
		if len(line) >= 3 && line[0] == ' ' && line[1] == ' ' && line[2] != ' ' && strings.HasSuffix(trimmed, ":") {
			names = append(names, strings.TrimSuffix(trimmed, ":"))
		}
	}
	return names
}
