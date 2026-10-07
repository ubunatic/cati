package spec

import (
	"strings"
	"testing"
	"testing/fstest"
)

const glyphSetFixture = `# yaml-language-server: $schema=../schemas/glyph_set.schema.json
$schema: ../schemas/glyph_set.schema.json
id: 0
order: 0
name: fulls
geometry: { w: 1, h: 1 }
glyphs: [" ", "█"]
masks: {" ": "0", "█": "1"}
optimized: true
`

func glyphSetFS(files map[string]string) fstest.MapFS {
	f := fstest.MapFS{"render_modes.yaml": {Data: []byte("cycle: []\nmodes: []\n")}}
	for name, data := range files {
		f[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return f
}

func TestLoadRenderModesGlyphSetFiles(t *testing.T) {
	f := glyphSetFS(map[string]string{"glyph_sets/00-fulls.yaml": glyphSetFixture})
	got, err := LoadRenderModesFrom(f)
	if err != nil {
		t.Fatalf("LoadRenderModesFrom: %v", err)
	}
	if len(got.SetRegistry) != 1 || got.SetRegistry[0].ID != 0 || got.SetRegistry[0].Name != "fulls" {
		t.Fatalf("registry = %+v, want one fulls set", got.SetRegistry)
	}
}

func TestLoadRenderModesGlyphSetFileErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "duplicate id",
			files: map[string]string{
				"glyph_sets/00-fulls.yaml": glyphSetFixture,
				"glyph_sets/00-copy.yaml":  strings.ReplaceAll(glyphSetFixture, "name: fulls", "name: copy"),
			},
			want: "duplicate glyph set ID 0",
		},
		{
			name:  "filename prefix mismatch",
			files: map[string]string{"glyph_sets/01-fulls.yaml": glyphSetFixture},
			want:  "filename ID prefix does not match id 0",
		},
		{
			name:  "invalid file names file",
			files: map[string]string{"glyph_sets/00-fulls.yaml": "id: [\n"},
			want:  "glyph_sets/00-fulls.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadRenderModesFrom(glyphSetFS(tt.files))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadRenderModesMissingGlyphSetDirectory(t *testing.T) {
	got, err := LoadRenderModesFrom(glyphSetFS(nil))
	if err != nil {
		t.Fatalf("LoadRenderModesFrom without glyph_sets/: %v", err)
	}
	if len(got.SetRegistry) != 0 {
		t.Fatalf("registry has %d sets without directory, want empty", len(got.SetRegistry))
	}
}
