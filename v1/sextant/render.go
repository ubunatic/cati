package sextant

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"

	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/viewgeom"
	"codeberg.org/ubunatic/cati/v1/core"
)

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K"
	ansiCarriageReturn = "\r"
	ansiLinePrefix     = ansiEraseLine + ansiCarriageReturn

	blockCols = 2
	blockRows = 3
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Mode selects the sextant candidate search strategy.

type Options struct {
	Mode Mode
	Rows int
	Jobs int
}

// ScaleToFit scales img for sextant rendering within the given terminal dimensions.
func ScaleToFit(img image.Image, cols, rows int) image.Image {
	b := img.Bounds()
	if cols <= 0 && rows <= 0 {
		return img
	}
	spec := viewgeom.NewV2CellRatio(2, 3, 4, 3)
	plan := spec.Fit(b.Dx(), b.Dy(), cols, rows, false)
	result := imgutil.ScaleNN(img, plan.RenderW, plan.RenderH)
	if plan.ExtH > 0 {
		result = imgutil.AppendTransparentRows(result, plan.ExtH)
	}
	return result
}

type Mode int

const (
	ModeSextant Mode = iota
)

func (m Mode) String() string {
	return "2x3"
}

var (
	sextantRuneByMask = map[uint8]rune{}
	sextantMasks      []uint8
)

func init() {
	next := rune(0x1FB00)
	for _, name := range sextantNames() {
		mask := sextantBits(name)
		sextantRuneByMask[mask] = next
		sextantMasks = append(sextantMasks, mask)
		next++
	}
	// The two pure-column splits (left column 1·3·5, right column 2·4·6) have no
	// dedicated sextant glyph in U+1FB00.. — they coincide with the pre-existing
	// half-block characters ▌ (U+258C) and ▐ (U+2590), so Unicode omits them from
	// the sextant set. Without these entries displayMask returns 0 for masks
	// 0b101010 / 0b010101 and the renderer emits rune(0) (a zero-width NUL),
	// which shifts the rest of the row and leaves the right edge unfilled.
	sextantRuneByMask[leftColumnMask] = '▌'
	sextantRuneByMask[rightColumnMask] = '▐'
}

// leftColumnMask / rightColumnMask are the two full-column patterns that map to
// half-block glyphs rather than dedicated sextant runes (see init).
const (
	leftColumnMask  uint8 = 0b101010 // pixels 1,3,5 → ▌
	rightColumnMask uint8 = 0b010101 // pixels 2,4,6 → ▐
)

func sextantNames() []string {
	return []string{
		"1", "2", "12", "3", "13", "23", "123",
		"4", "14", "24", "124", "34", "134", "234", "1234",
		"5", "15", "25", "125", "35", "235", "1235",
		"45", "145", "245", "1245", "345", "1345", "2345", "12345",
		"6", "16", "26", "126", "36", "136", "236", "1236",
		"46", "146", "1246", "346", "1346", "2346", "12346",
		"56", "156", "256", "1256", "356", "1356", "2356", "12356",
		"456", "1456", "2456", "12456", "3456", "13456", "23456",
	}
}

func sextantBits(subset string) uint8 {
	var bits uint8
	for _, r := range subset {
		if r < '1' || r > '6' {
			continue
		}
		bits |= sextantBit(int(r - '0'))
	}
	return bits
}

func maskName(mask uint8) string {
	if mask == 0 {
		return ""
	}
	var sb strings.Builder
	for digit := 1; digit <= 6; digit++ {
		if mask&sextantBit(digit) != 0 {
			sb.WriteByte(byte('0' + digit))
		}
	}
	return sb.String()
}

func sextantBit(digit int) uint8 {
	switch digit {
	case 1:
		return 1 << 5
	case 2:
		return 1 << 4
	case 3:
		return 1 << 3
	case 4:
		return 1 << 2
	case 5:
		return 1 << 1
	case 6:
		return 1 << 0
	default:
		return 0
	}
}

func displayMask(mask uint8) (uint8, bool) {
	mask &= 0b111111
	if _, ok := sextantRuneByMask[mask]; ok {
		return mask, false
	}
	inverted := ^mask & 0b111111
	if _, ok := sextantRuneByMask[inverted]; ok {
		return inverted, true
	}
	return 0, false
}

func bitForIndex(idx int) uint8 {
	return sextantBit(idx + 1)
}

func maskContains(mask uint8, idx int) bool {
	return mask&bitForIndex(idx) != 0
}

func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func isTransparent(c color.RGBA) bool { return c.A == 0 }

func fgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func bgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
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

