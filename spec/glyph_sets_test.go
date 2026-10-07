package spec

import (
	"fmt"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

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
		{"vec2", []int{0, 52}, 2, 2},
		{"vec3", []int{0, 53}, 2, 3},
		{"vec4", []int{0, 54}, 3, 4},
		{"vec5", []int{0, 55}, 3, 5},
		{"vec6", []int{0, 56}, 4, 6},
		{"vec8", []int{0, 58}, 4, 8},
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

	vec, err := ResolveGlyphSetExpression("vec6")
	if err != nil {
		t.Fatalf("ResolveGlyphSetExpression(vec6): %v", err)
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
	}
	if !hasDiagonal {
		t.Errorf("vector mode missing U+1FB3C..U+1FB6F linear diagonal shapes")
	}

	// Verify that hourglass and bowtie are exact complements
	var hourglass, bowtie GlyphShape
	for _, s := range vec.Shapes {
		if s.Glyph == 0x1FB9A {
			hourglass = s
		} else if s.Glyph == 0x1FB9B {
			bowtie = s
		}
	}
	if hourglass.Glyph == 0 || bowtie.Glyph == 0 {
		t.Fatalf("vector mode missing hourglass or bowtie glyphs")
	}
	for i := range hourglass.Mask {
		if hourglass.Mask[i] == bowtie.Mask[i] {
			t.Errorf("hourglass and bowtie masks at bit %d must be complements, got hourglass=%v bowtie=%v", i, hourglass.Mask[i], bowtie.Mask[i])
		}
	}
}

