package spec

import "testing"

func TestResolveGlyphSetExpression(t *testing.T) {
	tests := []struct {
		expr string
		ids  []int
		w, h int
	}{
		{"d", []int{0}, 1, 1},
		{"d1", []int{0, 1}, 1, 2},
		{"d2", []int{0, 2}, 1, 2},
		{"d1,6,9,44", []int{0, 1, 6, 9, 44}, 12, 12},
		{"q", []int{0, 1, 2, 4}, 2, 2},
		{"Q", []int{0, 1, 2, 4, 14, 15}, 2, 4},
		{"quad+six", []int{0, 1, 2, 4, 6}, 2, 6},
		{"bars+", []int{0, 1, 2, 4, 86}, 8, 8},
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

func TestResolveGlyphSetExpressionCaseAndErrors(t *testing.T) {
	for _, expr := range []string{"q", "Q", "b", "B", "a", "A", "z", "Z"} {
		if _, err := ResolveGlyphSetExpression(expr); err != nil {
			t.Errorf("%q: %v", expr, err)
		}
	}
	for _, expr := range []string{"d1,,2", "d,1", "d-1", "dfoo", "d999", "d ", "unknown"} {
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
