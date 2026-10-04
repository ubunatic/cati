//go:build catibrowse

package cmd

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	spec "ubunatic.com/cati/spec"
)

func TestBrowser_DrawBottomMenu(t *testing.T) {
	var buf bytes.Buffer
	style, err := loadStyle()
	if err != nil {
		t.Fatal(err)
	}
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

func TestControlsFixtureDrivesSettingsPage(t *testing.T) {
	makeControls := func(entries string) spec.ControlsSpec {
		t.Helper()
		loaded, err := spec.LoadControlsFrom(fstest.MapFS{"controls.yaml": &fstest.MapFile{Data: []byte("controls:\n" + entries)}})
		if err != nil {
			t.Fatal(err)
		}
		return loaded
	}
	render := func(loaded spec.ControlsSpec) string {
		var out bytes.Buffer
		drawSettingsPage(&out, 100, 40, controlsFromSpec(loaded), Settings{}, -1, nil)
		return out.String()
	}
	first := makeControls("  view_mode:\n    type: enum\n    values: [tiles, preview]\n    set: set_view_mode\n    get: get_view_mode\n  preview_height:\n    type: int\n    min: 7\n    max: 90\n    set: set_preview_height\n    get: get_preview_height\n")
	second := makeControls("  preview_height:\n    type: int\n    min: 7\n    max: 90\n    set: set_preview_height\n    get: get_preview_height\n")
	reordered := makeControls("  preview_height:\n    type: int\n    min: 7\n    max: 90\n    set: set_preview_height\n    get: get_preview_height\n  view_mode:\n    type: enum\n    values: [tiles, preview]\n    set: set_view_mode\n    get: get_view_mode\n")
	firstPage, secondPage, reorderedPage := render(first), render(second), render(reordered)
	if strings.Index(firstPage, "View Mode:") > strings.Index(firstPage, "Preview Height:") {
		t.Fatal("settings page did not preserve fixture control order")
	}
	if !strings.Contains(firstPage, "View Mode:") || strings.Contains(secondPage, "View Mode:") {
		t.Fatal("adding/removing a fixture control did not change settings page inventory")
	}
	if strings.Index(reorderedPage, "Preview Height:") > strings.Index(reorderedPage, "View Mode:") || reorderedPage == firstPage {
		t.Fatal("reordering fixture controls did not change settings page order")
	}
	if controlsFromSpec(first)[1].Min != 7 || controlsFromSpec(first)[1].Max != 90 {
		t.Fatal("fixture bounds were not loaded")
	}
	if controlsFromSpec(first)[0].Type != "enum" {
		t.Fatalf("fixture type = %q, want enum", controlsFromSpec(first)[0].Type)
	}
	settings := Settings{ViewMode: "tiles"}
	control := controlsFromSpec(first)[0]
	applySettingsDelta(control, 1, &settings)
	if settings.ViewMode != "preview" {
		t.Fatalf("fixture enum values did not drive behavior: %q", settings.ViewMode)
	}
	settings.MaxPreviewHeight = 7
	applySettingsDelta(controlsFromSpec(first)[1], -1, &settings)
	if settings.MaxPreviewHeight != 7 {
		t.Fatalf("fixture minimum bound not enforced: %d", settings.MaxPreviewHeight)
	}
}

func TestControlWithoutGoHandlerFailsIntegrity(t *testing.T) {
	controls := []ControlSpec{{Key: "extension", Set: "missing_set", Get: "missing_get"}}
	if err := validateControlHandlers(controls, controlHandlers); err == nil || !strings.Contains(err.Error(), "no Go setter handler") {
		t.Fatalf("missing handler integrity error = %v", err)
	}
	controls[0].Set = "preview_height"
	controls[0].Get = "get_preview_height"
	if err := validateControlHandlers(controls, controlHandlers); err == nil || !strings.Contains(err.Error(), "no Go setter handler") {
		t.Fatalf("bare control key unexpectedly registered as binding: %v", err)
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

func BenchmarkTplWidth(b *testing.B) {
	tpl := " {app_name | bold} [{dir}] — Page {page}/{pages} ({start}-{end} of {total})"
	vars := map[string]string{
		"app_name": "Cati Browser",
		"dir":      "/home/user/Pictures",
		"page":     "1",
		"pages":    "12",
		"start":    "1",
		"end":      "18",
		"total":    "210",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tplWidth(tpl, vars)
	}
}

func BenchmarkRenderTpl(b *testing.B) {
	tpl := " {app_name | bold} [{dir}] — Page {page}/{pages} ({start}-{end} of {total})"
	vars := map[string]string{
		"app_name": "Cati Browser",
		"dir":      "/home/user/Pictures",
		"page":     "1",
		"pages":    "12",
		"start":    "1",
		"end":      "18",
		"total":    "210",
	}
	baseAnsi := "\x1b[1m\x1b[30m\x1b[47m"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderTpl(tpl, vars, baseAnsi)
  }
}

func TestStyleAndAboutFixturesDriveOutput(t *testing.T) {
	styleDoc := "app:\n  border_style: box\nbuttons:\n  left_cap: '<'\n  right_cap: '>'\nscroll_bar:\n  width: 1\n"
	styleSpec, err := spec.LoadStyleFrom(fstest.MapFS{"style.yaml": {Data: []byte(styleDoc)}})
	if err != nil {
		t.Fatalf("load style fixture: %v", err)
	}
	style := styleConfigFromSpec(styleSpec)
	if got := loadButtons(style.BtnLeftCap, style.BtnRightCap)["prev"]; got != "<◀ Prev>" {
		t.Fatalf("changed style caps produced button label %q", got)
	}

	aboutFS := fstest.MapFS{"about.yaml": {Data: []byte("type: view\nname: about\ntitle: Fixture Title\ncontent: Fixture content\n")}}
	view, err := aboutViewFrom(aboutFS)
	if err != nil {
		t.Fatalf("load about fixture: %v", err)
	}
	var rendered bytes.Buffer
	drawAboutPage(&rendered, 80, 24, &StyleConfig{}, view)
	if !strings.Contains(rendered.String(), "Fixture Title") || !strings.Contains(rendered.String(), "Fixture content") {
		t.Fatalf("changed about fixture did not drive rendered output: %q", rendered.String())
	}
}

func TestMissingAndInvalidUIStyleSpecs(t *testing.T) {
	style, err := loadStyleFrom(fstest.MapFS{})
	if err != nil || style == nil || style.BtnLeftCap != "" || style.ScrollThumbChar != "" {
		t.Fatalf("missing style should degrade without seeded content: %+v, %v", style, err)
	}
	if _, err := loadStyleFrom(fstest.MapFS{"style.yaml": {Data: []byte("app: [")}}); err == nil {
		t.Fatal("invalid present style spec was silently accepted")
	}
	if _, err := aboutViewFrom(fstest.MapFS{"about.yaml": {Data: []byte("type: view\nname: about\ntitle: x\nunknown: y\n")}}); err == nil {
		t.Fatal("invalid present about spec was silently accepted")
	}
	missing, err := aboutViewFrom(fstest.MapFS{})
	if err != nil || missing == nil || missing.Title != "" || missing.Content != "" {
		t.Fatalf("missing about spec should degrade without seeded content: %+v, %v", missing, err)
	}
}
