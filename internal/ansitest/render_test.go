package ansitest

import (
	"reflect"
	"testing"
)

func TestEquivalentSGREncodings(t *testing.T) {
	t.Parallel()
	separate := "\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀▀\x1b[m\n"
	combined := "\x1b[2K\r\x1b[38;2;255;0;0;48;2;0;0;255m▀\x1b[38;2;255;0;0m▀\x1b[0m\n"
	AssertEqual(t, separate, combined)
	want := [][]Cell{{
		{Rune: '▀', FG: RGB(255, 0, 0), BG: RGB(0, 0, 255)},
		{Rune: '▀', FG: RGB(255, 0, 0), BG: RGB(0, 0, 255)},
	}}
	got, err := Parse(combined)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("cells = %+v, %v; want %+v", got, err, want)
	}
	AssertEqual(t, "\x1b[31mX", "\x1b[38;5;1mX")
	AssertEqual(t, "\x1b[91;104mX", "\x1b[38;5;9;48;5;12mX")
}

func TestResetAndColourChangesRemainObservable(t *testing.T) {
	t.Parallel()
	rows, err := Parse("\x1b[31;44mA\x1b[39mB\x1b[49mC\n\x1b[1mD\x1b[22mE\x1b[2mF\x1b[mG\n")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]Cell{
		{{Rune: 'A', FG: Indexed(1), BG: Indexed(4)}, {Rune: 'B', BG: Indexed(4)}, {Rune: 'C'}},
		{{Rune: 'D', Attributes: 1 << 1}, {Rune: 'E'}, {Rune: 'F', Attributes: 1 << 2}, {Rune: 'G'}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("reset rows = %+v, want %+v", rows, want)
	}
	red, _ := Parse("\x1b[31mX")
	blue, _ := Parse("\x1b[34mX")
	if reflect.DeepEqual(red, blue) {
		t.Fatal("colour change was hidden")
	}
}

func TestAttributeResetsAndStyleAcrossRows(t *testing.T) {
	t.Parallel()
	rows, err := Parse("\x1b[3;4;7;9;32mA\nB\x1b[23;24;27;29mC\x1b[0mD")
	if err != nil {
		t.Fatal(err)
	}
	styled := Cell{Rune: 'A', FG: Indexed(2), Attributes: 1<<3 | 1<<4 | 1<<7 | 1<<9}
	nextRow := styled
	nextRow.Rune = 'B'
	want := [][]Cell{{styled}, {nextRow, {Rune: 'C', FG: Indexed(2)}, {Rune: 'D'}}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("attribute/row states = %+v, want %+v", rows, want)
	}
}

func TestRowsErasureAndCarriageReturn(t *testing.T) {
	t.Parallel()
	got, err := Parse("\x1b[HAB\rC\n\nold\r\x1b[2KX\n")
	want := [][]Cell{{{Rune: 'C'}, {Rune: 'B'}}, nil, {{Rune: 'X'}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %+v, %v; want %+v", got, err, want)
	}
	got, err = Parse("AB\rX\x1b[K")
	if err != nil || len(got) != 1 || len(got[0]) != 1 || got[0][0].Rune != 'X' {
		t.Fatalf("erase to end = %+v, %v", got, err)
	}
}

func TestNonPaintingViewerSetupPreservesCells(t *testing.T) {
	t.Parallel()
	AssertEqual(t, "\x1b[H\x1b[?1002h\x1b[?1006h\x1b[?25l\x1b[31mX\n\x1b[?1002l\x1b[?1003h\x1b[?25hY", "\x1b[31mX\nY")
}

func TestMalformedOrUnsupportedOutputFails(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"\x1b", "\x1b[38;2;1;2mX", "\x1b[38;2;300;0;0mX", "\x1b[48;5;256mX",
		"\x1b[38;1mX", "\x1b[999mX", "\x1b[?1049h", "\x1b[2;1HX", "\x1b[31", "\x1b[1K", "\t", "\xff",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatalf("accepted unsupported output %q", input)
			}
		})
	}
}
