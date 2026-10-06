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
		{"octant", []int{0, 10}, 2, 4},
		{"oct", []int{0, 10}, 2, 4},
		{"vector", []int{0, 50}, 4, 8},
		{"v", []int{0, 50}, 4, 8},
		{"vec", []int{0, 50}, 4, 8},
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

func TestOctantAndVectorGlyphSets(t *testing.T) {
	oct, err := ResolveGlyphSetExpression("octant")
	if err != nil {
		t.Fatalf("ResolveGlyphSetExpression(octant): %v", err)
	}
	if oct.Geometry.W != 2 || oct.Geometry.H != 4 {
		t.Errorf("octant geometry = %dx%d, want 2x4", oct.Geometry.W, oct.Geometry.H)
	}
	if len(oct.Shapes) != 256 {
		t.Errorf("octant shapes count = %d, want 256", len(oct.Shapes))
	}
	hasOctantRune := false
	for _, shape := range oct.Shapes {
		if shape.Glyph >= 0x1CD00 && shape.Glyph <= 0x1CDE5 {
			hasOctantRune = true
			break
		}
	}
	if !hasOctantRune {
		t.Errorf("octant mode missing U+1CD00..U+1CDE5 characters")
	}

	vec, err := ResolveGlyphSetExpression("vector")
	if err != nil {
		t.Fatalf("ResolveGlyphSetExpression(vector): %v", err)
	}
	if vec.Geometry.W != 4 || vec.Geometry.H != 8 {
		t.Errorf("vector geometry = %dx%d, want 4x8", vec.Geometry.W, vec.Geometry.H)
	}
	if len(vec.Shapes) < 50 {
		t.Errorf("vector shapes count = %d, want >= 50", len(vec.Shapes))
	}
	hasDiagonal := false
	for _, shape := range vec.Shapes {
		if shape.Glyph >= 0x1FB3C && shape.Glyph <= 0x1FB6F {
			hasDiagonal = true
		}
		if shape.Glyph >= 0x1FB00 && shape.Glyph <= 0x1FB3B && shape.Glyph != 0x1FB03 && shape.Glyph != 0x1FB07 && shape.Glyph != 0x1FB0B {
			t.Errorf("vector mode unexpectedly includes pixel-like sextant rune %U", shape.Glyph)
		}
		if shape.Glyph == '▘' || shape.Glyph == '▝' || shape.Glyph == '▖' || shape.Glyph == '▗' {
			t.Errorf("vector mode unexpectedly includes quadrant rune %c", shape.Glyph)
		}
	}
	if !hasDiagonal {
		t.Errorf("vector mode missing U+1FB3C..U+1FB6F linear diagonal shapes")
	}

	// Verify mask orientation invariants for representative vector shapes:
	// x=0..3 (left to right), y=0..7 (top to bottom).
	vecShapeMap := make(map[rune][]bool, len(vec.Shapes))
	for _, shape := range vec.Shapes {
		vecShapeMap[shape.Glyph] = shape.Mask
	}

	// 1. U+1FB9A (🮚): upper-left diagonal half block contains top-left (0,0)
	if mask, ok := vecShapeMap['\U0001FB9A']; ok {
		if !mask[0*4+0] || mask[7*4+3] {
			t.Errorf("U+1FB9A mask orientation incorrect: top-left (0,0)=%v, bottom-right (3,7)=%v", mask[0*4+0], mask[7*4+3])
		}
	} else {
		t.Errorf("missing U+1FB9A shape")
	}

	// 2. U+1FB9B (🮛): lower-left diagonal half block contains bottom-left (0,7)
	if mask, ok := vecShapeMap['\U0001FB9B']; ok {
		if !mask[7*4+0] || mask[0*4+3] {
			t.Errorf("U+1FB9B mask orientation incorrect: bottom-left (0,7)=%v, top-right (3,0)=%v", mask[7*4+0], mask[0*4+3])
		}
	} else {
		t.Errorf("missing U+1FB9B shape")
	}

	// 3. U+1FB3C (🬼): lower-left block diagonal contains bottom-left (0,7)
	if mask, ok := vecShapeMap[0x1FB3C]; ok {
		if !mask[7*4+0] || mask[0*4+3] {
			t.Errorf("U+1FB3C mask orientation incorrect: bottom-left (0,7)=%v, top-right (3,0)=%v", mask[7*4+0], mask[0*4+3])
		}
	} else {
		t.Errorf("missing U+1FB3C shape")
	}

	// 4. U+1FB41 (🭁): lower-right block diagonal contains bottom-right (3,7)
	if mask, ok := vecShapeMap[0x1FB41]; ok {
		if !mask[7*4+3] || mask[0*4+0] {
			t.Errorf("U+1FB41 mask orientation incorrect: bottom-right (3,7)=%v, top-left (0,0)=%v", mask[7*4+3], mask[0*4+0])
		}
	} else {
		t.Errorf("missing U+1FB41 shape")
	}

	// 5. U+1FB57 (🭗): upper-left block diagonal contains top-left (0,0)
	if mask, ok := vecShapeMap[0x1FB57]; ok {
		if !mask[0*4+0] || mask[7*4+3] {
			t.Errorf("U+1FB57 mask orientation incorrect: top-left (0,0)=%v, bottom-right (3,7)=%v", mask[0*4+0], mask[7*4+3])
		}
	} else {
		t.Errorf("missing U+1FB57 shape")
	}

	// 6. U+1FB62 (🭢): upper-right block diagonal contains top-right (3,0)
	if mask, ok := vecShapeMap[0x1FB62]; ok {
		if !mask[0*4+3] || mask[7*4+0] {
			t.Errorf("U+1FB62 mask orientation incorrect: top-right (3,0)=%v, bottom-left (0,7)=%v", mask[0*4+3], mask[7*4+0])
		}
	} else {
		t.Errorf("missing U+1FB62 shape")
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
