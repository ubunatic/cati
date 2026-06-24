package quadblock

import (
	"image"
	"image/color"
	"math"
)

// ColorReduction selects the colour palette applied before quad rendering.
// Apply via ReduceColors(img, cr) before calling ScaleToFit / RenderOpts.
type ColorReduction int

const (
	// ColorFull keeps all 24-bit colours (no reduction).
	ColorFull ColorReduction = iota
	// ColorANSI256 snaps to the 256-colour ANSI xterm palette
	// (16 basic + 6×6×6 colour cube + 24 grays).
	ColorANSI256
	// ColorANSI16 snaps to the 16 basic ANSI terminal colours.
	ColorANSI16
	// ColorGray8 converts to 8-level grayscale.
	ColorGray8
	// ColorGray16 converts to 16-level grayscale.
	ColorGray16
	// ColorGray64 converts to 64-level grayscale.
	ColorGray64
)

// ReduceColors returns a copy of img with every opaque pixel snapped to the
// nearest colour in the given palette.  Transparent pixels are preserved.
// Returns img unchanged when cr is ColorFull.
func ReduceColors(img image.Image, cr ColorReduction) image.Image {
	if cr == ColorFull {
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)

	var mapFn func(color.RGBA) color.RGBA
	switch cr {
	case ColorANSI256:
		pal := ansi256Palette()
		mapFn = func(c color.RGBA) color.RGBA { return nearestInPalette(c, pal) }
	case ColorANSI16:
		pal := ansi16Palette()
		mapFn = func(c color.RGBA) color.RGBA { return nearestInPalette(c, pal) }
	case ColorGray8:
		mapFn = func(c color.RGBA) color.RGBA { return toGray(c, 8) }
	case ColorGray16:
		mapFn = func(c color.RGBA) color.RGBA { return toGray(c, 16) }
	case ColorGray64:
		mapFn = func(c color.RGBA) color.RGBA { return toGray(c, 64) }
	default:
		return img
	}

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := toRGBA(img.At(x, y))
			if isTransparent(c) {
				dst.Set(x, y, color.RGBA{})
			} else {
				dst.Set(x, y, mapFn(c))
			}
		}
	}
	return dst
}

// ── Palette helpers ───────────────────────────────────────────────────────────

// nearestInPalette returns the palette entry with the smallest squared
// Euclidean distance from c in RGB space.
func nearestInPalette(c color.RGBA, palette []color.RGBA) color.RGBA {
	best := palette[0]
	bestD := colorDist2(c, best)
	for _, p := range palette[1:] {
		if d := colorDist2(c, p); d < bestD {
			best = p
			bestD = d
		}
	}
	return best
}

// toGray converts c to a grayscale value quantised to `levels` discrete steps.
func toGray(c color.RGBA, levels int) color.RGBA {
	lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
	step := 255.0 / float64(levels-1)
	v := uint8(math.Round(lum/step) * step)
	return color.RGBA{R: v, G: v, B: v, A: c.A}
}

// ── Palette definitions ───────────────────────────────────────────────────────

// ansi256Palette returns the 256-colour ANSI/xterm palette:
//   - colours 0-15:   16 basic colours (xterm approximations)
//   - colours 16-231: 6×6×6 colour cube (value[0]=0, value[n]=55+40n for n≥1)
//   - colours 232-255: 24 grayscale steps (8 + 10n for n=0..23)
func ansi256Palette() []color.RGBA {
	var pal []color.RGBA

	// 16 basic colours (xterm default approximations)
	pal = append(pal, ansi16Palette()...)

	// 6×6×6 colour cube (indices 16-231)
	cubeLevel := [6]uint8{0, 95, 135, 175, 215, 255}
	for r := range 6 {
		for g := range 6 {
			for b := range 6 {
				pal = append(pal, color.RGBA{
					R: cubeLevel[r], G: cubeLevel[g], B: cubeLevel[b], A: 255,
				})
			}
		}
	}

	// 24 grayscale steps (indices 232-255)
	for n := range 24 {
		v := uint8(8 + 10*n)
		pal = append(pal, color.RGBA{R: v, G: v, B: v, A: 255})
	}

	return pal
}

// ansi16Palette returns the 16 basic ANSI terminal colours.
func ansi16Palette() []color.RGBA {
	return []color.RGBA{
		{0, 0, 0, 255},       // 0  black
		{128, 0, 0, 255},     // 1  dark red
		{0, 128, 0, 255},     // 2  dark green
		{128, 128, 0, 255},   // 3  dark yellow
		{0, 0, 128, 255},     // 4  dark blue
		{128, 0, 128, 255},   // 5  dark magenta
		{0, 128, 128, 255},   // 6  dark cyan
		{192, 192, 192, 255}, // 7  light gray
		{128, 128, 128, 255}, // 8  dark gray
		{255, 0, 0, 255},     // 9  red
		{0, 255, 0, 255},     // 10 green
		{255, 255, 0, 255},   // 11 yellow
		{0, 0, 255, 255},     // 12 blue
		{255, 0, 255, 255},   // 13 magenta
		{0, 255, 255, 255},   // 14 cyan
		{255, 255, 255, 255}, // 15 white
	}
}
