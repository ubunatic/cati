package spec

import "testing"

func TestEmbeddedStyleAndAboutSpecsLoad(t *testing.T) {
	style, err := LoadStyle()
	if err != nil {
		t.Fatalf("LoadStyle embedded style.yaml: %v", err)
	}
	if style.Schema != "./schemas/style.schema.json" {
		t.Fatalf("style schema reference = %q", style.Schema)
	}
	view, err := LoadYamlView("about.yaml")
	if err != nil {
		t.Fatalf("LoadYamlView embedded about.yaml: %v", err)
	}
	if view.Name != "about" || view.Title == "" || view.Content == "" {
		t.Fatalf("embedded about view is incomplete: %+v", view)
	}
}
