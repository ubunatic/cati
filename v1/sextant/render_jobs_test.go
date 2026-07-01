package sextant

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRenderJMatchesSerial(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 161, 69))
	for y := 0; y < 69; y++ {
		for x := 0; x < 161; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 17) % 256),
				G: uint8((y * 31) % 256),
				B: uint8((x*3 + y*5) % 256),
				A: 255,
			})
		}
	}

	var serial, parallel strings.Builder
	if err := Render(&serial, img, 0, Options{Mode: ModeSextant}); err != nil {
		t.Fatalf("Render serial: %v", err)
	}
	if err := RenderJ(&parallel, img, ModeSextant, 10); err != nil {
		t.Fatalf("RenderJ parallel: %v", err)
	}
	if serial.String() != parallel.String() {
		t.Fatalf("RenderJ output differs\nserial:   %q\nparallel: %q", serial.String(), parallel.String())
	}
}

func TestRenderJVisibleLineWidthsStayEqual(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 160, 69))
	for y := 0; y < 69; y++ {
		for x := 0; x < 160; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / 159),
				G: uint8(y * 255 / 68),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	var out strings.Builder
	if err := RenderJ(&out, img, ModeSextant, 10); err != nil {
		t.Fatalf("RenderJ: %v", err)
	}
	widths := visibleWidths(out.String())
	if len(widths) == 0 {
		t.Fatal("RenderJ emitted no rows")
	}
	for row, width := range widths {
		if width != 80 {
			t.Fatalf("row %d width = %d, want 80 (%v)", row, width, widths)
		}
	}
}

func visibleWidths(out string) []int {
	var widths []int
	col := 0
	for i := 0; i < len(out); {
		if out[i] == '\x1b' {
			i = skipCSI(out, i)
			continue
		}
		switch out[i] {
		case '\r':
			col = 0
			i++
		case '\n':
			widths = append(widths, col)
			col = 0
			i++
		default:
			_, size := utf8.DecodeRuneInString(out[i:])
			col++
			i += size
		}
	}
	if col > 0 {
		widths = append(widths, col)
	}
	return widths
}

func skipCSI(out string, i int) int {
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
