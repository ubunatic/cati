package cmd

import (
	"bytes"
	"testing"
)

func TestBrowser_DrawBottomMenu(t *testing.T) {
	var buf bytes.Buffer

	// Test standard grid buttons
	btns := drawBottomMenu(&buf, 80, 24, 0, 1, false)
	if len(btns) != 4 {
		t.Errorf("expected 4 buttons in grid view, got %d", len(btns))
	}
	expectedActions := []string{"prev", "next", "about", "quit"}
	for i, b := range btns {
		if b.action != expectedActions[i] {
			t.Errorf("btn[%d] action = %q, want %q", i, b.action, expectedActions[i])
		}
	}

	// Test about page buttons
	btnsAbout := drawBottomMenu(&buf, 80, 24, 0, 0, true)
	if len(btnsAbout) != 2 {
		t.Errorf("expected 2 buttons in about view, got %d", len(btnsAbout))
	}
	expectedAboutActions := []string{"back", "quit"}
	for i, b := range btnsAbout {
		if b.action != expectedAboutActions[i] {
			t.Errorf("btn[%d] action = %q, want %q", i, b.action, expectedAboutActions[i])
		}
	}
}
