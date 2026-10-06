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
	formulaYAML := "\nformula: {default: {num: 2, den: 3}, halfblock: {num: 1, den: 2}}"
	defaultFormula := FormulaPolicy{
		Default:   FormulaParams{Num: 2, Den: 3},
		Halfblock: FormulaParams{Num: 1, Den: 2},
	}
	for _, tc := range []struct {
		name, document string
		want           PixelAspectPolicy
		wantError      bool
	}{
		{"valid", "pixel: {max_distortion: 0.07, max_padding: 0.18}" + formulaYAML, PixelAspectPolicy{0.07, 0.18, defaultFormula}, false},
		{"zero", "pixel: {max_distortion: 0, max_padding: 0}" + formulaYAML, PixelAspectPolicy{0, 0, defaultFormula}, false},
		{"missing field", "pixel: {max_padding: 0.1}" + formulaYAML, PixelAspectPolicy{}, true},
		{"missing object", "{}", PixelAspectPolicy{}, true},
		{"missing formula", "pixel: {max_distortion: 0.1, max_padding: 0.1}", PixelAspectPolicy{}, true},
		{"formula zero num", "pixel: {max_distortion: 0.1, max_padding: 0.1}, formula: {default: {num: 0, den: 3}, halfblock: {num: 1, den: 2}}", PixelAspectPolicy{}, true},
		{"unknown field", "pixel: {max_distortion: 0.1, max_padding: 0.1, typo: 1}" + formulaYAML, PixelAspectPolicy{}, true},
		{"negative", "pixel: {max_distortion: -0.1, max_padding: 0.1}" + formulaYAML, PixelAspectPolicy{}, true},
		{"too large", "pixel: {max_distortion: 1.1, max_padding: 0.1}" + formulaYAML, PixelAspectPolicy{}, true},
		{"empty content allowed", "pixel: {max_distortion: 0.1, max_padding: 1}" + formulaYAML, PixelAspectPolicy{}, true},
		{"nan", "pixel: {max_distortion: .nan, max_padding: 0.1}" + formulaYAML, PixelAspectPolicy{}, true},
		{"infinity", "pixel: {max_distortion: 0.1, max_padding: .inf}" + formulaYAML, PixelAspectPolicy{}, true},
		{"malformed", "pixel: [", PixelAspectPolicy{}, true},
		{"extra document", "pixel: {max_distortion: 0.1, max_padding: 0.1}" + formulaYAML + "\n---\npixel: {}", PixelAspectPolicy{}, true},
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
		Schema  string             `yaml:"$schema"`
		Pixel   map[string]float64 `yaml:"pixel"`
		Formula struct {
			Default   map[string]int `yaml:"default"`
			Halfblock map[string]int `yaml:"halfblock"`
		} `yaml:"formula"`
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
	var schema struct {
		Required   []string `json:"required"`
		Properties struct {
			Pixel struct {
				Required   []string `json:"required"`
				Properties map[string]struct {
					Type             string   `json:"type"`
					Minimum          float64  `json:"minimum"`
					Maximum          *float64 `json:"maximum"`
					ExclusiveMaximum *float64 `json:"exclusiveMaximum"`
				} `json:"properties"`
			} `json:"pixel"`
			Formula struct {
				Required   []string `json:"required"`
				Properties map[string]struct {
					Required   []string `json:"required"`
					Properties map[string]struct {
						Type    string `json:"type"`
						Minimum int    `json:"minimum"`
					} `json:"properties"`
				} `json:"properties"`
			} `json:"formula"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(schema.Required, []string{"pixel", "formula"}) {
		t.Fatalf("schema required fields disagree: %+v", schema.Required)
	}
	pixel := schema.Properties.Pixel
	if len(document.Pixel) != len(pixel.Properties) || len(pixel.Required) != len(pixel.Properties) {
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
	if loaded.Formula.Default.Num != document.Formula.Default["num"] || loaded.Formula.Default.Den != document.Formula.Default["den"] ||
		loaded.Formula.Halfblock.Num != document.Formula.Halfblock["num"] || loaded.Formula.Halfblock.Den != document.Formula.Halfblock["den"] {
		t.Fatalf("loader formula differs from embedded YAML: %+v", loaded)
	}
}
