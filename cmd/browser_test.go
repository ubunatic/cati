package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrowser_DrawBottomMenu(t *testing.T) {
	var buf bytes.Buffer

	// Test standard grid buttons (5 buttons: prev, next, settings, about, quit)
	btns := drawBottomMenu(&buf, 80, 24, 0, 1, "grid")
	if len(btns) != 5 {
		t.Errorf("expected 5 buttons in grid view, got %d", len(btns))
	}
	expectedActions := []string{"prev", "next", "settings", "about", "quit"}
	for i, b := range btns {
		if b.action != expectedActions[i] {
			t.Errorf("btn[%d] action = %q, want %q", i, b.action, expectedActions[i])
		}
	}

	// Test about page buttons (2 buttons: back, quit)
	btnsAbout := drawBottomMenu(&buf, 80, 24, 0, 0, "about")
	if len(btnsAbout) != 2 {
		t.Errorf("expected 2 buttons in about view, got %d", len(btnsAbout))
	}
	expectedAboutActions := []string{"back", "quit"}
	for i, b := range btnsAbout {
		if b.action != expectedAboutActions[i] {
			t.Errorf("btn[%d] action = %q, want %q", i, b.action, expectedAboutActions[i])
		}
	}

	// Test settings page buttons (4 buttons: inc, dec, save, cancel)
	btnsSettings := drawBottomMenu(&buf, 80, 24, 0, 0, "settings")
	if len(btnsSettings) != 4 {
		t.Errorf("expected 4 buttons in settings view, got %d", len(btnsSettings))
	}
	expectedSettingsActions := []string{"inc", "dec", "save", "cancel"}
	for i, b := range btnsSettings {
		if b.action != expectedSettingsActions[i] {
			t.Errorf("btn[%d] action = %q, want %q", i, b.action, expectedSettingsActions[i])
		}
	}
}

func TestBrowser_ParseYaml(t *testing.T) {
	view, err := parseYamlView("../spec/about.yaml")
	if err != nil {
		t.Fatalf("failed to parse about.yaml: %v", err)
	}
	if view.Type != "view" {
		t.Errorf("expected type 'view', got %q", view.Type)
	}
	if view.Name != "about" {
		t.Errorf("expected name 'about', got %q", view.Name)
	}
	if !strings.Contains(view.Content, "Version: 1.0.0") {
		t.Errorf("expected version text in content, got:\n%s", view.Content)
	}
}
