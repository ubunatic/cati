package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrowser_DrawBottomMenu(t *testing.T) {
	var buf bytes.Buffer
	labels := loadLabels()
	rows := loadViewButtonRows()

	// Test standard grid buttons (5 buttons: prev, next, settings, about, quit)
	btns := drawBottomMenu(&buf, 24, "grid", "", nil, labels, rows, nil)
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
	btnsAbout := drawBottomMenu(&buf, 24, "about", "", nil, labels, rows, nil)
	if len(btnsAbout) != 2 {
		t.Errorf("expected 2 buttons in about view, got %d", len(btnsAbout))
	}
	expectedAboutActions := []string{"back", "quit"}
	for i, b := range btnsAbout {
		if b.action != expectedAboutActions[i] {
			t.Errorf("btn[%d] action = %q, want %q", i, b.action, expectedAboutActions[i])
		}
	}

	// Test settings page buttons (3 buttons: save, cancel, quit)
	btnsSettings := drawBottomMenu(&buf, 24, "settings", "", nil, labels, rows, nil)
	if len(btnsSettings) != 3 {
		t.Errorf("expected 3 buttons in settings view, got %d", len(btnsSettings))
	}
	expectedSettingsActions := []string{"save", "cancel", "quit"}
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
