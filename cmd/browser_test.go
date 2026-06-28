package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrowser_DrawBottomMenu(t *testing.T) {
	var buf bytes.Buffer
	style := loadStyle()
	labels := loadLabels()
	for k, v := range loadButtons(style.BtnLeftCap, style.BtnRightCap) {
		labels[k] = v
	}
	rows := loadViewButtonRows()
	btnActions := loadButtonActions()

	cases := []struct {
		view    string
		actions []string
	}{
		// browser: prev next back | settings mode about | quit
		{"grid", []string{"nav_prev", "nav_next", "go_back", "open_settings", "toggle_mode", "open_about", "quit"}},
		// about: back website | quit
		{"about", []string{"go_back", "open_website", "quit"}},
		// settings: save cancel | quit
		{"settings", []string{"save_settings", "cancel_settings", "quit"}},
	}
	for _, tc := range cases {
		btns := drawBottomMenu(&buf, 24, 80, tc.view, "", style, labels, rows, nil, btnActions, nil)
		if len(btns) != len(tc.actions) {
			t.Errorf("view %q: expected %d buttons, got %d", tc.view, len(tc.actions), len(btns))
			continue
		}
		for i, b := range btns {
			if b.action != tc.actions[i] {
				t.Errorf("view %q btn[%d] action = %q, want %q", tc.view, i, b.action, tc.actions[i])
			}
		}
	}
}

func TestTruncateANSIPreservesEscapesAndWidth(t *testing.T) {
	got := truncateANSI("\x1b[31mabcdef\x1b[m", 3)
	if got != "\x1b[31mabc" {
		t.Fatalf("truncateANSI = %q, want red abc prefix", got)
	}
}

func TestFitMenuItemsCollapsesLowPriorityBeforeRemoving(t *testing.T) {
	items := []menuLayoutItem{
		{literal: "", label: "[High]", fullLabel: "[High]", compactLabel: "[H]", prio: 100, visible: true},
		{literal: " ", label: "[Low]", fullLabel: "[Low]", compactLabel: "[L]", prio: 10, visible: true},
		{literal: " ", label: "[Mid]", fullLabel: "[Mid]", compactLabel: "[M]", prio: 50, visible: true},
	}
	fitMenuItems(items, 14)
	if items[0].collapsed {
		t.Fatal("high-priority item collapsed before lower-priority items")
	}
	if !items[1].collapsed {
		t.Fatal("low-priority item was not collapsed first")
	}
	if !items[2].visible {
		t.Fatal("mid-priority item removed before all collapse options were used")
	}
}

func TestFitMenuItemsRemovesLowPriorityAfterCollapse(t *testing.T) {
	items := []menuLayoutItem{
		{literal: "", label: "[High]", fullLabel: "[High]", compactLabel: "[H]", prio: 100, visible: true},
		{literal: " ", label: "[Low]", fullLabel: "[Low]", compactLabel: "[L]", prio: 10, visible: true},
		{literal: " ", label: "[Mid]", fullLabel: "[Mid]", compactLabel: "[M]", prio: 50, visible: true},
	}
	fitMenuItems(items, 7)
	if items[0].visible != true {
		t.Fatal("high-priority item removed")
	}
	if items[1].visible != false {
		t.Fatal("low-priority item should be removed first after collapse")
	}
}

func TestBrowser_ParseYaml(t *testing.T) {
	view, err := parseYamlView("about.yaml")
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
