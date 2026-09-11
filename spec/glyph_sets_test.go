package spec

import "testing"

func TestResolveGlyphSetExpression(t *testing.T) {
	tests := []struct {
		expr string
		ids  []int
		w, h int
	}{
		{"d", []int{0}, 1, 1},
		{"d1", []int{0, 1}, 2, 1},
		{"d2", []int{0, 2}, 1, 2},
		{"d1,6,9,44", []int{0, 1, 6, 9, 44}, 12, 12},
		{"q", []int{0, 1, 2, 4}, 2, 2},
		{"Q", []int{0, 1, 2, 4, 14, 15}, 2, 4},
		{"quad+six", []int{0, 1, 2, 4, 6}, 2, 6},
		{"quad++six", []int{0, 1, 2, 4, 6, 14, 15}, 2, 12},
		{"fulls+quads", []int{0, 4}, 2, 2},
		{"bars+", []int{0, 1, 2, 4, 86}, 8, 8},
		{"1", []int{0}, 1, 1},
		{"2", []int{0, 2}, 1, 2},
		{"4", []int{0, 1, 2, 4}, 2, 2},
	}
	for _, tt := range tests {
		got, err := ResolveGlyphSetExpression(tt.expr)
		if err != nil {
			t.Fatalf("ResolveGlyphSetExpression(%q): %v", tt.expr, err)
		}
		if !equalInts(got.IDs, tt.ids) || got.Geometry.W != tt.w || got.Geometry.H != tt.h {
			t.Errorf("%q = ids %v geometry %dx%d, want %v geometry %dx%d", tt.expr, got.IDs, got.Geometry.W, got.Geometry.H, tt.ids, tt.w, tt.h)
		}
	}
}

func TestResolveGlyphSetExpressionMasksMatchGeometry(t *testing.T) {
	for _, expr := range []string{"d", "d1", "d2", "d4", "d6", "d9", "d14", "d15", "d44", "d45", "d86", "d88"} {
		got, err := ResolveGlyphSetExpression(expr)
		if err != nil {
			t.Fatalf("ResolveGlyphSetExpression(%q): %v", expr, err)
		}
		if len(got.Shapes) != len(got.Glyphs) {
			t.Fatalf("%q has %d shapes for %d glyphs", expr, len(got.Shapes), len(got.Glyphs))
		}
		for _, shape := range got.Shapes {
			if len(shape.Mask) != got.Geometry.W*got.Geometry.H {
				t.Errorf("%q glyph %q mask has %d pixels, want %d", expr, shape.Glyph, len(shape.Mask), got.Geometry.W*got.Geometry.H)
			}
		}
	}
}

func TestQuadMasksUseRowMajorCoverage(t *testing.T) {
	got, err := ResolveGlyphSetExpression("d4")
	if err != nil {
		t.Fatal(err)
	}
	want := map[rune]string{
		'▛': "1110", '▜': "1101", '▙': "1011", '▟': "0111",
	}
	for _, shape := range got.Shapes {
		if bits, ok := want[shape.Glyph]; ok && maskString(shape.Mask) != bits {
			t.Errorf("%q mask = %s, want %s", shape.Glyph, maskString(shape.Mask), bits)
		}
	}
}

func maskString(mask []bool) string {
	b := make([]byte, len(mask))
	for i, on := range mask {
		b[i] = '0'
		if on {
			b[i] = '1'
		}
	}
	return string(b)
}

func TestResolveGlyphSetExpressionCaseAndErrors(t *testing.T) {
	for _, expr := range []string{"q", "Q", "b", "B", "a", "A", "z", "Z"} {
		if _, err := ResolveGlyphSetExpression(expr); err != nil {
			t.Errorf("%q: %v", expr, err)
		}
	}
	for _, expr := range []string{"d1,,2", "d,1", "d-1", "dfoo", "d999", "d ", "unknown", "quad++", "+six", "14"} {
		if _, err := ResolveGlyphSetExpression(expr); err == nil {
			t.Errorf("ResolveGlyphSetExpression(%q) succeeded, want error", expr)
		}
	}
}

func TestResolveGlyphSetExpressionIsIdempotentAndSorted(t *testing.T) {
	got, err := ResolveGlyphSetExpression("d44,1,44,6,1")
	if err != nil {
		t.Fatal(err)
	}
	if !equalInts(got.IDs, []int{0, 1, 6, 44}) {
		t.Fatalf("IDs = %v", got.IDs)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