func avgRGBA(pixels ...color.RGBA) color.RGBA {
	var r, g, b, n int
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		r += int(p.R)
		g += int(p.G)
		b += int(p.B)
		n++
	}
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 255}
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

func sampleBlock(img image.Image, x0, x1, y0, y1 int) [6]color.RGBA {
	var pixels [6]color.RGBA
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
			p := toRGBA(img.At(x, y))
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

func directMask(pixels [6]color.RGBA) uint8 {
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

func topNMask(pixels [6]color.RGBA, n int) uint8 {
	type ranked struct {
		idx  int
		luma float64
	}
	items := make([]ranked, 0, len(pixels))
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		items = append(items, ranked{idx: i, luma: luma(p)})
	}
	if n <= 0 || len(items) == 0 {
		return 0
	}
	if n >= len(items) {
		return 0b111111
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].luma == items[j].luma {
			return items[i].idx < items[j].idx
		}
		return items[i].luma > items[j].luma
	})
	var mask uint8
	for i := 0; i < n; i++ {
		mask |= bitForIndex(items[i].idx)
	}
	return mask
}

func rowMask(pixels [6]color.RGBA) uint8 {
	var sums [blockRows]float64
	var counts [blockRows]int
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		row := i / blockCols
		sums[row] += luma(p)
		counts[row]++
	}
	var total float64
	var n int
	for i := range sums {
		if counts[i] == 0 {
			continue
		}
		total += sums[i] / float64(counts[i])
		n++
	}
	if n == 0 {
		return 0
	}
	mean := total / float64(n)
	var mask uint8
	for row := 0; row < blockRows; row++ {
		if counts[row] == 0 {
			continue
		}
		if sums[row]/float64(counts[row]) >= mean {
			mask |= 0b11 << (4 - row*2)
		}
	}
	return mask
}

func colMask(pixels [6]color.RGBA) uint8 {
	var sums [blockCols]float64
	var counts [blockCols]int
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		col := i % blockCols
		sums[col] += luma(p)
		counts[col]++
	}
	var total float64
	var n int
	for i := range sums {
		if counts[i] == 0 {
			continue
		}
		total += sums[i] / float64(counts[i])
		n++
	}
	if n == 0 {
		return 0
	}
	mean := total / float64(n)
	var mask uint8
	for col := 0; col < blockCols; col++ {
		if counts[col] == 0 {
			continue
		}
		if sums[col]/float64(counts[col]) >= mean {
			for row := 0; row < blockRows; row++ {
				mask |= bitForIndex(row*blockCols + col)
			}
		}
	}
	return mask
}

func heuristicMasks(pixels [6]color.RGBA) []uint8 {
	candidates := make(map[uint8]struct{}, 16)
	add := func(mask uint8) {
		candidates[mask] = struct{}{}
	}

	direct := directMask(pixels)
	add(direct)
	add(^direct & 0b111111)
	add(topNMask(pixels, popcount(direct)))
	add(^topNMask(pixels, popcount(direct)) & 0b111111)
	add(rowMask(pixels))
	add(^rowMask(pixels) & 0b111111)
	add(colMask(pixels))
	add(^colMask(pixels) & 0b111111)
	add(0)
	add(0b111111)

	out := make([]uint8, 0, len(candidates))
	for m := range candidates {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func popcount(mask uint8) int {
	n := 0
	for mask > 0 {
		n += int(mask & 1)
		mask >>= 1
	}
	return n
}

func scoreMask(pixels [6]color.RGBA, mask uint8) (cellResult, int) {
	mask &= 0b111111
	fgPixels := make([]color.RGBA, 0, 6)
	bgPixels := make([]color.RGBA, 0, 6)
	var opaque int
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		opaque++
		if maskContains(mask, i) {
			fgPixels = append(fgPixels, p)
		} else {
			bgPixels = append(bgPixels, p)
		}
	}

	if opaque == 0 {
		return cellResult{ch: ' ', mask: 0, transparent: true}, 0
	}

	fg := avgRGBA(fgPixels...)
	bg := avgRGBA(bgPixels...)
	display, inverted := displayMask(mask)
	displayFG, displayBG := fg, bg
	if inverted {
		displayFG, displayBG = bg, fg
	}
	cell := cellResult{
		ch:   sextantRuneByMask[display],
		mask: display,
		fg:   displayFG,
		bg:   displayBG,
	}
	if cell.fg.A != 0 {
		cell.hasFG = true
	}
	if cell.bg.A != 0 {
		cell.hasBG = true
	}
	if opaque > 0 && !cell.hasBG {
		cell.bg = avgRGBA(pixels[:]...)
		cell.hasBG = true
	}

	score := 0
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		target := bg
		if maskContains(mask, i) {
			target = fg
		}
		score += rgbaDist2(p, target)
	}

	if mask == 0 || mask == 0b111111 {
		cell.ch = ' '
		cell.mask = 0
		if mask == 0 && cell.bg.A == 0 {
			cell.bg = avgRGBA(bgPixels...)
			cell.hasBG = cell.bg.A != 0
		}
		if mask == 0b111111 && cell.bg.A == 0 {
			cell.bg = avgRGBA(fgPixels...)
			cell.hasBG = cell.bg.A != 0
		}
	}
	return cell, score
}

