package geomshape

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K"
	ansiCarriageReturn = "\r"
	ansiLinePrefix     = ansiEraseLine + ansiCarriageReturn

	blockCols = 2
	blockRows = 2
)

// FIXME: this is a v2 family copy. Keep it isolated until the geometry and
// glyph scoring settle, then consolidate with the other block renderers.
type Mode int

const (
	ModeShape Mode = iota
	ModeGeom
	ModeBest
)

func (m Mode) String() string {
	switch m {
	case ModeGeom:
		return "geom"
	case ModeBest:
		return "best"
	default:
		return "2x2"
	}
}

var (
	geomshapeRuneByMask = map[uint8]rune{}
	geomshapeMasks      []uint8
)

func init() {
	for mask := 0; mask < 16; mask++ {
		geomshapeRuneByMask[uint8(mask)] = rune(0x1FB40 + mask)
		geomshapeMasks = append(geomshapeMasks, uint8(mask))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func bitForIndex(idx int) uint8 {
	switch idx {
	case 0:
		return 1 << 3 // UL
	case 1:
		return 1 << 2 // UR
	case 2:
		return 1 << 1 // LL
	case 3:
		return 1 << 0 // LR
	default:
		return 0
	}
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

func sampleBlock(img image.Image, x0, x1, y0, y1 int, sampler Sampler) [4]color.RGBA {
	return sampleBlockWithSampler(img, x0, x1, y0, y1, sampler)
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

func directMask(pixels [4]color.RGBA) uint8 {
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

func topNMask(pixels [4]color.RGBA, n int) uint8 {
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
		return 0b1111
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

func topMask(pixels [4]color.RGBA) uint8 {
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
			mask |= 0b0001 << uint(row)
		}
	}
	return mask
}

func rightMask(pixels [4]color.RGBA) uint8 {
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
			mask |= 0b0010 << uint(col)
		}
	}
	return mask
}

func diagMask(pixels [4]color.RGBA) uint8 {
	var a, b []color.RGBA
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if i == 0 || i == 3 {
			a = append(a, p)
		} else {
			b = append(b, p)
		}
	}
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	ma := avgRGBA(a...)
	mb := avgRGBA(b...)
	if luma(ma) >= luma(mb) {
		return bitForIndex(0) | bitForIndex(3)
	}
	return bitForIndex(1) | bitForIndex(2)
}

func antiDiagMask(pixels [4]color.RGBA) uint8 {
	var a, b []color.RGBA
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if i == 1 || i == 2 {
			a = append(a, p)
		} else {
			b = append(b, p)
		}
	}
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	ma := avgRGBA(a...)
	mb := avgRGBA(b...)
	if luma(ma) >= luma(mb) {
		return bitForIndex(1) | bitForIndex(2)
	}
	return bitForIndex(0) | bitForIndex(3)
}

