// Package halfblock renders images into the terminal using Unicode half-block
// characters (▀ U+2580, ▄ U+2584, █ U+2588) combined with 24-bit ANSI
// true-color escape sequences.
//
// Each terminal cell encodes two vertical pixel rows:
//
//	▀  top=fg,  bot=bg
//	▄  top=bg,  bot=fg
//	█  top=fg,  bot=fg  (same color, pick either)
//	   top=bg,  bot=bg  (transparent/space)
package halfblock

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
)

// ── ANSI helpers ─────────────────────────────────────────────────────────────

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K"  // erase entire current line (any cursor column)
	ansiCarriageReturn = "\r"        // explicit CR: go to col 0 (needed in raw tty mode)
	ansiLinePrefix     = ansiEraseLine + ansiCarriageReturn
)

// fgRGB returns an ANSI 24-bit foreground escape sequence.
func fgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

// bgRGB returns an ANSI 24-bit background escape sequence.
func bgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

// ── Color helpers ─────────────────────────────────────────────────────────────

// toRGBA converts any color.Color to color.RGBA, pre-multiplying alpha.
func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA() // 16-bit, pre-multiplied
	if a == 0 {
		return color.RGBA{}
	}
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

// isTransparent returns true when a pixel is fully transparent.
func isTransparent(c color.RGBA) bool { return c.A == 0 }

// eqRGB returns true when two opaque colors have the same RGB values.
func eqRGB(a, b color.RGBA) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B
}

// ── Scaling ───────────────────────────────────────────────────────────────────

// Scale returns the image unchanged when its pixel width already fits within
// maxCols terminal columns.  Otherwise it returns a nearest-neighbour scaled
// copy whose pixel width equals maxCols (height is scaled proportionally).
//
// Each terminal column renders one pixel wide, so "fits in maxCols columns"
// means img.Bounds().Dx() <= maxCols.
func Scale(img image.Image, maxCols int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxCols <= 0 || w <= maxCols {
		return img
	}
	// Scale down proportionally; keep at least 1 row.
	newW := maxCols
	newH := h * newW / w
	if newH < 1 {
		newH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := b.Min.Y + y*h/newH
		for x := 0; x < newW; x++ {
			srcX := b.Min.X + x*w/newW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// cell describes one terminal cell derived from a top and bottom pixel.
type cell struct {
	ch         rune      // ▀, ▄, █, or ' '
	fg, bg     color.RGBA
	hasFG      bool
	hasBG      bool
	transparent bool // both pixels transparent → plain space, no ANSI
}

// pairToCell converts a (top, bottom) pixel pair into a terminal cell.
func pairToCell(top, bot color.RGBA) cell {
	topT := isTransparent(top)
	botT := isTransparent(bot)

	switch {
	case topT && botT:
		return cell{ch: ' ', transparent: true}

	case topT && !botT:
		// Only bottom pixel: ▄ with fg=bot, bg=default (no bg escape).
		return cell{ch: '▄', fg: bot, hasFG: true}

	case !topT && botT:
		// Only top pixel: ▀ with fg=top, bg=default.
		return cell{ch: '▀', fg: top, hasFG: true}

	default:
		// Both opaque.
		if eqRGB(top, bot) {
			return cell{ch: '█', fg: top, hasFG: true}
		}
		// Two distinct colors: ▀ fg=top bg=bot.
		return cell{ch: '▀', fg: top, bg: bot, hasFG: true, hasBG: true}
	}
}

// cellEscape returns the ANSI escape prefix for this cell.
func cellEscape(c cell) string {
	var b strings.Builder
	if c.hasBG {
		b.WriteString(bgRGB(c.bg))
	}
	if c.hasFG {
		b.WriteString(fgRGB(c.fg))
	}
	return b.String()
}

// Render writes the image to w as ANSI half-block art followed by a newline.
// The image should already be scaled to the desired terminal width via Scale.
// A trailing ansiReset is emitted at the end of every non-transparent row.
func Render(w io.Writer, img image.Image) error {
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()

	// Process rows in pairs (top, bottom).
	for y := b.Min.Y; y < b.Min.Y+height; y += 2 {
		topY := y
		botY := y + 1

		var sb strings.Builder

		// Erase the line and return to column 0 before writing pixels.
		// The explicit \r is required in raw terminal mode (used by --play for
		// the 'q' keypress), where \n is LF-only and does not reset the column.
		sb.WriteString(ansiLinePrefix)

		for x := b.Min.X; x < b.Min.X+width; x++ {
			top := toRGBA(img.At(x, topY))
			var bot color.RGBA
			if botY < b.Min.Y+height {
				bot = toRGBA(img.At(x, botY))
			}
			// If botY is out of range, treat as transparent.

			c := pairToCell(top, bot)
			if c.transparent {
				sb.WriteRune(' ')
			} else {
				sb.WriteString(cellEscape(c))
				sb.WriteRune(c.ch)
				sb.WriteString(ansiReset)
			}
		}

		_, err := fmt.Fprintln(w, sb.String())
		if err != nil {
			return fmt.Errorf("halfblock render: %w", err)
		}
	}
	return nil
}
