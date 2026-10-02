package braille

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"ubunatic.com/cati/internal/imgutil"
	"ubunatic.com/cati/internal/viewgeom"
	"ubunatic.com/cati/v1/core"
)

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K"
	ansiCarriageReturn = "\r"
	ansiLinePrefix     = ansiEraseLine + ansiCarriageReturn

	blockCols = 2
	blockRows = 4

	// transparentOverpaintPenalty matches sextant/quadblock penalty
	transparentOverpaintPenalty = 1 << 24
)

// Braille dot offset mapping for standard Unicode Braille Patterns (U+2800..U+28FF)
// Standard Unicode Braille dot numbering:
// Dot 1: col 0, row 0 -> 0x01
// Dot 2: col 0, row 1 -> 0x02
// Dot 3: col 0, row 2 -> 0x04
// Dot 4: col 1, row 0 -> 0x08
// Dot 5: col 1, row 1 -> 0x10
// Dot 6: col 1, row 2 -> 0x20
// Dot 7: col 0, row 3 -> 0x40
// Dot 8: col 1, row 3 -> 0x80
var dotBitTable = [8]uint8{
	0x01, // (0,0) -> Dot 1
	0x08, // (1,0) -> Dot 4
	0x02, // (0,1) -> Dot 2
	0x10, // (1,1) -> Dot 5
	0x04, // (0,2) -> Dot 3
	0x20, // (1,2) -> Dot 6
	0x40, // (0,3) -> Dot 7
	0x80, // (1,3) -> Dot 8
}

func bitForIndex(idx int) uint8 {
	if uint(idx) < 8 {
		return dotBitTable[idx]
	}
	return 0
}

func maskContains(mask uint8, idx int) bool {
	return mask&dotBitTable[idx] != 0
}

type Options struct {
	NoLinePrefix bool
	Mode         Mode
	Rows         int
	Jobs         int
	OnProgress   func(core.Progress)
}

func ScaleToFit(img image.Image, cols, rows int) image.Image {
	b := img.Bounds()
	if cols <= 0 && rows <= 0 {
		return img
	}
	// 2x4 braille cell has 2 cols, 4 rows. Standard terminal cell aspect ratio is 1:2.
	spec := viewgeom.NewV2CellRatio(2, 4, 1, 1)
	plan := spec.Fit(b.Dx(), b.Dy(), cols, rows, false)
	result := imgutil.ScaleNN(img, plan.RenderW, plan.RenderH)
	if plan.ExtH > 0 {
		result = imgutil.AppendTransparentRows(result, plan.ExtH)
	}
	return result
}

func Glyphs() []rune {
	result := make([]rune, 256)
	for i := 0; i < 256; i++ {
		result[i] = rune(0x2800 + i)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func isTransparent(c color.RGBA) bool { return c.A == 0 }

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

func fgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func bgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

func samplePixelFast(img image.Image, x, y int) color.RGBA {
	if core.Fastpath {
		if rgba, ok := img.(*image.RGBA); ok {
			if !image.Pt(x, y).In(rgba.Rect) {
				return color.RGBA{}
			}
			off := rgba.PixOffset(x, y)
			p := rgba.Pix[off : off+4 : off+4]
			if p[3] == 0 {
				return color.RGBA{}
			}
			return color.RGBA{R: p[0], G: p[1], B: p[2], A: p[3]}
		}
		if nrgba, ok := img.(*image.NRGBA); ok {
			if !image.Pt(x, y).In(nrgba.Rect) {
				return color.RGBA{}
			}
			off := nrgba.PixOffset(x, y)
			p := nrgba.Pix[off : off+4 : off+4]
			if p[3] == 0 {
				return color.RGBA{}
			}
			return color.RGBA{R: p[0], G: p[1], B: p[2], A: p[3]}
		}
	}
	return toRGBA(img.At(x, y))
}

type cellResult struct {
	ch          rune
	mask        uint8
	fg, bg      color.RGBA
	hasFG       bool
	hasBG       bool
	transparent bool
}

func rgbaDist2(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	da := int(a.A) - int(b.A)
	return dr*dr + dg*dg + db*db + da*da
}

func luma(p color.RGBA) float64 {
	if p.A == 0 {
		return 0
	}
	return 0.2126*float64(p.R) + 0.7152*float64(p.G) + 0.0722*float64(p.B)
}

func splitAxis(min, max, parts, idx int) (int, int) {
	if max <= min || parts <= 0 || idx < 0 || idx >= parts {
		return min, min
	}
	size := max - min
	start := min + idx*size/parts
	end := min + (idx+1)*size/parts
	if end <= start {
		end = start + 1
	}
	if end > max {
		end = max
	}
	return start, end
}

func sampleBlock(img image.Image, x0, x1, y0, y1 int) [8]color.RGBA {
	var pixels [8]color.RGBA
	i := 0
	for row := 0; row < blockRows; row++ {
		sy0, sy1 := splitAxis(y0, y1, blockRows, row)
		for col := 0; col < blockCols; col++ {
			sx0, sx1 := splitAxis(x0, x1, blockCols, col)
			pixels[i] = avgRegion(img, sx0, sx1, sy0, sy1)
			i++
		}
	}
	return pixels
}

func avgRegion(img image.Image, x0, x1, y0, y1 int) color.RGBA {
	if x1 <= x0 || y1 <= y0 {
		return color.RGBA{}
	}
	var r, g, b, n int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			p := samplePixelFast(img, x, y)
			if isTransparent(p) {
				continue
			}
			r += int(p.R)
			g += int(p.G)
			b += int(p.B)
			n++
		}
	}
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 255}
}

