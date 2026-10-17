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
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"ubunatic.com/cati/v1/core"
)

// ── ANSI helpers ─────────────────────────────────────────────────────────────

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K" // erase entire current line (any cursor column)
	ansiCarriageReturn = "\r"      // explicit CR: go to col 0 (needed in raw tty mode)
	ansiLinePrefix     = ansiEraseLine + ansiCarriageReturn
)

func appendFgRGB(b []byte, c color.RGBA) []byte {
	b = append(b, "\x1b[38;2;"...)
	b = strconv.AppendUint(b, uint64(c.R), 10)
	b = append(b, ';')
	b = strconv.AppendUint(b, uint64(c.G), 10)
	b = append(b, ';')
	b = strconv.AppendUint(b, uint64(c.B), 10)
	return append(b, 'm')
}

func appendBgRGB(b []byte, c color.RGBA) []byte {
	b = append(b, "\x1b[48;2;"...)
	b = strconv.AppendUint(b, uint64(c.R), 10)
	b = append(b, ';')
	b = strconv.AppendUint(b, uint64(c.G), 10)
	b = append(b, ';')
	b = strconv.AppendUint(b, uint64(c.B), 10)
	return append(b, 'm')
}

// fgRGB returns an ANSI 24-bit foreground escape sequence.
func fgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

// bgRGB returns an ANSI 24-bit background escape sequence.
func bgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

// The byte readers are shared by grid construction and direct reconstruction.
// Keep them small so the row loops can inline them without indirect calls.
func rgbaPixel(pix []byte, off int) color.RGBA {
	if off < 0 || off+3 >= len(pix) || pix[off+3] == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: pix[off], G: pix[off+1], B: pix[off+2], A: pix[off+3]}
}

func nrgbaPixel(pix []byte, off int) color.RGBA {
	if off < 0 || off+3 >= len(pix) {
		return color.RGBA{}
	}
	return core.NRGBAToRGBA(pix[off], pix[off+1], pix[off+2], pix[off+3])
}

// Reconstruction of pairToCell is the premultiplied source RGB with alpha
// forced to 255 for painted pixels; an unpainted pixel remains all zero.
// ANSI has no fractional alpha. This lets the fast path bypass the grid.
func storeOpaquePixel(pix []byte, off int, c color.RGBA) {
	if c.A == 0 {
		c = color.RGBA{}
	} else {
		c.A = 255
	}
	p := pix[off : off+4 : off+4]
	p[0], p[1], p[2], p[3] = c.R, c.G, c.B, c.A
}

func safePixel(img image.Image, x, y int) color.RGBA {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return color.RGBA{}
	}
	if core.Fastpath {
		if rgba, ok := img.(*image.RGBA); ok {
			off := (y-rgba.Rect.Min.Y)*rgba.Stride + (x-rgba.Rect.Min.X)*4
			if off >= 0 && off+3 < len(rgba.Pix) {
				return rgbaPixel(rgba.Pix, off)
			}
		}
	}
	return toRGBA(img.At(x, y))
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
	if core.Fastpath {
		if srcRGBA, ok := img.(*image.RGBA); ok {
			for y := 0; y < newH; y++ {
				srcY := b.Min.Y + y*srcH/newH
				dstRowOff := y * dst.Stride
				srcRowOff := (srcY - srcRGBA.Rect.Min.Y) * srcRGBA.Stride
				for x := 0; x < newW; x++ {
					srcX := b.Min.X + x*srcW/newW
					sOff := srcRowOff + (srcX-srcRGBA.Rect.Min.X)*4
					dOff := dstRowOff + x*4
					dPix := dst.Pix[dOff : dOff+4 : dOff+4]
					sPix := srcRGBA.Pix[sOff : sOff+4 : sOff+4]
					dPix[0] = sPix[0]
					dPix[1] = sPix[1]
					dPix[2] = sPix[2]
					dPix[3] = sPix[3]
				}
			}
			return dst
		}
	}
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
	if core.Fastpath {
		if srcRGBA, ok := img.(*image.RGBA); ok {
			for y := 0; y < h; y++ {
				srcY := b.Min.Y + y*srcH/h
				dstRowOff := y * dst.Stride
				srcRowOff := (srcY - srcRGBA.Rect.Min.Y) * srcRGBA.Stride
				for x := 0; x < w; x++ {
					srcX := b.Min.X + x*srcW/w
					sOff := srcRowOff + (srcX-srcRGBA.Rect.Min.X)*4
					dOff := dstRowOff + x*4
					dPix := dst.Pix[dOff : dOff+4 : dOff+4]
					sPix := srcRGBA.Pix[sOff : sOff+4 : sOff+4]
					dPix[0] = sPix[0]
					dPix[1] = sPix[1]
					dPix[2] = sPix[2]
					dPix[3] = sPix[3]
				}
			}
			return dst
		}
	}
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

