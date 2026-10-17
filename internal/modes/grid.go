// Package modes formats render-mode demo cards independently of the CLI.
package modes

import (
	"strings"
	"ubunatic.com/cati/internal/ansi"
)

// AutoFitGridCols fits up to three card columns into the terminal width.
func AutoFitGridCols(cards [][]string, termWidth int) int {
	if termWidth <= 0 || len(cards) == 0 {
		return 1
	}
	maxCardW := 0
	for _, card := range cards {
		for _, line := range card {
			if w := ansi.FirstLineWidth(line); w > maxCardW {
				maxCardW = w
			}
		}
	}
	if maxCardW <= 0 {
		return 1
	}
	const gutter = 4
	for k := 3; k >= 1; k-- {
		needed := k*maxCardW + (k-1)*gutter
		if needed <= termWidth {
			return k
		}
	}
	return 1
}

func formatGridRowLines(rowCards [][]string) []string {
	if len(rowCards) == 0 {
		return nil
	}
	colWidths := make([]int, len(rowCards))
	maxH := 0
	for j, card := range rowCards {
		w := 0
		for _, line := range card {
			if lw := ansi.FirstLineWidth(line); lw > w {
				w = lw
			}
		}
		colWidths[j] = w
		if len(card) > maxH {
			maxH = len(card)
		}
	}

	lines := make([]string, 0, maxH+1)
	lines = append(lines, "") // blank line before row
	for r := 0; r < maxH; r++ {
		var sb strings.Builder
		for j, card := range rowCards {
			var line string
			if r < len(card) {
				line = card[r]
			}
			if j < len(rowCards)-1 {
				sb.WriteString(ansi.PadLine(line, colWidths[j]))
				sb.WriteString("    ")
			} else {
				sb.WriteString(line)
			}
		}
		lines = append(lines, strings.TrimRight(sb.String(), " "))
	}
	return lines
}

// FormatCardsGrid arranges cards with a four-cell gutter and right padding.
func FormatCardsGrid(cards [][]string, cols int, termWidth int) []string {
	if len(cards) == 0 {
		return nil
	}
	effectiveCols := cols
	if effectiveCols <= 0 {
		effectiveCols = AutoFitGridCols(cards, termWidth)
	}
	if effectiveCols <= 0 {
		effectiveCols = 1
	}

	var result []string
	for i := 0; i < len(cards); i += effectiveCols {
		end := i + effectiveCols
		if end > len(cards) {
			end = len(cards)
		}
		result = append(result, formatGridRowLines(cards[i:end])...)
	}
	return result
}

// WrapGlyphs wraps a glyph inventory, showing spaces with a visible marker.
func WrapGlyphs(shapes []rune, width, prefixWidth int) string {
	if len(shapes) == 0 {
		return "(none)"
	}
	var b strings.Builder
	lineWidth := prefixWidth
	for i, r := range shapes {
		glyphWidth := 1
		if i > 0 {
			glyphWidth++
		}
		if lineWidth+glyphWidth > width && b.Len() > 0 {
			b.WriteString("\n")
			b.WriteString(strings.Repeat(" ", prefixWidth))
			lineWidth = prefixWidth
			glyphWidth = 1
		}
		if i > 0 && lineWidth > prefixWidth {
			b.WriteByte(' ')
		}
		if r == ' ' {
			b.WriteRune('␠')
		} else {
			b.WriteRune(r)
		}
		lineWidth += glyphWidth
	}
	return b.String()
}
