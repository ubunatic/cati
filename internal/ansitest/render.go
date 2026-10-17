// Package ansitest parses the single-cell ANSI glyph rows emitted by Cati renderers.
// It deliberately rejects unsupported terminal operations instead of hiding them.
package ansitest

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// Colour distinguishes terminal-default, palette-indexed and true RGB colours.
type Colour struct {
	Kind  string
	Value [3]uint8
}

// RGB returns a true-colour value independent of the terminal's palette.
func RGB(r, g, b uint8) Colour { return Colour{Kind: "rgb", Value: [3]uint8{r, g, b}} }

// Indexed returns a terminal palette entry.
func Indexed(index uint8) Colour { return Colour{Kind: "index", Value: [3]uint8{index}} }

// Cell is one single-width renderer glyph and the SGR state in effect at that cell.
type Cell struct {
	Rune       rune
	FG, BG     Colour
	Attributes uint16
}

// Parse returns rendered rows, including blank rows, without a phantom trailing row.
// It handles SGR, carriage return, line erasure and initial cursor-home sequences.
// Cati's mouse-reporting and cursor-visibility toggles do not paint cells.
// Glyphs follow Cati's single-cell alphabet; this is not a general terminal emulator.
func Parse(input string) ([][]Cell, error) {
	var rows [][]Cell
	var row []Cell
	style := Cell{}
	col := 0
	for i := 0; i < len(input); {
		switch input[i] {
		case '\x1b':
			start := i
			if i+1 >= len(input) || input[i+1] != '[' {
				return nil, fmt.Errorf("non-CSI escape at byte %d", i)
			}
			i += 2
			paramsStart := i
			for i < len(input) && (input[i] < 0x40 || input[i] > 0x7e) {
				i++
			}
			if i == len(input) {
				return nil, fmt.Errorf("unterminated CSI at byte %d", start)
			}
			params, final := input[paramsStart:i], input[i]
			i++
			switch final {
			case 'm':
				if err := applySGR(&style, params); err != nil {
					return nil, fmt.Errorf("SGR at byte %d: %w", start, err)
				}
			case 'K':
				switch params {
				case "", "0":
					row = row[:min(col, len(row))]
				case "2":
					row = nil
				default:
					return nil, fmt.Errorf("unsupported erase-line %q", params)
				}
			case 'H':
				if (params != "" && params != "1;1") || len(rows) != 0 || len(row) != 0 {
					return nil, fmt.Errorf("only initial cursor home is supported")
				}
				col = 0
			case 'h', 'l':
				switch params {
				case "?25", "?1002", "?1003", "?1006":
					// Cursor visibility and mouse input modes preserve cell state.
				default:
					return nil, fmt.Errorf("unsupported terminal mode %q", params)
				}
			default:
				return nil, fmt.Errorf("unsupported CSI %q at byte %d", input[start:i], start)
			}
		case '\r':
			col = 0
			i++
		case '\n':
			rows = append(rows, row)
			row = nil
			col = 0
			i++
		default:
			r, size := utf8.DecodeRuneInString(input[i:])
			if r == utf8.RuneError && size == 1 {
				return nil, fmt.Errorf("invalid UTF-8 at byte %d", i)
			}
			if r < 0x20 || r == 0x7f {
				return nil, fmt.Errorf("unsupported control at byte %d", i)
			}
			cell := style
			cell.Rune = r
			for len(row) < col {
				row = append(row, Cell{Rune: ' '})
			}
			if col < len(row) {
				row[col] = cell
			} else {
				row = append(row, cell)
			}
			col++
			i += size
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows, nil
}

func applySGR(style *Cell, params string) error {
	parts := strings.Split(params, ";")
	codes := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid parameter %q", part)
		}
		codes[i] = n
	}
	for i := 0; i < len(codes); i++ {
		code := codes[i]
		switch {
		case code == 0:
			*style = Cell{}
		case code == 1 || code == 2 || code == 3 || code == 4 || code == 7 || code == 9:
			style.Attributes |= 1 << code
		case code == 22:
			style.Attributes &^= 1<<1 | 1<<2
		case code == 23:
			style.Attributes &^= 1 << 3
		case code == 24:
			style.Attributes &^= 1 << 4
		case code == 27:
			style.Attributes &^= 1 << 7
		case code == 29:
			style.Attributes &^= 1 << 9
		case code == 39:
			style.FG = Colour{}
		case code == 49:
			style.BG = Colour{}
		case code >= 30 && code <= 37:
			style.FG = Indexed(uint8(code - 30))
		case code >= 40 && code <= 47:
			style.BG = Indexed(uint8(code - 40))
		case code >= 90 && code <= 97:
			style.FG = Indexed(uint8(code - 90 + 8))
		case code >= 100 && code <= 107:
			style.BG = Indexed(uint8(code - 100 + 8))
		case code == 38 || code == 48:
			if i+1 >= len(codes) {
				return fmt.Errorf("missing extended colour mode")
			}
			var colour Colour
			switch codes[i+1] {
			case 2:
				if i+4 >= len(codes) {
					return fmt.Errorf("missing RGB channels")
				}
				for _, n := range codes[i+2 : i+5] {
					if n > 255 {
						return fmt.Errorf("RGB channel out of range: %d", n)
					}
				}
				colour = RGB(uint8(codes[i+2]), uint8(codes[i+3]), uint8(codes[i+4]))
				i += 4
			case 5:
				if i+2 >= len(codes) || codes[i+2] > 255 {
					return fmt.Errorf("missing or invalid palette index")
				}
				colour = Indexed(uint8(codes[i+2]))
				i += 2
			default:
				return fmt.Errorf("unsupported extended colour mode: %d", codes[i+1])
			}
			if code == 38 {
				style.FG = colour
			} else {
				style.BG = colour
			}
		default:
			return fmt.Errorf("unsupported SGR: %d", code)
		}
	}
	return nil
}

// AssertEqual compares row/cell counts, glyphs, colours and SGR attributes.
// Equivalent escape encodings compare equal; colour changes remain observable.
func AssertEqual(t testing.TB, got, want string) {
	t.Helper()
	gotRows, err := Parse(got)
	if err != nil {
		t.Fatalf("parse actual ANSI: %v", err)
	}
	wantRows, err := Parse(want)
	if err != nil {
		t.Fatalf("parse expected ANSI: %v", err)
	}
	if len(gotRows) != len(wantRows) {
		t.Fatalf("ANSI rows = %d, want %d", len(gotRows), len(wantRows))
	}
	for y := range wantRows {
		if len(gotRows[y]) != len(wantRows[y]) {
			t.Fatalf("ANSI row %d has %d cells, want %d", y, len(gotRows[y]), len(wantRows[y]))
		}
		for x := range wantRows[y] {
			if gotRows[y][x] != wantRows[y][x] {
				t.Errorf("ANSI cell (%d,%d) = %+v, want %+v", x, y, gotRows[y][x], wantRows[y][x])
			}
		}
	}
}