func chooseCell(pixels [6]color.RGBA, mode Mode) cellResult {
	mask := directMask(pixels)
	cell, _ := scoreMask(pixels, mask)
	return cell
}

func chooseBestCell(pixels [6]color.RGBA, masks []uint8) cellResult {
	if len(masks) == 0 {
		return cellResult{ch: ' ', transparent: true}
	}
	preferred := directMask(pixels)
	bestCell, bestScore := scoreMask(pixels, masks[0])
	for _, mask := range masks[1:] {
		cell, score := scoreMask(pixels, mask)
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

func allMasks() []uint8 {
	out := make([]uint8, len(sextantMasks))
	copy(out, sextantMasks)
	return out
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

func renderCell(w io.Writer, pixels [6]color.RGBA, mode Mode) error {
	cell := chooseCell(pixels, mode)
	if cell.transparent {
		_, err := io.WriteString(w, " ")
		return err
	}
	if _, err := io.WriteString(w, cellEscape(cell)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(cell.ch)); err != nil {
		return err
	}
	_, err := io.WriteString(w, ansiReset)
	return err
}

func renderRow(w io.Writer, img image.Image, b image.Rectangle, row int, mode Mode) error {
	y0 := b.Min.Y + row*blockRows
	y1 := min(y0+blockRows, b.Max.Y)
	var sb strings.Builder
	sb.WriteString(ansiLinePrefix)
	for col := 0; b.Min.X+col*blockCols < b.Max.X; col++ {
		x0 := b.Min.X + col*blockCols
		x1 := min(x0+blockCols, b.Max.X)
		pixels := sampleBlock(img, x0, x1, y0, y1)
		cell := chooseCell(pixels, mode)
		if cell.transparent {
			sb.WriteRune(' ')
			continue
		}
		sb.WriteString(cellEscape(cell))
		sb.WriteRune(cell.ch)
		sb.WriteString(ansiReset)
	}
	_, err := fmt.Fprintln(w, sb.String())
	return err
}

// RenderToGrid scales and converts the image into a grid of terminal cells.
func RenderToGrid(img image.Image, cols int, opts Options) (*core.Grid, error) {
	scaled := ScaleToFit(img, cols, opts.Rows)
	b := scaled.Bounds()
	rowCount := (b.Dy() + blockRows - 1) / blockRows
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
					renderRow(row)
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

// Render writes img to w as ANSI sextant art.
func Render(w io.Writer, img image.Image, cols int, opts Options) error {
	grid, err := RenderToGrid(img, cols, opts)
	if err != nil {
		return err
	}

	for y := 0; y < grid.Height; y++ {
		var sb strings.Builder
		sb.WriteString(ansiLinePrefix)
		for x := 0; x < grid.Width; x++ {
			cell := grid.Cells[y][x]
			if cell.Transparent {
				sb.WriteRune(' ')
				continue
			}
			if cell.HasBg {
				sb.WriteString(bgRGB(cell.Bg))
			}
			if cell.HasFg {
				sb.WriteString(fgRGB(cell.Fg))
			}
			sb.WriteRune(cell.Ch)
			sb.WriteString(ansiReset)
		}
		if _, err := fmt.Fprintln(w, sb.String()); err != nil {
			return fmt.Errorf("sextant render: %w", err)
		}
	}
	return nil
}

// RenderToImage reconstructs the rendered pixel output of img.
func RenderToImage(img image.Image, mode Mode) *image.RGBA {
	return RenderToImageJ(img, mode, 1)
}

// RenderJ is kept for compatibility.
func RenderJ(w io.Writer, img image.Image, mode Mode, jobs int) error {
	return Render(w, img, 0, Options{Mode: mode, Jobs: jobs})
}

// RenderToImageJ reconstructs the rendered pixel output of img.
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
	if workerN > runtime.NumCPU() {
		workerN = runtime.NumCPU()
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
					for idx, px := range pixels {
						if isTransparent(px) {
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
