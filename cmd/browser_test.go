package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBrowser_DrawBottomMenu(t *testing.T) {
	var buf bytes.Buffer
	labels := loadLabels()
	rows := loadViewButtonRows()

	// Test standard grid buttons (5 buttons: prev, next, settings, about, quit)
	btns := drawBottomMenu(&buf, 24, "grid", "", nil, labels, rows, nil, nil)
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
	btnsAbout := drawBottomMenu(&buf, 24, "about", "", nil, labels, rows, nil, nil)
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
	btnsSettings := drawBottomMenu(&buf, 24, "settings", "", nil, labels, rows, nil, nil)
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

// TestButtonSchemaActionsHandled verifies every action name in the buttons schema
// enum appears as a string literal in the Go action handler source files.
// This catches schema additions that have no corresponding Go handler.
func TestButtonSchemaActionsHandled(t *testing.T) {
	schemaData, err := os.ReadFile("../spec/schemas/buttons.schema.json")
	if err != nil {
		t.Skip("buttons.schema.json not available:", err)
	}

	var schema struct {
		Definitions struct {
			Button struct {
				Properties struct {
					Action struct {
						Enum []string `json:"enum"`
					} `json:"action"`
				} `json:"properties"`
			} `json:"button"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	actions := schema.Definitions.Button.Properties.Action.Enum
	if len(actions) == 0 {
		t.Fatal("schema action enum is empty — schema may have changed structure")
	}

	var src strings.Builder
	for _, f := range []string{"browser.go", "interactive.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src.Write(b)
	}
	source := src.String()

	for _, action := range actions {
		if !strings.Contains(source, `"`+action+`"`) {
			t.Errorf("action %q declared in schema enum but not found as string literal in Go source — add a case handler", action)
		}
	}
}