func heuristicMasks(pixels [4]color.RGBA) []uint8 {
	candidates := make(map[uint8]struct{}, 16)
	add := func(mask uint8) {
		candidates[mask] = struct{}{}
	}

	direct := directMask(pixels)
	add(direct)
	add(^direct & 0b1111)

	directCount := topNMask(pixels, popcount(direct))
	add(directCount)
	add(^directCount & 0b1111)

	add(diagMask(pixels))
	add(^diagMask(pixels) & 0b1111)
	add(antiDiagMask(pixels))
	add(^antiDiagMask(pixels) & 0b1111)
	add(0)
	add(0b1111)

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

func scoreMask(pixels [4]color.RGBA, mask uint8) (cellResult, int) {
	fgPixels := make([]color.RGBA, 0, 4)
	bgPixels := make([]color.RGBA, 0, 4)
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
	cell := cellResult{
		ch:   geomshapeRuneByMask[mask],
		mask: mask,
		fg:   fg,
		bg:   bg,
	}
	if fg.A != 0 {
		cell.hasFG = true
	}
	if bg.A != 0 {
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

	if mask == 0 {
		cell.transparent = false
	}
	return cell, score
}

func chooseCell(pixels [4]color.RGBA, mode Mode) cellResult {
	switch mode {
	case ModeBest:
		return chooseBestCell(pixels, allMasks())
	case ModeGeom:
		return chooseBestCell(pixels, heuristicMasks(pixels))
	default:
		cell, _ := scoreMask(pixels, directMask(pixels))
		return cell
	}
}

func chooseBestCell(pixels [4]color.RGBA, masks []uint8) cellResult {
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
	out := make([]uint8, len(geomshapeMasks))
	copy(out, geomshapeMasks)
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

func renderCell(w io.Writer, pixels [4]color.RGBA, mode Mode) error {
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

func renderRow(w io.Writer, img image.Image, b image.Rectangle, row int, mode Mode, sampler Sampler) error {
	y0 := b.Min.Y + row*blockRows
	y1 := min(y0+blockRows, b.Max.Y)
	var sb strings.Builder
	sb.WriteString(ansiLinePrefix)
	for col := 0; b.Min.X+col*blockCols < b.Max.X; col++ {
		x0 := b.Min.X + col*blockCols
		x1 := min(x0+blockCols, b.Max.X)
		pixels := sampleBlock(img, x0, x1, y0, y1, sampler)
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

// Render writes img to w as ANSI geomshape art.
func Render(w io.Writer, img image.Image, mode Mode) error {
	return RenderWithSampler(w, img, mode, SamplerLegacy)
}

// RenderWithSampler writes img to w as ANSI geomshape art.
func RenderWithSampler(w io.Writer, img image.Image, mode Mode, sampler Sampler) error {
	if sampler == SamplerV2 {
		return v2Render(w, img, mode)
	}
	b := img.Bounds()
	rowCount := (b.Dy() + blockRows - 1) / blockRows
	if rowCount <= 0 {
		return nil
	}
	for row := 0; row < rowCount; row++ {
		if err := renderRow(w, img, b, row, mode, sampler); err != nil {
			return fmt.Errorf("geomshape render: %w", err)
		}
	}
	return nil
}

// RenderToImage reconstructs the rendered pixel output of img.
func RenderToImage(img image.Image, mode Mode) *image.RGBA {
	return RenderToImageWithSampler(img, mode, SamplerLegacy)
}

// RenderToImageWithSampler reconstructs the rendered pixel output of img.
func RenderToImageWithSampler(img image.Image, mode Mode, sampler Sampler) *image.RGBA {
	if sampler == SamplerV2 {
		return v2RenderToImage(img, mode)
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	rowCount := (b.Dy() + blockRows - 1) / blockRows
	if rowCount <= 0 {
		return dst
	}

	for row := 0; row < rowCount; row++ {
		y0 := b.Min.Y + row*blockRows
		y1 := min(y0+blockRows, b.Max.Y)
		for col := 0; b.Min.X+col*blockCols < b.Max.X; col++ {
			x0 := b.Min.X + col*blockCols
			x1 := min(x0+blockCols, b.Max.X)
			pixels := sampleBlock(img, x0, x1, y0, y1, sampler)
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
	}
	return dst
}

// RenderJ is a worker-aware copy of Render.
// FIXME: keep this isolated until the mode strategy settles.
func RenderJ(w io.Writer, img image.Image, mode Mode, jobs int) error {
	return RenderJWithSampler(w, img, mode, SamplerLegacy, jobs)
}

// RenderJWithSampler is a worker-aware copy of RenderWithSampler.
// FIXME: keep this isolated until the mode strategy settles.
func RenderJWithSampler(w io.Writer, img image.Image, mode Mode, sampler Sampler, jobs int) error {
	if jobs <= 1 {
		return RenderWithSampler(w, img, mode, sampler)
	}
	if sampler == SamplerV2 {
		return v2RenderJ(w, img, mode, jobs)
	}
	b := img.Bounds()
	rowCount := (b.Dy() + blockRows - 1) / blockRows
	if rowCount <= 0 {
		return nil
	}

	rows := make([]string, rowCount)
	jobsCh := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	workerN := jobs
	if workerN > rowCount {
		workerN = rowCount
	}
	if workerN > runtime.NumCPU() {
		workerN = runtime.NumCPU()
	}
	for i := 0; i < workerN; i++ {
		go func() {
			for row := range jobsCh {
				y0 := b.Min.Y + row*blockRows
				y1 := min(y0+blockRows, b.Max.Y)
				var sb strings.Builder
				sb.WriteString(ansiLinePrefix)
				for col := 0; b.Min.X+col*blockCols < b.Max.X; col++ {
					x0 := b.Min.X + col*blockCols
					x1 := min(x0+blockCols, b.Max.X)
					pixels := sampleBlock(img, x0, x1, y0, y1, sampler)
					cell := chooseCell(pixels, mode)
					if cell.transparent {
						sb.WriteRune(' ')
						continue
					}
					sb.WriteString(cellEscape(cell))
					sb.WriteRune(cell.ch)
					sb.WriteString(ansiReset)
				}
				rows[row] = sb.String()
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

	select {
	case err := <-errCh:
		return err
	default:
	}

	for _, row := range rows {
		if _, err := fmt.Fprintln(w, row); err != nil {
			return fmt.Errorf("geomshape render: %w", err)
		}
	}
	return nil
}

// RenderToImageJ is a worker-aware copy of RenderToImage.
// FIXME: keep this isolated until the mode strategy settles.
func RenderToImageJ(img image.Image, mode Mode, jobs int) *image.RGBA {
	return RenderToImageJWithSampler(img, mode, SamplerLegacy, jobs)
}

// RenderToImageJWithSampler is a worker-aware copy of RenderToImageWithSampler.
// FIXME: keep this isolated until the mode strategy settles.
func RenderToImageJWithSampler(img image.Image, mode Mode, sampler Sampler, jobs int) *image.RGBA {
	if jobs <= 1 {
		return RenderToImageWithSampler(img, mode, sampler)
	}
	if sampler == SamplerV2 {
		return v2RenderToImageJ(img, mode, jobs)
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	rowCount := (b.Dy() + blockRows - 1) / blockRows
	if rowCount <= 0 {
		return dst
	}

	jobsCh := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	workerN := jobs
	if workerN > rowCount {
		workerN = rowCount
	}
	if workerN > runtime.NumCPU() {
		workerN = runtime.NumCPU()
	}
	for i := 0; i < workerN; i++ {
		go func() {
			for row := range jobsCh {
				y0 := b.Min.Y + row*blockRows
				y1 := min(y0+blockRows, b.Max.Y)
				for col := 0; b.Min.X+col*blockCols < b.Max.X; col++ {
					x0 := b.Min.X + col*blockCols
					x1 := min(x0+blockCols, b.Max.X)
					pixels := sampleBlock(img, x0, x1, y0, y1, sampler)
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
	select {
	case err := <-errCh:
		_ = err
	default:
	}
	return dst
}
