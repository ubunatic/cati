package core

import (
	"image/color"
	"testing"
)

func TestNRGBAToRGBAExactMatch(t *testing.T) {
	for a := 0; a <= 255; a++ {
		for v := 0; v <= 255; v++ {
			c := color.NRGBA{R: uint8(v), G: uint8(255 - v), B: uint8(v / 2), A: uint8(a)}
			r, g, b, aa := c.RGBA()
			want := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(aa >> 8)}
			if got := NRGBAToRGBA(c.R, c.G, c.B, c.A); got != want {
				t.Fatalf("NRGBAToRGBA(%v) = %v, want %v", c, got, want)
			}
		}
	}
}