func directMask(pixels [8]color.RGBA) uint8 {
	var sum float64
	var n int
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		sum += luma(p)
		n++
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)
	var mask uint8
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if luma(p) >= mean {
			mask |= bitForIndex(i)
		}
	}
	return mask
}

func popcount(mask uint8) int {
	return bits.OnesCount8(mask)
}

func scoreMask(pixels [8]color.RGBA, mask uint8, mode Mode) (cellResult, int) {
	var fgR, fgG, fgB, fgN int
	var bgR, bgG, bgB, bgN int
	var opaque int

	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		opaque++
		if maskContains(mask, i) {
			fgR += int(p.R)
			fgG += int(p.G)
			fgB += int(p.B)
			fgN++
		} else {
			bgR += int(p.R)
			bgG += int(p.G)
			bgB += int(p.B)
			bgN++
		}
	}

	if opaque == 0 {
		return cellResult{ch: '⠀', mask: 0, transparent: true}, 0
	}

	var fg, bg color.RGBA
	if fgN > 0 {
		fg = color.RGBA{R: uint8(fgR / fgN), G: uint8(fgG / fgN), B: uint8(fgB / fgN), A: 255}
	}
	if bgN > 0 {
		bg = color.RGBA{R: uint8(bgR / bgN), G: uint8(bgG / bgN), B: uint8(bgB / bgN), A: 255}
	}

	// For ModeDots: only foreground dots represent image pixels. Background is not colored.
	// For ModeForegroundBackground: bg is set to represent the complementary area.
	cell := cellResult{
		ch:   rune(0x2800 + int(mask)),
		mask: mask,
		fg:   fg,
	}
	if mode == ModeForegroundBackground {
		cell.bg = bg
	}

	if cell.fg.A != 0 {
		cell.hasFG = true
	}
	if cell.bg.A != 0 {
		cell.hasBG = true
	}

	score := 0
	for i, p := range pixels {
		if isTransparent(p) {
			if emittedCoverage(cell, i) {
				score += transparentOverpaintPenalty
			}
			continue
		}
		target := bg
		if mode == ModeDots {
			target = color.RGBA{}
		}
		if maskContains(mask, i) {
			target = fg
		}
		score += rgbaDist2(p, target)
	}

	return cell, score
}

func emittedCoverage(cell cellResult, idx int) bool {
	if cell.transparent {
		return false
	}
	if cell.hasBG {
		return true
	}
	return cell.hasFG && maskContains(cell.mask, idx)
}

func chooseCell(pixels [8]color.RGBA, mode Mode) cellResult {
	if core.Fastpath {
		allTransparent := true
		for i := 0; i < 8; i++ {
			if pixels[i].A != 0 {
				allTransparent = false
				break
			}
		}
		if allTransparent {
			return cellResult{ch: '⠀', transparent: true}
		}
	}
	return chooseBestCell(pixels, allMasks(), mode)
}

func chooseBestCell(pixels [8]color.RGBA, masks []uint8, mode Mode) cellResult {
	if len(masks) == 0 {
		return cellResult{ch: '⠀', transparent: true}
	}
	preferred := directMask(pixels)
	bestCell, bestScore := scoreMask(pixels, masks[0], mode)
	for _, mask := range masks[1:] {
		cell, score := scoreMask(pixels, mask, mode)
		if score < bestScore ||
			(score == bestScore && maskOverlap(mask, preferred) > maskOverlap(bestCell.mask, preferred)) ||
			(score == bestScore && maskOverlap(mask, preferred) == maskOverlap(bestCell.mask, preferred) && popcount(mask) > popcount(bestCell.mask)) {
			bestCell = cell
			bestScore = score
		}
	}
	return bestCell
}

func maskOverlap(a, b uint8) int {
	return popcount(a & b)
}

var staticAllMasks = func() []uint8 {
	out := make([]uint8, 256)
	for mask := range out {
		out[mask] = uint8(mask)
	}
	return out
}()

