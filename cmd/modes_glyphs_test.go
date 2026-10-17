package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"ubunatic.com/cati/spec"
)

func executeGlyphs(args ...string) (string, error) {
	command := modesCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(append([]string{"glyphs"}, args...))
	err := command.Execute()
	return out.String(), err
}

func TestModesGlyphsSelectors(t *testing.T) {
	if testing.Short() {
		t.Skip("glyph inventory selector sweep")
	}
	for _, tc := range []struct{ selector, geometry string }{
		{"half", "1x2"}, {"h", "1x2"}, {"oct", "2x4"}, {"vec6", "4x6"},
		{"octants", "2x4"}, {"d10", "2x4"}, {"d10,56", "4x12"},
		{"half+sextants", "2x6"}, {"six", "2x6"}, {"2x3", "2x3"}, {"2x4", "2x4"},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			out, err := executeGlyphs(tc.selector)
			if err != nil {
				t.Fatal(err)
			}
			resolution, err := spec.ResolveGlyphSetExpression(tc.selector)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "Geometry: "+tc.geometry) || strings.Count(out, " U+") != len(resolution.Shapes) {
				t.Fatalf("incorrect inventory: %s", out)
			}
			offset := 0
			for _, shape := range resolution.Shapes {
				// Verify stable resolver order, including supplementary codepoints.
				token := fmtCodepoint(shape.Glyph)
				index := strings.Index(out[offset:], token)
				if index < 0 {
					t.Fatalf("missing or reordered %s", token)
				}
				offset += index + len(token)
			}
		})
	}
	a, _ := executeGlyphs("d56,10,10")
	b, _ := executeGlyphs("d10,56")
	if a[strings.Index(a, "\n"):] != b[strings.Index(b, "\n"):] {
		t.Fatal("union normalization differs")
	}
}

func fmtCodepoint(r rune) string { return fmt.Sprintf("U+%04X", r) }

func TestModesGlyphsHalfCards(t *testing.T) {
	asciiOut, err := executeGlyphs("half", "--format", "ascii", "--grid", "1")
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range []string{
		"␠ (space) U+0020 1x2\n┌─┐\n│.│\n│.│\n└─┘",
		"'█' U+2588 1x2\n┌─┐\n│#│\n│#│\n└─┘",
		"'▀' U+2580 1x2\n┌─┐\n│#│\n│.│\n└─┘",
		"'▄' U+2584 1x2\n┌─┐\n│.│\n│#│\n└─┘",
	} {
		if !strings.Contains(asciiOut, card) {
			t.Fatalf("missing card %q", card)
		}
	}

	// Test default format (halfblock) and default grid (8 columns side-by-side)
	gridOut, err := executeGlyphs("half")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gridOut, "␠ (space) U+0020 1x2    '█' U+2588 1x2    '▀' U+2580 1x2    '▄' U+2584 1x2") {
		t.Fatalf("expected grid layout in %s", gridOut)
	}
	if !strings.Contains(gridOut, "│ │                     │█│               │▀│               │▄│") {
		t.Fatalf("expected halfblock glyph preview in %s", gridOut)
	}
}

func TestModesGlyphsGeneratedOrientation(t *testing.T) {
	for _, tc := range []struct{ selector, card string }{
		{"sextants", "'🬀' U+1FB00 2x3\n┌──┐\n│#.│\n│..│\n│..│\n└──┘"},
		{"oct", "'▘' U+2598 2x4\n┌──┐\n│#.│\n│#.│\n│..│\n│..│\n└──┘"},
		{"brailles", "'⠈' U+2808 2x4\n┌──┐\n│.#│\n│..│\n│..│\n│..│\n└──┘"},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			out, err := executeGlyphs(tc.selector, "--format", "ascii", "--grid", "1")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.card) {
				t.Fatalf("missing oriented card %q", tc.card)
			}
		})
	}
}

