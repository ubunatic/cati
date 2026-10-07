package spec

import (
	"strings"
	"testing"
)

func TestLoadCompositionsExpandAliasesInFileOrder(t *testing.T) {
	data := `$schema: schemas/compositions.schema.json
compositions:
  - name: full
    aliases: [f]
    sets: [fulls]
    info: full cell
    experimental: false
  - name: next
    aliases: []
    sets: [fulls]
    info: next cell
    experimental: true
`
	got, err := LoadRenderModesFrom(glyphSetFS(map[string]string{
		"glyph_sets/00-fulls.yaml": glyphSetFixture,
		"compositions.yaml":        data,
	}))
	if err != nil {
		t.Fatalf("LoadRenderModesFrom: %v", err)
	}
	if !equalInts(got.Compositions["full"], []int{0}) || !equalInts(got.Compositions["f"], []int{0}) {
		t.Fatalf("full composition and alias = %#v", got.Compositions)
	}
	if strings.Join(got.CompositionOrder, ",") != "full,next" {
		t.Fatalf("composition order = %v", got.CompositionOrder)
	}
	if got.CompositionInfo["full"] != "full cell" || strings.Join(got.Experimental, ",") != "next" {
		t.Fatalf("composition metadata = info %#v, experimental %v", got.CompositionInfo, got.Experimental)
	}
}

func TestLoadCompositionsRejectUnknownSetAndAliasCollision(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "unknown set",
			data: `$schema: schemas/compositions.schema.json
compositions:
  - name: full
    aliases: []
    sets: [missing]
    info: full cell
    experimental: false
`,
			want: `composition "full" references unknown glyph set "missing"`,
		},
		{
			name: "alias collides with composition name",
			data: `$schema: schemas/compositions.schema.json
compositions:
  - name: first
    aliases: [second]
    sets: [fulls]
    info: first cell
    experimental: false
  - name: second
    aliases: []
    sets: [fulls]
    info: second cell
    experimental: false
`,
			want: `composition name "second" collides`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadRenderModesFrom(glyphSetFS(map[string]string{
				"glyph_sets/00-fulls.yaml": glyphSetFixture,
				"compositions.yaml":        tt.data,
			}))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