// TestVectorStraightBarsAndQuadrants checks the spec's vector shapes at 4x4,
// where every bar and quadrant boundary falls on a pixel edge; no render mode
// is involved.
func TestVectorStraightBarsAndQuadrants(t *testing.T) {
	rm, err := LoadRenderModes()
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := vectorShapes(rm.VectorShapes, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	masks := make(map[rune]uint16)
	for _, shape := range shapes {
		if _, exists := masks[shape.Glyph]; exists {
			t.Errorf("duplicate vector rune %U", shape.Glyph)
		}
		if len(shape.Mask) != 16 {
			t.Fatalf("%U mask length = %d, want 16", shape.Glyph, len(shape.Mask))
		}
		var bits uint16
		for i, on := range shape.Mask {
			if on {
				bits |= 1 << i // Row-major bit index = y*4+x.
			}
		}
		masks[shape.Glyph] = bits
	}
	want := map[rune]uint16{
		0x1FB82: 0x000F, 0x1FB85: 0x0FFF, '▂': 0xF000, '▆': 0xFFF0,
		'▎': 0x1111, '▊': 0x7777, 0x1FB87: 0x8888, 0x1FB8A: 0xEEEE,
		'▖': 0x3300, '▗': 0xCC00, '▘': 0x0033, '▙': 0xFF33, '▚': 0xCC33,
		'▛': 0x33FF, '▜': 0xCCFF, '▝': 0x00CC, '▞': 0x33CC, '▟': 0xFFCC,
	}
	for r, bits := range want {
		if got, exists := masks[r]; !exists || got != bits {
			t.Errorf("%U present=%v mask=%04X, want %04X", r, exists, got, bits)
		}
	}
	for _, pair := range [][2]rune{{0x1FB82, '▆'}, {'▂', 0x1FB85}, {'▎', 0x1FB8A}, {0x1FB87, '▊'}} {
		if got := masks[pair[0]] ^ masks[pair[1]]; got != 0xFFFF {
			t.Errorf("%U XOR %U = %04X, want FFFF", pair[0], pair[1], got)
		}
	}
}

func TestVectorParameterizedGeometries(t *testing.T) {
	geometries := []struct {
		name string
		w, h int
	}{
		{"vec2", 2, 2},
		{"vec3", 2, 3},
		{"vec4", 3, 4},
		{"vec5", 3, 5},
		{"vec6", 4, 6},
		{"vec8", 4, 8},
	}

	for _, g := range geometries {
		t.Run(g.name, func(t *testing.T) {
			res, err := ResolveGlyphSetExpression(g.name)
			if err != nil {
				t.Fatalf("ResolveGlyphSetExpression(%q): %v", g.name, err)
			}
			if res.Geometry.W != g.w || res.Geometry.H != g.h {
				t.Errorf("%s geometry = %dx%d, want %dx%d", g.name, res.Geometry.W, res.Geometry.H, g.w, g.h)
			}

			shapesByRune := make(map[rune][]bool, len(res.Shapes))
			wantLen := g.w * g.h
			for _, shape := range res.Shapes {
				if len(shape.Mask) != wantLen {
					t.Errorf("%s glyph %U mask length = %d, want %d", g.name, shape.Glyph, len(shape.Mask), wantLen)
				}
				shapesByRune[shape.Glyph] = shape.Mask
			}

			assertComplements := func(r1, r2 rune) {
				m1, ok1 := shapesByRune[r1]
				m2, ok2 := shapesByRune[r2]
				if !ok1 || !ok2 {
					t.Errorf("%s missing complement rune pair %U / %U", g.name, r1, r2)
					return
				}
				for i := 0; i < wantLen; i++ {
					if m1[i] == m2[i] {
						t.Errorf("%s glyph pair %U and %U mask bit %d = %v, want complement", g.name, r1, r2, i, m1[i])
					}
				}
			}

			// Complement checks:
			assertComplements(' ', '█')
			assertComplements('▀', '▄')
			assertComplements('▌', '▐')
			assertComplements(0x1FB9A, 0x1FB9B) // hourglass & bowtie

			// Triangular corners:
			assertComplements(0x1FB6C, 0x1FB68)
			assertComplements(0x1FB6D, 0x1FB69)
			assertComplements(0x1FB6E, 0x1FB6A)
			assertComplements(0x1FB6F, 0x1FB6B)

			// Straight bars:
			assertComplements(0x1FB82, '▆')
			assertComplements('▂', 0x1FB85)
			assertComplements('▎', 0x1FB8A)
			assertComplements(0x1FB87, '▊')

			// Quadrants (each 1-quadrant block is paired with its 3-quadrant inverse):
			assertComplements('▘', '▟')
			assertComplements('▝', '▙')
			assertComplements('▖', '▜')
			assertComplements('▗', '▛')

			// 22 block diagonal pairs:
			for k := rune(0); k < 22; k++ {
				assertComplements(0x1FB3C+k, 0x1FB52+k)
			}
		})
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

func TestVectorShapesRejectInvalidDefinitions(t *testing.T) {
	half := CellFrac{1, 2}
	one := CellFrac{1, 1}
	points := map[string][2]CellFrac{"UL": {}, "LR": {one, one}}
	for name, glyphs := range map[string][]VectorShapeDef{
		"two runes":       {{Glyph: "ab"}},
		"duplicate":       {{Glyph: "a"}, {Glyph: "a"}},
		"mixed kinds":     {{Glyph: "a", Rects: [][4]CellFrac{{{}, {}, half, half}}, Invert: "b"}},
		"unknown point":   {{Glyph: "a", Split: []string{"UL", "XX", "LR"}}},
		"short split":     {{Glyph: "a", Split: []string{"UL", "LR"}}},
		"invert missing":  {{Glyph: "a", Invert: "b"}},
		"invert chain":    {{Glyph: "a"}, {Glyph: "b", Invert: "a"}, {Glyph: "c", Invert: "b"}},
		"masks on invert": {{Glyph: "a"}, {Glyph: "b", Invert: "a", Masks: map[string][]string{"1x1": {"#"}}}},
		"mask key":        {{Glyph: "a", Masks: map[string][]string{"3by5": {"#"}}}},
		"mask rows":       {{Glyph: "a", Masks: map[string][]string{"1x2": {"#"}}}},
		"mask width":      {{Glyph: "a", Masks: map[string][]string{"2x1": {"#"}}}},
		"mask char":       {{Glyph: "a", Masks: map[string][]string{"1x1": {"x"}}}},
	} {
		if _, err := vectorShapes(VectorShapesSpec{Points: points, Glyphs: glyphs}, 4, 4); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestCellFracParsesNumbersAndFractions(t *testing.T) {
	var doc struct {
		V []CellFrac `yaml:"v"`
	}
	if err := yaml.Unmarshal([]byte("v: [0, 1/3, 0.25, 1]"), &doc); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{0, 2, 1.5, 6} {
		if got := doc.V[i].of(6); got != want {
			t.Errorf("v[%d] of 6 = %v, want %v", i, got, want)
		}
	}
	for _, bad := range []string{"v: [2]", "v: [3/2]", "v: [1/0]", "v: [x]"} {
		if err := yaml.Unmarshal([]byte(bad), &doc); err == nil {
			t.Errorf("%s: want error", bad)
		}
	}
}

// TestVectorSextantsMatchUnicodeSegments checks every sextant rune of the
// vector set against the sextant generator, which follows the Unicode names.
func TestVectorSextantsMatchUnicodeSegments(t *testing.T) {
	rm, err := LoadRenderModes()
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[rune][]bool)
	for _, shape := range generatedSextantShapes() {
		want[shape.Glyph] = shape.Mask
	}
	sextant := RenderModeGeometry{W: 2, H: 3}
	for _, g := range []RenderModeGeometry{{W: 2, H: 3}, {W: 4, H: 6}} {
		shapes, err := vectorShapes(rm.VectorShapes, g.W, g.H)
		if err != nil {
			t.Fatal(err)
		}
		for _, shape := range shapes {
			if shape.Glyph < 0x1FB00 || shape.Glyph > 0x1FB3B {
				continue
			}
			wantMask := scaleMask(want[shape.Glyph], sextant, g)
			if !slices.Equal(shape.Mask, wantMask) {
				t.Errorf("%dx%d %c %U mask %v, want %v", g.W, g.H, shape.Glyph, shape.Glyph, shape.Mask, wantMask)
			}
		}
	}
}

// TestVectorDrawnMasksAreUsed checks every hand-drawn mask in the spec: its size
// belongs to a vector set, the generator returns it unchanged, and a glyph that
// inverts it gets the exact complement.
func TestVectorDrawnMasksAreUsed(t *testing.T) {
	rm, err := LoadRenderModes()
	if err != nil {
		t.Fatal(err)
	}
	sizes := make(map[string]bool)
	for _, def := range rm.SetRegistry {
		if def.Generated == "vector_linear_splits" {
			sizes[fmt.Sprintf("%dx%d", def.Geometry.W, def.Geometry.H)] = true
		}
	}
	for _, def := range rm.VectorShapes.Glyphs {
		for key, rows := range def.Masks {
			if !sizes[key] {
				t.Errorf("%s: mask size %s matches no vector set", def.Glyph, key)
				continue
			}
			var w, h int
			fmt.Sscanf(key, "%dx%d", &w, &h)
			shapes, err := vectorShapes(rm.VectorShapes, w, h)
			if err != nil {
				t.Fatal(err)
			}
			want := []bool{}
			for _, row := range rows {
				for _, c := range row {
					want = append(want, c == '#')
				}
			}
			for _, shape := range shapes {
				if string(shape.Glyph) == def.Glyph && !slices.Equal(shape.Mask, want) {
					t.Errorf("%s %s: generator mask %v, want drawn %v", def.Glyph, key, shape.Mask, want)
				}
			}
			for _, inv := range rm.VectorShapes.Glyphs {
				if inv.Invert != def.Glyph {
					continue
				}
				for _, shape := range shapes {
					if string(shape.Glyph) != inv.Glyph {
						continue
					}
					for i := range want {
						if shape.Mask[i] == want[i] {
							t.Errorf("%s %s: inverse %s pixel %d not complemented", def.Glyph, key, inv.Glyph, i)
						}
					}
				}
			}
		}
	}
}

// TestVectorQuadrantsTileHalves checks every vector set geometry, odd ones
// included: two quadrants fill their half block exactly, without overlap.
func TestVectorQuadrantsTileHalves(t *testing.T) {
	rm, err := LoadRenderModes()
	if err != nil {
		t.Fatal(err)
	}
	for _, def := range rm.SetRegistry {
		if def.Generated != "vector_linear_splits" {
			continue
		}
		w, h := def.Geometry.W, def.Geometry.H
		shapes, err := vectorShapes(rm.VectorShapes, w, h)
		if err != nil {
			t.Fatal(err)
		}
		masks := make(map[rune][]bool)
		for _, shape := range shapes {
			masks[shape.Glyph] = shape.Mask
		}
		for _, tile := range [][3]rune{{'▘', '▝', '▀'}, {'▖', '▗', '▄'}, {'▘', '▖', '▌'}, {'▝', '▗', '▐'}} {
			a, b, whole := masks[tile[0]], masks[tile[1]], masks[tile[2]]
			for i := range whole {
				if a[i] && b[i] || (a[i] || b[i]) != whole[i] {
					t.Errorf("%s %dx%d: %c + %c != %c at pixel %d", def.Name, w, h, tile[0], tile[1], tile[2], i)
					break
				}
			}
		}
	}
}
