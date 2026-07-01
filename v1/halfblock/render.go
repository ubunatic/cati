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
	"runtime"
	"strings"
	"sync"

	"codeberg.org/ubunatic/cati/v1/core"
)

// ── ANSI helpers ─────────────────────────────────────────────────────────────

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K" // erase entire current line (any cursor column)
	ansiCarriageReturn = "\r"      // explicit CR: go to col 0 (needed in raw tty mode)
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

// ScaleToFit scales img to fit within the given terminal dimensions while
// always preserving the original aspect ratio.
//
// cols and rows are in terminal characters; because each character cell encodes
// two pixel rows, the effective pixel height budget is rows*2.
//
// Rules:
//   - cols <= 0  →  no width constraint.
//   - rows <= 0  →  no height constraint.
//   - both <= 0  →  image is returned unchanged.
//
// When both constraints are active the tighter one (the one that results in the
// smaller image) wins, so the output always fits inside the requested box.
// The image is never upscaled; if it already fits it is returned as-is.
func ScaleToFit(img image.Image, cols, rows int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return img
	}

	// Convert terminal rows to pixel rows (each cell = 2 pixel rows).
	pixelH := rows * 2

	// Compute candidate target dimensions from each active constraint.
	newW, newH := srcW, srcH // start with original (= no scaling)

	if cols > 0 && cols < newW {
		// Width-constrained candidate.
		newW = cols
		newH = srcH * newW / srcW
	}
	if pixelH > 0 && srcH*newW/srcW > pixelH {
		// Height-constrained candidate (may tighten the width-constrained result).
		newH = pixelH
		newW = srcW * newH / srcH
	}

	if newH < 1 {
		newH = 1
	}
	if newW < 1 {
		newW = 1
	}

	// No change needed.
	if newW == srcW && newH == srcH {
		return img
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := b.Min.Y + y*srcH/newH
		for x := 0; x < newW; x++ {
			srcX := b.Min.X + x*srcW/newW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

// Scale returns the image unchanged when its pixel width already fits within
// maxCols terminal columns.  Otherwise it returns a nearest-neighbour scaled
// copy whose pixel width equals maxCols (height is scaled proportionally).
//
// Deprecated: prefer ScaleToFit which also supports a height constraint.
func Scale(img image.Image, maxCols int) image.Image {
	return ScaleToFit(img, maxCols, 0)
}

// ScaleNN returns a nearest-neighbour scaled copy of img at exactly w×h pixels.
// Unlike ScaleToFit it also upscales, so it is suitable for zoom > 1.
func ScaleNN(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 || w < 1 || h < 1 {
		return img
	}
	if srcW == w && srcH == h {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := b.Min.Y + y*srcH/h
		for x := 0; x < w; x++ {
			srcX := b.Min.X + x*srcW/w
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// cell describes one terminal cell derived from a top and bottom pixel.
type cell struct {
	ch          rune // ▀, ▄, █, or ' '
	fg, bg      color.RGBA
	hasFG       bool
	hasBG       bool
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

// Options configures the half-block renderer options.
type Options struct {
	Rows int
	Jobs int
}

// RenderToGrid scales and converts the image into a grid of terminal cells.
func RenderToGrid(img image.Image, cols int, opts Options) (*core.Grid, error) {
	var scaled image.Image
	if cols > 0 || opts.Rows > 0 {
		scaled = ScaleToFit(img, cols, opts.Rows)
	} else {
		scaled = img
	}

	b := scaled.Bounds()
	width := b.Dx()
	height := b.Dy()

	// Process rows in pairs (top, bottom).
	rowCount := (height + 1) / 2
	if rowCount <= 0 {
		return &core.Grid{Cells: [][]core.Cell{}}, nil
	}

	cells := make([][]core.Cell, rowCount)
	for i := range cells {
		cells[i] = make([]core.Cell, width)
	}

	renderRowFunc := func(row int) {
		topY := b.Min.Y + row*2
		botY := topY + 1
		for x := 0; x < width; x++ {
			srcX := b.Min.X + x
			top := toRGBA(scaled.At(srcX, topY))
			var bot color.RGBA
			if botY < b.Min.Y+height {
				bot = toRGBA(scaled.At(srcX, botY))
			}
			c := pairToCell(top, bot)
			cells[row][x] = core.Cell{
				Ch:          c.ch,
				Fg:          c.fg,
				Bg:          c.bg,
				HasFg:       c.hasFG,
				HasBg:       c.hasBG,
				Transparent: c.transparent,
			}
		}
	}

	jobs := opts.Jobs
	if jobs <= 1 {
		for row := 0; row < rowCount; row++ {
			renderRowFunc(row)
		}
	} else {
		var wg sync.WaitGroup
		jobsCh := make(chan int)
		workerN := jobs
		if workerN > rowCount {
			workerN = rowCount
		}
		if workerN > runtime.NumCPU() {
			workerN = runtime.NumCPU()
		}
		for range workerN {
			go func() {
				for row := range jobsCh {
					renderRowFunc(row)
					wg.Done()
				}
			}()
		}
		for row := 0; row < rowCount; row++ {
			wg.Add(1)
			jobsCh <- row
		}
		close(jobsCh)
		wg.Wait()
	}

	return &core.Grid{
		Width:  width,
		Height: rowCount,
		Cells:  cells,
	}, nil
}

// Render writes the image to w as ANSI half-block art followed by a newline.
func Render(w io.Writer, img image.Image, cols int, opts Options) error {
	grid, err := RenderToGrid(img, cols, opts)
	if err != nil {
		return err
	}

	for y := 0; y < grid.Height; y++ {
		var sb strings.Builder
		sb.WriteString(ansiLinePrefix)
		for x := 0; x < grid.Width; x++ {
			c := grid.Cells[y][x]
			if c.Transparent {
				sb.WriteRune(' ')
			} else {
				sb.WriteString(cellEscape(c))
				sb.WriteRune(c.Ch)
				sb.WriteString(ansiReset)
			}
		}
		if _, err := fmt.Fprintln(w, sb.String()); err != nil {
			return fmt.Errorf("halfblock render: %w", err)
		}
	}
	return nil
}

// cellEscape returns the ANSI escape prefix for this cell.
func cellEscape(c core.Cell) string {
	var b strings.Builder
	if c.HasBg {
		b.WriteString(bgRGB(c.Bg))
	}
	if c.HasFg {
		b.WriteString(fgRGB(c.Fg))
	}
	return b.String()
}

// RenderToImage renders the image using the halfblock algorithm and returns a new image.
func RenderToImage(img image.Image) *image.RGBA {
	return RenderToImageJ(img, 1)
}

// RenderToImageJ is a worker-aware copy of RenderToImage.
func RenderToImageJ(img image.Image, jobs int) *image.RGBA {
	b := img.Bounds()
	grid, _ := RenderToGrid(img, b.Dx(), Options{Jobs: jobs})
	dst := image.NewRGBA(b)

	for y := 0; y < grid.Height; y++ {
		topY := b.Min.Y + y*2
		botY := topY + 1
		for x := 0; x < grid.Width; x++ {
			c := grid.Cells[y][x]
			var topColor color.RGBA
			var botColor color.RGBA

			if c.Transparent {
				topColor = color.RGBA{}
				botColor = color.RGBA{}
			} else {
				bg := c.Bg
				if !c.HasBg {
					bg = color.RGBA{}
				}
				fg := c.Fg
				if !c.HasFg {
					fg = bg
				}

				switch c.Ch {
				case '▀':
					topColor = fg
					botColor = bg
				case '▄':
					topColor = bg
					botColor = fg
				case '█':
					topColor = fg
					botColor = fg
				default:
					topColor = bg
					botColor = bg
				}
			}

			dst.SetRGBA(b.Min.X+x, topY, topColor)
			if botY < b.Max.Y {
				dst.SetRGBA(b.Min.X+x, botY, botColor)
			}
		}
	}
	return dst
}

// RenderJ is kept for testing compatibility.
func RenderJ(w io.Writer, img image.Image, jobs int) error {
	b := img.Bounds()
	return Render(w, img, b.Dx(), Options{Jobs: jobs})
}
