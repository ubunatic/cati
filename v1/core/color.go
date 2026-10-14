package core

import "image/color"

// NRGBAToRGBA premultiplies one non-premultiplied pixel, bit-identical to
// converting color.NRGBA{r, g, b, a}.RGBA() down to 8 bits. Fastpaths use it to
// read *image.NRGBA pixels without the color.Color interface.
func NRGBAToRGBA(r, g, b, a uint8) color.RGBA {
	switch a {
	case 0:
		return color.RGBA{}
	case 255:
		return color.RGBA{R: r, G: g, B: b, A: 255}
	}
	a16 := uint32(a) * 257
	return color.RGBA{
		R: uint8((uint32(r) * 257 * a16 / 65535) >> 8),
		G: uint8((uint32(g) * 257 * a16 / 65535) >> 8),
		B: uint8((uint32(b) * 257 * a16 / 65535) >> 8),
		A: a,
	}
}
