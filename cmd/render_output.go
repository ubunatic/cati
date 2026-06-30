package cmd

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"unicode/utf8"
)

func renderChecked(w io.Writer, vp image.Image, rc renderCfg) error {
	var buf bytes.Buffer
	if err := rc.render(&buf, vp); err != nil {
		return err
	}
	if err := validateRenderedANSI(buf.String(), renderedCellSize(vp, rc), rcModeName(rc)); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

func validateRenderedANSI(out string, want renderCells, modeName string) error {
	if want.Cols <= 0 || want.Rows <= 0 || out == "" {
		return nil
	}
	widths := visibleLineWidths(out)
	if len(widths) != want.Rows {
		return fmt.Errorf("render line count mismatch for %s: got %d rows, want %d", modeName, len(widths), want.Rows)
	}
	for row, got := range widths {
		if got != want.Cols {
			return fmt.Errorf("render line width mismatch for %s: row %d has %d cells, want %d", modeName, row, got, want.Cols)
		}
	}
	return nil
}

func visibleLineWidths(out string) []int {
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
