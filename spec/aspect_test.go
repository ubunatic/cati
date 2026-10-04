package spec

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v3"
)

func TestPixelAspectPolicyLoader(t *testing.T) {
	for _, tc := range []struct {
		name, document string
		want           PixelAspectPolicy
		wantError      bool
	}{
		{"valid", "pixel: {max_distortion: 0.07, max_padding: 0.18}", PixelAspectPolicy{0.07, 0.18}, false},
		{"zero", "pixel: {max_distortion: 0, max_padding: 0}", PixelAspectPolicy{}, false},
		{"missing field", "pixel: {max_padding: 0.1}", PixelAspectPolicy{}, true},
		{"missing object", "{}", PixelAspectPolicy{}, true},
		{"unknown field", "pixel: {max_distortion: 0.1, max_padding: 0.1, typo: 1}", PixelAspectPolicy{}, true},
		{"negative", "pixel: {max_distortion: -0.1, max_padding: 0.1}", PixelAspectPolicy{}, true},
		{"too large", "pixel: {max_distortion: 1.1, max_padding: 0.1}", PixelAspectPolicy{}, true},
		{"empty content allowed", "pixel: {max_distortion: 0.1, max_padding: 1}", PixelAspectPolicy{}, true},
		{"nan", "pixel: {max_distortion: .nan, max_padding: 0.1}", PixelAspectPolicy{}, true},
		{"infinity", "pixel: {max_distortion: 0.1, max_padding: .inf}", PixelAspectPolicy{}, true},
		{"malformed", "pixel: [", PixelAspectPolicy{}, true},
		{"extra document", "pixel: {max_distortion: 0.1, max_padding: 0.1}\n---\npixel: {}", PixelAspectPolicy{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadPixelAspectPolicy(fstest.MapFS{"aspect.yaml": &fstest.MapFile{Data: []byte(tc.document)}})
			if (err != nil) != tc.wantError || got != tc.want {
				t.Fatalf("got %+v, %v; want %+v, error=%v", got, err, tc.want, tc.wantError)
			}
		})
	}
	if got, err := loadPixelAspectPolicy(fstest.MapFS{}); err != nil || got != (PixelAspectPolicy{}) {
		t.Fatalf("missing file should disable optional snapping: %+v, %v", got, err)
	}
}

func TestPixelAspectEmbeddedSchemaAndFidelity(t *testing.T) {
	data, err := fs.ReadFile(FS, "aspect.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Schema string             `yaml:"$schema"`
		Pixel  map[string]float64 `yaml:"pixel"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.Schema != "./schemas/aspect.schema.json" {
		t.Fatalf("unexpected schema reference %q", document.Schema)
	}
	schemaData, err := fs.ReadFile(FS, "schemas/aspect.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	// This spec's pixel properties are all bounded numbers. Check the embedded
	// YAML against those declared bounds and require exact field coverage.
	var schema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Required   []string `json:"required"`
			Properties map[string]struct {
				Type             string   `json:"type"`
				Minimum          float64  `json:"minimum"`
				Maximum          *float64 `json:"maximum"`
				ExclusiveMaximum *float64 `json:"exclusiveMaximum"`
			} `json:"properties"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	pixel := schema.Properties["pixel"]
	if !reflect.DeepEqual(schema.Required, []string{"pixel"}) || len(document.Pixel) != len(pixel.Properties) || len(pixel.Required) != len(pixel.Properties) {
		t.Fatal("schema required fields and YAML properties disagree")
	}
	for _, key := range pixel.Required {
		value, ok := document.Pixel[key]
		bounds, declared := pixel.Properties[key]
		if !ok || !declared || bounds.Type != "number" || value < bounds.Minimum || (bounds.Maximum != nil && value > *bounds.Maximum) || (bounds.ExclusiveMaximum != nil && value >= *bounds.ExclusiveMaximum) {
			t.Fatalf("%s=%v does not conform to schema", key, value)
		}
	}
	loaded, err := LoadPixelAspectPolicy()
	if err != nil || loaded.MaxDistortion != document.Pixel["max_distortion"] || loaded.MaxPadding != document.Pixel["max_padding"] {
		t.Fatalf("loader differs from embedded YAML: %+v, %v", loaded, err)
	}
}
