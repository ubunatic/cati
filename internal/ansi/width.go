// Package ansi handles Cati's ANSI output and single-cell renderer glyphs.
package ansi

import (
	"strings"
	"unicode/utf8"
)

// LineWidths counts single-cell renderer glyphs per row, ignoring ANSI escapes.
// It preserves the renderer's carriage-return policy; it is not a terminal emulator.
func LineWidths(out string) []int {
	var widths []int
	col := 0
	for i := 0; i < len(out); {
		if out[i] == '\x1b' {
			i = skipANSI(out, i)
			continue
		}
		switch out[i] {
		case '\r':
			col = 0
			i++
			continue
		case '\n':
			widths = append(widths, col)
			col = 0
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(out[i:])
		if size == 0 {
			break
		}
		col++
		i += size
	}
	if col > 0 {
		widths = append(widths, col)
	}
	return widths
}
func skipANSI(out string, i int) int {
	i++
	if i >= len(out) || out[i] != '[' {
		return i
	}
	i++
	for i < len(out) {
		c := out[i]
		i++
		if c >= 0x40 && c <= 0x7e {
			break
		}
	}
	return i
}

// FirstLineWidth returns the width of the first rendered row.
func FirstLineWidth(line string) int {
	widths := LineWidths(line)
	if len(widths) == 0 {
		return 0
	}
	return widths[0]
}

// PadLine appends spaces until a single-cell renderer row reaches width.
func PadLine(line string, width int) string {
	return line + strings.Repeat(" ", max(0, width-FirstLineWidth(line)))
}