func (c cell) gridCell() core.Cell {
	return core.Cell{
		Ch: c.ch, Fg: c.fg, Bg: c.bg,
		HasFg: c.hasFG, HasBg: c.hasBG, Transparent: c.transparent,
	}
}

// Options configures the half-block renderer options.
type Options struct {
	Rows int
	Jobs int
	// OnProgress is called as rendered rows complete. Parallel rendering may
	// invoke it concurrently; callbacks should be quick and concurrency-safe.
	OnProgress func(core.Progress)

	// NoLinePrefix omits the erase-line and carriage-return prefix emitted
	// before each rendered line. Set this when composing output alongside
	// other content on the same terminal row.
	NoLinePrefix bool
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
	progress := core.NewProgressReporter(opts.OnProgress, rowCount)
	if rowCount <= 0 {
		return &core.Grid{Cells: [][]core.Cell{}}, nil
	}

	cells := make([][]core.Cell, rowCount)
	cellStorage := make([]core.Cell, rowCount*width)
	for i := range cells {
		cells[i] = cellStorage[i*width : (i+1)*width]
	}

	renderRowFunc := func(row int) {
		topY := b.Min.Y + row*2
		botY := topY + 1
		hasBot := botY < b.Min.Y+height
		rowCells := cells[row]

		if core.Fastpath {
			if rgba, ok := scaled.(*image.RGBA); ok {
				topRowOff := (topY - rgba.Rect.Min.Y) * rgba.Stride
				botRowOff := (botY - rgba.Rect.Min.Y) * rgba.Stride
				pix := rgba.Pix
				minX := b.Min.X - rgba.Rect.Min.X
				for x := 0; x < width; x++ {
					topOff := topRowOff + (minX+x)*4
					top := rgbaPixel(pix, topOff)
					var bot color.RGBA
					if hasBot {
						botOff := botRowOff + (minX+x)*4
						bot = rgbaPixel(pix, botOff)
					}
					rowCells[x] = pairToCell(top, bot).gridCell()
				}
				return
			}
			if nrgba, ok := scaled.(*image.NRGBA); ok {
				topRowOff := (topY - nrgba.Rect.Min.Y) * nrgba.Stride
				botRowOff := (botY - nrgba.Rect.Min.Y) * nrgba.Stride
				pix := nrgba.Pix
				minX := b.Min.X - nrgba.Rect.Min.X
				for x := 0; x < width; x++ {
					topOff := topRowOff + (minX+x)*4
					top := nrgbaPixel(pix, topOff)
					var bot color.RGBA
					if hasBot {
						botOff := botRowOff + (minX+x)*4
						bot = nrgbaPixel(pix, botOff)
					}
					rowCells[x] = pairToCell(top, bot).gridCell()
				}
				return
			}
		}

		for x := 0; x < width; x++ {
			srcX := b.Min.X + x
			top := safePixel(scaled, srcX, topY)
			var bot color.RGBA
			if hasBot {
				bot = safePixel(scaled, srcX, botY)
			}
			rowCells[x] = pairToCell(top, bot).gridCell()
		}
	}

	jobs := opts.Jobs
	if jobs <= 1 {
		for row := 0; row < rowCount; row++ {
			renderRowFunc(row)
			if opts.OnProgress != nil {
				progress.Done()
			}
		}
	} else {
		var wg sync.WaitGroup
		jobsCh := make(chan int)
		workerN := jobs
		if workerN > rowCount {
			workerN = rowCount
		}
		if workerN > core.MaxWorkers() {
			workerN = core.MaxWorkers()
		}
		for range workerN {
			go func() {
				for row := range jobsCh {
					renderRowFunc(row)
					if opts.OnProgress != nil {
						progress.Done()
					}
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
// By default each line starts with an erase-line/carriage-return prefix for
// standalone redraws; set Options.NoLinePrefix when composing output with
// other content on the same terminal row.
func Render(w io.Writer, img image.Image, cols int, opts Options) error {
	grid, err := RenderToGrid(img, cols, opts)
	if err != nil {
		return err
	}

	if !core.Fastpath {
		for y := 0; y < grid.Height; y++ {
			var sb strings.Builder
			if !opts.NoLinePrefix {
				sb.WriteString(ansiLinePrefix)
			}
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

	var buf []byte
	var runeBuf [utf8.UTFMax]byte
	for y := 0; y < grid.Height; y++ {
		buf = buf[:0]
		if !opts.NoLinePrefix {
			buf = append(buf, ansiLinePrefix...)
		}
		for x := 0; x < grid.Width; x++ {
			c := grid.Cells[y][x]
			if c.Transparent {
				buf = append(buf, ' ')
			} else {
				if c.HasBg {
					buf = appendBgRGB(buf, c.Bg)
				}
				if c.HasFg {
					buf = appendFgRGB(buf, c.Fg)
				}
				n := utf8.EncodeRune(runeBuf[:], c.Ch)
				buf = append(buf, runeBuf[:n]...)
				buf = append(buf, ansiReset...)
			}
		}
		buf = append(buf, '\n')
		n, err := w.Write(buf)
		if err != nil {
			return fmt.Errorf("halfblock render: %w", err)
		}
		if n != len(buf) {
			return fmt.Errorf("halfblock render: %w", io.ErrShortWrite)
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

// ansiColor returns the opaque pixel represented by an ANSI true-color
// escape. ANSI RGB escapes do not carry alpha; transparent halves are
// represented by the absence of a foreground/background escape instead.
func ansiColor(c color.RGBA) color.RGBA {
	c.A = 255
	return c
}

// RenderToImage renders the image using the halfblock algorithm and returns a new image.
func RenderToImage(img image.Image) *image.RGBA {
	return RenderToImageJ(img, 1)
}

// RenderToImageJ is a worker-aware copy of RenderToImage.
func RenderToImageJ(img image.Image, jobs int) *image.RGBA {
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()
	dst := image.NewRGBA(b)

	if !core.Fastpath {
		grid, _ := RenderToGrid(img, width, Options{Jobs: jobs})
		for y := 0; y < grid.Height; y++ {
			topY := b.Min.Y + y*2
			botY := topY + 1
			for x := 0; x < grid.Width; x++ {
				c := grid.Cells[y][x]
				var topColor, botColor color.RGBA
				if !c.Transparent {
					bg := c.Bg
					if !c.HasBg {
						bg = color.RGBA{}
					}
					fg := c.Fg
					if !c.HasFg {
						fg = bg
					}
					if c.HasBg {
						bg = ansiColor(bg)
					}
					if c.HasFg {
						fg = ansiColor(fg)
					}
					switch c.Ch {
					case '▀':
						topColor, botColor = fg, bg
					case '▄':
						topColor, botColor = bg, fg
					case '█':
						topColor, botColor = fg, fg
					default:
						topColor, botColor = bg, bg
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

	rowCount := (height + 1) / 2
	if rowCount <= 0 {
		return dst
	}

	renderRowFunc := func(row int) {
		topY := b.Min.Y + row*2
		botY := topY + 1
		hasBot := botY < b.Max.Y
		topRowOff := (topY - dst.Rect.Min.Y) * dst.Stride
		botRowOff := (botY - dst.Rect.Min.Y) * dst.Stride

		if rgba, ok := img.(*image.RGBA); ok {
			srcTopRowOff := (topY - rgba.Rect.Min.Y) * rgba.Stride
			srcBotRowOff := (botY - rgba.Rect.Min.Y) * rgba.Stride
			pix := rgba.Pix
			minX := b.Min.X - rgba.Rect.Min.X
			for x := 0; x < width; x++ {
				srcX := minX + x
				topOff := srcTopRowOff + srcX*4
				storeOpaquePixel(dst.Pix, topRowOff+x*4, rgbaPixel(pix, topOff))
				if hasBot {
					botOff := srcBotRowOff + srcX*4
					storeOpaquePixel(dst.Pix, botRowOff+x*4, rgbaPixel(pix, botOff))
				}
			}
			return
		}

		if nrgba, ok := img.(*image.NRGBA); ok {
			srcTopRowOff := (topY - nrgba.Rect.Min.Y) * nrgba.Stride
			srcBotRowOff := (botY - nrgba.Rect.Min.Y) * nrgba.Stride
			pix := nrgba.Pix
			minX := b.Min.X - nrgba.Rect.Min.X
			for x := 0; x < width; x++ {
				srcX := minX + x
				topOff := srcTopRowOff + srcX*4
				storeOpaquePixel(dst.Pix, topRowOff+x*4, nrgbaPixel(pix, topOff))
				if hasBot {
					botOff := srcBotRowOff + srcX*4
					storeOpaquePixel(dst.Pix, botRowOff+x*4, nrgbaPixel(pix, botOff))
				}
			}
			return
		}

		for x := 0; x < width; x++ {
			srcX := b.Min.X + x
			top := safePixel(img, srcX, topY)
			storeOpaquePixel(dst.Pix, topRowOff+x*4, top)
			if hasBot {
				bot := safePixel(img, srcX, botY)
				storeOpaquePixel(dst.Pix, botRowOff+x*4, bot)
			}
		}
	}

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
		if workerN > core.MaxWorkers() {
			workerN = core.MaxWorkers()
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

	return dst
}

// RenderJ is kept for testing compatibility.
func RenderJ(w io.Writer, img image.Image, jobs int) error {
	b := img.Bounds()
	return Render(w, img, b.Dx(), Options{Jobs: jobs})
}