func allMasks() []uint8 {
	return staticAllMasks
}

func cellEscape(c cellResult) string {
	var b strings.Builder
	if c.hasBG {
		b.WriteString(bgRGB(c.bg))
	}
	if c.hasFG {
		b.WriteString(fgRGB(c.fg))
	}
	return b.String()
}

func RenderToGrid(img image.Image, cols int, opts Options) (*core.Grid, error) {
	scaled := ScaleToFit(img, cols, opts.Rows)
	b := scaled.Bounds()
	rowCount := (b.Dy() + blockRows - 1) / blockRows
	progress := core.NewProgressReporter(opts.OnProgress, rowCount)
	if rowCount <= 0 {
		return &core.Grid{Cells: [][]core.Cell{}}, nil
	}

	colCount := (b.Dx() + blockCols - 1) / blockCols
	cells := make([][]core.Cell, rowCount)
	for r := range cells {
		cells[r] = make([]core.Cell, colCount)
	}

	renderRow := func(row int) {
		y0 := b.Min.Y + row*blockRows
		y1 := min(y0+blockRows, b.Max.Y)
		for col := 0; col < colCount; col++ {
			x0 := b.Min.X + col*blockCols
			x1 := min(x0+blockCols, b.Max.X)
			pixels := sampleBlock(scaled, x0, x1, y0, y1)
			c := chooseCell(pixels, opts.Mode)
			cells[row][col] = core.Cell{
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
			renderRow(row)
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
		maxCPUs := core.MaxWorkers()
		if workerN > maxCPUs {
			workerN = maxCPUs
		}
		for range workerN {
			go func() {
				for row := range jobsCh {
					renderRow(row)
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
		Width:  colCount,
		Height: rowCount,
		Cells:  cells,
	}, nil
}

func Render(w io.Writer, img image.Image, cols int, opts Options) error {
	grid, err := RenderToGrid(img, cols, opts)
	if err != nil {
		return err
	}

	var buf []byte
	for y := 0; y < grid.Height; y++ {
		buf = buf[:0]
		if !opts.NoLinePrefix {
			buf = append(buf, ansiLinePrefix...)
		}
		for x := 0; x < grid.Width; x++ {
			cell := grid.Cells[y][x]
			if cell.Transparent {
				buf = append(buf, ' ')
				continue
			}
			if core.Fastpath {
				if cell.HasBg {
					buf = appendBgRGB(buf, cell.Bg)
				}
				if cell.HasFg {
					buf = appendFgRGB(buf, cell.Fg)
				}
			} else {
				if cell.HasBg {
					buf = append(buf, bgRGB(cell.Bg)...)
				}
				if cell.HasFg {
					buf = append(buf, fgRGB(cell.Fg)...)
				}
			}
			buf = utf8.AppendRune(buf, cell.Ch)
			buf = append(buf, ansiReset...)
		}
		buf = append(buf, '\n')
		if _, err := w.Write(buf); err != nil {
			return fmt.Errorf("braille render: %w", err)
		}
	}
	return nil
}

func RenderToImage(img image.Image, mode Mode) *image.RGBA {
	return RenderToImageJ(img, mode, 1)
}

func RenderJ(w io.Writer, img image.Image, mode Mode, jobs int) error {
	return Render(w, img, 0, Options{Mode: mode, Jobs: jobs})
}

func RenderToImageJ(img image.Image, mode Mode, jobs int) *image.RGBA {
	b := img.Bounds()
	dst := image.NewRGBA(b)
	rowCount := (b.Dy() + blockRows - 1) / blockRows
	if rowCount <= 0 {
		return dst
	}
	jobsCh := make(chan int)
	var wg sync.WaitGroup
	workerN := jobs
	if workerN > rowCount {
		workerN = rowCount
	}
	maxCPUs := core.MaxWorkers()
	if workerN > maxCPUs {
		workerN = maxCPUs
	}
	for range workerN {
		go func() {
			for row := range jobsCh {
				y0 := b.Min.Y + row*blockRows
				y1 := min(y0+blockRows, b.Max.Y)
				for col := 0; b.Min.X+col*blockCols < b.Max.X; col++ {
					x0 := b.Min.X + col*blockCols
					x1 := min(x0+blockCols, b.Max.X)
					pixels := sampleBlock(img, x0, x1, y0, y1)
					cell := chooseCell(pixels, mode)
					for idx := range pixels {
						if !emittedCoverage(cell, idx) {
							continue
						}
						target := cell.bg
						if maskContains(cell.mask, idx) {
							target = cell.fg
						}
						yy := y0 + idx/blockCols
						xx := x0 + idx%blockCols
						if xx < b.Max.X && yy < b.Max.Y {
							dst.SetRGBA(xx, yy, target)
						}
					}
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
	return dst
}