func TestGlyphMaskPreview(t *testing.T) {
	for _, tc := range []struct {
		name, format, want string
		w, h               int
		mask               []bool
	}{
		{"quadrant", "ascii", "┌──┐\n│#.│\n│..│\n└──┘", 2, 2, []bool{true, false, false, false}},
		{"asymmetric sextant", "ascii", "┌──┐\n│#.│\n│.#│\n│..│\n└──┘", 2, 3, []bool{true, false, false, true, false, false}},
		{"halfblock combinations", "halfblock", "┌────┐\n│ ▀▄█│\n└────┘", 4, 2, []bool{false, true, false, true, false, false, true, true}},
		{"odd height", "halfblock", "┌──┐\n│▀▄│\n│▀ │\n└──┘", 2, 3, []bool{true, false, false, true, true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := glyphMaskPreview(tc.mask, tc.w, tc.h, tc.format)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		w, h   int
		mask   []bool
		format string
	}{
		{0, 1, nil, "ascii"}, {1, 0, nil, "ascii"}, {2, 2, []bool{true}, "ascii"}, {1, 1, []bool{true}, "bad"},
	} {
		if _, err := glyphMaskPreview(tc.mask, tc.w, tc.h, tc.format); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}

func TestModesGlyphsErrorsAndHelp(t *testing.T) {
	for _, args := range [][]string{
		{"unknown"}, {"d999"}, {"d10,"}, {"half+"}, {"half", "quad"},
		{"--format", "bad"}, {"--image", "missing.png"}, {"--width", "3"},
		{"half", "--grid", "0"}, {"half", "--grid", "-1"},
	} {
		if _, err := executeGlyphs(args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, args := range [][]string{nil, {"--help"}} {
		out, err := executeGlyphs(args...)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "glyph") && !strings.Contains(out, "Glyph") {
			t.Fatal(out)
		}
	}
	out, err := executeGlyphs("half")
	if err != nil || !strings.Contains(out, "│▀│") {
		t.Fatalf("%s %v", out, err)
	}
	asciiOut, err := executeGlyphs("half", "--format", "ascii")
	if err != nil || !strings.Contains(asciiOut, "│#│") {
		t.Fatalf("%s %v", asciiOut, err)
	}
}

type glyphFailWriter struct{}

var errGlyphWrite = errors.New("glyph writer failed")

func (glyphFailWriter) Write([]byte) (int, error) { return 0, errGlyphWrite }
func TestModesGlyphsWriterErrors(t *testing.T) {
	if err := inspectGlyphs(glyphFailWriter{}, "half", "ascii", 8); !errors.Is(err, errGlyphWrite) {
		t.Fatal(err)
	}
	if err := listGlyphSelectors(glyphFailWriter{}); !errors.Is(err, errGlyphWrite) {
		t.Fatal(err)
	}
}

// The independent fixture specifies an asymmetric 2x3 mask. Decode both
// previews back to pixels so swapped rows, columns and odd-row padding fail.
func TestGlyphMaskPreviewGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/glyph_masks/asymmetric_2x3.png")
	if err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []string{"Algorithm\x00glyph-mask-card", "Geometry\x002x3", "Mask\x00100110"} {
		if !bytes.Contains(data, []byte(metadata)) {
			t.Fatalf("missing PNG metadata %q", metadata)
		}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 3 {
		t.Fatal(img.Bounds())
	}
	mask := []bool{true, false, false, true, true, false}
	for _, format := range []string{"ascii", "halfblock"} {
		preview, err := glyphMaskPreview(mask, 2, 3, format)
		if err != nil {
			t.Fatal(err)
		}
		rows := strings.Split(preview, "\n")
		for y := 0; y < 3; y++ {
			for x := 0; x < 2; x++ {
				row := y
				if format == "halfblock" {
					row = y / 2
				}
				ch := []rune(rows[row+1])[x+1]
				on := ch == '#'
				if format == "halfblock" {
					on = ch == '█' || (y%2 == 0 && ch == '▀') || (y%2 == 1 && ch == '▄')
				}
				r, _, _, _ := img.At(x, y).RGBA()
				if on != (r != 0) {
					t.Fatalf("%s pixel (%d,%d) differs", format, x, y)
				}
			}
		}
	}
}
