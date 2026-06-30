package geomshape

import (
	"errors"
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
	v2GridCols = 4
	v2GridRows = 4
)

var v2PrefixOrder = [...]int{
	0, 1, 4, 2,
	5, 8, 3, 6,
	9, 12, 7, 10,
	13, 11, 14, 15,
}

var v2SupportedMasks = func() map[uint16]struct{} {
	out := make(map[uint16]struct{}, len(v2PrefixOrder))
	for i := 1; i <= len(v2PrefixOrder); i++ {
		out[v2MaskByRank(i)] = struct{}{}
	}
	return out
}()

func v2SplitAxis(min, max, parts, idx int) (int, int) {
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

func v2MaskByRank(n int) uint16 {
	if n < 1 {
		n = 1
	}
	if n > len(v2PrefixOrder) {
		return 0xffff
	}
	var mask uint16
	for i := 0; i < n; i++ {
		mask |= 1 << uint(v2PrefixOrder[i])
	}
	return mask
}

func v2AllMasks() []uint16 {
	out := make([]uint16, 0, len(v2PrefixOrder))
	for i := 1; i <= len(v2PrefixOrder); i++ {
		out = append(out, v2MaskByRank(i))
	}
	return out
}

func v2MaskContains(mask uint16, idx int) bool {
	if idx < 0 || idx >= v2GridCols*v2GridRows {
		return false
	}
	return mask&(1<<uint(idx)) != 0
}

func v2MaskCells(mask uint16) []int {
	out := make([]int, 0, v2GridCols*v2GridRows)
	for i := 0; i < v2GridCols*v2GridRows; i++ {
		if v2MaskContains(mask, i) {
			out = append(out, i)
		}
	}
	return out
}

func v2MaskSupported(mask uint16) bool {
	_, ok := v2SupportedMasks[mask]
	return ok
}

func v2MonotoneCounts(counts []int) bool {
	first := -1
	last := -1
	trimmed := make([]int, 0, len(counts))
	for _, c := range counts {
		if c == 0 {
			continue
		}
		if first < 0 {
			first = c
		}
		last = c
		trimmed = append(trimmed, c)
	}
	if len(trimmed) <= 1 {
		return true
	}
	nondecreasing := first <= last
	prev := trimmed[0]
	for _, c := range trimmed[1:] {
		if absInt(c-prev) > 1 {
			return false
		}
		if nondecreasing {
			if c < prev {
				return false
			}
		} else if c > prev {
			return false
		}
		prev = c
	}
	return true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// v2MaskIsOrthConvex rejects masks that would produce visible gaps or dents.
// A valid mask must be connected and row/column convex on the 4x4 sampling grid.
func v2MaskIsOrthConvex(mask uint16) bool {
	cells := v2MaskCells(mask)
	if len(cells) == 0 {
		return false
	}

	rowHas := [v2GridRows]bool{}
	colHas := [v2GridCols]bool{}
	rowMin := [v2GridRows]int{}
	rowMax := [v2GridRows]int{}
	colMin := [v2GridCols]int{}
	colMax := [v2GridCols]int{}
	for i := range rowMin {
		rowMin[i] = v2GridCols
	}
	for i := range colMin {
		colMin[i] = v2GridRows
	}

	for _, idx := range cells {
		row := idx / v2GridCols
		col := idx % v2GridCols
		rowHas[row] = true
		colHas[col] = true
		if col < rowMin[row] {
			rowMin[row] = col
		}
		if col > rowMax[row] {
			rowMax[row] = col
		}
		if row < colMin[col] {
			colMin[col] = row
		}
		if row > colMax[col] {
			colMax[col] = row
		}
	}

	for row := 0; row < v2GridRows; row++ {
		if !rowHas[row] {
			continue
		}
		for col := rowMin[row]; col <= rowMax[row]; col++ {
			if !v2MaskContains(mask, row*v2GridCols+col) {
				return false
			}
		}
	}
	for col := 0; col < v2GridCols; col++ {
		if !colHas[col] {
			continue
		}
		for row := colMin[col]; row <= colMax[col]; row++ {
			if !v2MaskContains(mask, row*v2GridCols+col) {
				return false
			}
		}
	}

	rowCounts := make([]int, 0, v2GridRows)
	colCounts := make([]int, 0, v2GridCols)
	for row := 0; row < v2GridRows; row++ {
		count := 0
		for col := 0; col < v2GridCols; col++ {
			if v2MaskContains(mask, row*v2GridCols+col) {
				count++
			}
		}
		rowCounts = append(rowCounts, count)
	}
	for col := 0; col < v2GridCols; col++ {
		count := 0
		for row := 0; row < v2GridRows; row++ {
			if v2MaskContains(mask, row*v2GridCols+col) {
				count++
			}
		}
		colCounts = append(colCounts, count)
	}
	if !v2MonotoneCounts(rowCounts) || !v2MonotoneCounts(colCounts) {
		return false
	}

	seen := map[int]struct{}{cells[0]: struct{}{}}
	queue := []int{cells[0]}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		row := cur / v2GridCols
		col := cur % v2GridCols
		for _, nb := range []int{
			(row-1)*v2GridCols + col,
			(row+1)*v2GridCols + col,
			row*v2GridCols + (col - 1),
			row*v2GridCols + (col + 1),
		} {
			nr := nb / v2GridCols
			nc := nb % v2GridCols
			if nr < 0 || nr >= v2GridRows || nc < 0 || nc >= v2GridCols {
				continue
			}
			if !v2MaskContains(mask, nb) {
				continue
			}
			if _, ok := seen[nb]; ok {
				continue
			}
			seen[nb] = struct{}{}
			queue = append(queue, nb)
		}
	}
	return len(seen) == len(cells)
}

var errV2MaskInvalid = errors.New("geomshape v2 mask has gap/dent/disconnection")

func v2SampleBlock(img image.Image, x0, x1, y0, y1 int) [16]color.RGBA {
	var pixels [16]color.RGBA
	if x1 <= x0 || y1 <= y0 {
		return pixels
	}
	i := 0
	for row := 0; row < v2GridRows; row++ {
		sy0, sy1 := v2SplitAxis(y0, y1, v2GridRows, row)
		for col := 0; col < v2GridCols; col++ {
			sx0, sx1 := v2SplitAxis(x0, x1, v2GridCols, col)
			pixels[i] = avgRegion(img, sx0, sx1, sy0, sy1)
			i++
		}
	}
	return pixels
}

type v2CellResult struct {
	ch          rune
	mask        uint16
	fg, bg      color.RGBA
	hasFG       bool
	hasBG       bool
	transparent bool
}

func v2ScoreMask(pixels [16]color.RGBA, mask uint16) (v2CellResult, int, error) {
	if !v2MaskSupported(mask) {
		return v2CellResult{}, 0, fmt.Errorf("%w: unsupported mask=%04x", errV2MaskInvalid, mask)
	}
	if !v2MaskIsOrthConvex(mask) {
		return v2CellResult{}, 0, fmt.Errorf("%w: mask=%04x", errV2MaskInvalid, mask)
	}
	var fgPixels, bgPixels []color.RGBA
	var opaque int
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		opaque++
		if v2MaskContains(mask, i) {
			fgPixels = append(fgPixels, p)
		} else {
			bgPixels = append(bgPixels, p)
		}
	}
	if opaque == 0 {
		return v2CellResult{ch: ' ', mask: 0, transparent: true}, 0, nil
	}
	fg := avgRGBA(fgPixels...)
	bg := avgRGBA(bgPixels...)
	cell := v2CellResult{
		ch:   rune(0x1FB40 + popcount16(mask) - 1),
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
		if v2MaskContains(mask, i) {
			target = fg
		}
		score += rgbaDist2(p, target)
	}
	return cell, score, nil
}

func v2DirectMask(pixels [16]color.RGBA) uint16 {
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
	var bright int
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if luma(p) >= mean {
			bright++
		}
	}
	if bright < 1 {
		bright = 1
	}
	return v2MaskByRank(bright)
}

func v2HeuristicMasks(pixels [16]color.RGBA) []uint16 {
	candidates := make(map[uint16]struct{}, 16)
	add := func(mask uint16) {
		candidates[mask] = struct{}{}
	}

	direct := v2DirectMask(pixels)
	add(direct)

	bright := popcount16(direct)
	add(v2MaskByRank(bright))
	add(v2MaskByRank(bright + 1))
	if bright > 1 {
		add(v2MaskByRank(bright - 1))
	}
	add(0xffff)

	out := make([]uint16, 0, len(candidates))
	for m := range candidates {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func popcount16(mask uint16) int {
	n := 0
	for mask > 0 {
		n += int(mask & 1)
		mask >>= 1
	}
	return n
}

func v2ChooseCell(pixels [16]color.RGBA, mode Mode) (v2CellResult, error) {
	switch mode {
	case ModeBest:
		return v2ChooseBestCell(pixels, v2AllMasks())
	case ModeGeom:
		cell, _, err := v2ScoreMask(pixels, v2DirectMask(pixels))
		return cell, err
	default:
		cell, _, err := v2ScoreMask(pixels, v2DirectMask(pixels))
		return cell, err
	}
}

func v2ChooseBestCell(pixels [16]color.RGBA, masks []uint16) (v2CellResult, error) {
	preferred := v2DirectMask(pixels)
	var (
		bestCell  v2CellResult
		bestScore int
		haveBest  bool
	)
	for _, mask := range masks {
		cell, score, err := v2ScoreMask(pixels, mask)
		if err != nil {
			continue
		}
		if !haveBest {
			bestCell = cell
			bestScore = score
			haveBest = true
			continue
		}
		if score < bestScore ||
			(score == bestScore && popcount16(mask&preferred) > popcount16(bestCell.mask&preferred)) ||
			(score == bestScore && popcount16(mask&preferred) == popcount16(bestCell.mask&preferred) && popcount16(mask) > popcount16(bestCell.mask)) {
			bestCell = cell
			bestScore = score
		}
	}
	if !haveBest {
		return v2CellResult{}, fmt.Errorf("%w: no valid mask candidates", errV2MaskInvalid)
	}
	return bestCell, nil
}

func v2CellEscape(c v2CellResult) string {
	var b strings.Builder
	if c.hasBG {
		b.WriteString(bgRGB(c.bg))
	}
	if c.hasFG {
		b.WriteString(fgRGB(c.fg))
	}
	return b.String()
}

func v2RenderRow(w io.Writer, img image.Image, b image.Rectangle, row int, mode Mode) error {
	y0 := b.Min.Y + row*v2GridRows
	y1 := min(y0+v2GridRows, b.Max.Y)
	var sb strings.Builder
	sb.WriteString(ansiLinePrefix)
	for col := 0; b.Min.X+col*v2GridCols < b.Max.X; col++ {
		x0 := b.Min.X + col*v2GridCols
		x1 := min(x0+v2GridCols, b.Max.X)
		pixels := v2SampleBlock(img, x0, x1, y0, y1)
		cell, err := v2ChooseCell(pixels, mode)
		if err != nil {
			return fmt.Errorf("geomshape render: %w", err)
		}
		if cell.transparent {
			sb.WriteRune(' ')
			continue
		}
		sb.WriteString(v2CellEscape(cell))
		sb.WriteRune(cell.ch)
		sb.WriteString(ansiReset)
	}
	_, err := fmt.Fprintln(w, sb.String())
	return err
}

func v2Render(w io.Writer, img image.Image, mode Mode) error {
	b := img.Bounds()
	rowCount := (b.Dy() + v2GridRows - 1) / v2GridRows
	if rowCount <= 0 {
		return nil
	}
	for row := 0; row < rowCount; row++ {
		if err := v2RenderRow(w, img, b, row, mode); err != nil {
			return fmt.Errorf("geomshape render: %w", err)
		}
	}
	return nil
}

func v2RenderToImage(img image.Image, mode Mode) *image.RGBA {
	b := img.Bounds()
	dst := image.NewRGBA(b)
	rowCount := (b.Dy() + v2GridRows - 1) / v2GridRows
	if rowCount <= 0 {
		return dst
	}
	for row := 0; row < rowCount; row++ {
		y0 := b.Min.Y + row*v2GridRows
		y1 := min(y0+v2GridRows, b.Max.Y)
		for col := 0; b.Min.X+col*v2GridCols < b.Max.X; col++ {
			x0 := b.Min.X + col*v2GridCols
			x1 := min(x0+v2GridCols, b.Max.X)
			pixels := v2SampleBlock(img, x0, x1, y0, y1)
			cell, err := v2ChooseCell(pixels, mode)
			if err != nil {
				cell, err = v2ChooseBestCell(pixels, v2HeuristicMasks(pixels))
				if err != nil {
					continue
				}
			}
			for idx, px := range pixels {
				if isTransparent(px) {
					continue
				}
				target := cell.bg
				if v2MaskContains(cell.mask, idx) {
					target = cell.fg
				}
				yy := y0 + idx/v2GridCols
				xx := x0 + idx%v2GridCols
				if xx < b.Max.X && yy < b.Max.Y {
					dst.SetRGBA(xx, yy, target)
				}
			}
		}
	}
	return dst
}

func v2RenderJ(w io.Writer, img image.Image, mode Mode, jobs int) error {
	if jobs <= 1 {
		return v2Render(w, img, mode)
	}
	b := img.Bounds()
	rowCount := (b.Dy() + v2GridRows - 1) / v2GridRows
	if rowCount <= 0 {
		return nil
	}
	rows := make([]string, rowCount)
	jobsCh := make(chan int)
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
				y0 := b.Min.Y + row*v2GridRows
				y1 := min(y0+v2GridRows, b.Max.Y)
				var sb strings.Builder
				sb.WriteString(ansiLinePrefix)
				for col := 0; b.Min.X+col*v2GridCols < b.Max.X; col++ {
					x0 := b.Min.X + col*v2GridCols
					x1 := min(x0+v2GridCols, b.Max.X)
					pixels := v2SampleBlock(img, x0, x1, y0, y1)
					cell, err := v2ChooseCell(pixels, mode)
					if err != nil {
						panic(fmt.Errorf("geomshape render: %w", err))
					}
					if cell.transparent {
						sb.WriteRune(' ')
						continue
					}
					sb.WriteString(v2CellEscape(cell))
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
	for _, row := range rows {
		if _, err := fmt.Fprintln(w, row); err != nil {
			return fmt.Errorf("geomshape render: %w", err)
		}
	}
	return nil
}

func v2RenderToImageJ(img image.Image, mode Mode, jobs int) *image.RGBA {
	if jobs <= 1 {
		return v2RenderToImage(img, mode)
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	rowCount := (b.Dy() + v2GridRows - 1) / v2GridRows
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
	for i := 0; i < workerN; i++ {
		go func() {
			for row := range jobsCh {
				y0 := b.Min.Y + row*v2GridRows
				y1 := min(y0+v2GridRows, b.Max.Y)
				for col := 0; b.Min.X+col*v2GridCols < b.Max.X; col++ {
					x0 := b.Min.X + col*v2GridCols
					x1 := min(x0+v2GridCols, b.Max.X)
					pixels := v2SampleBlock(img, x0, x1, y0, y1)
					cell, err := v2ChooseCell(pixels, mode)
					if err != nil {
						cell, err = v2ChooseBestCell(pixels, v2HeuristicMasks(pixels))
						if err != nil {
							continue
						}
					}
					for idx, px := range pixels {
						if isTransparent(px) {
							continue
						}
						target := cell.bg
						if v2MaskContains(cell.mask, idx) {
							target = cell.fg
						}
						yy := y0 + idx/v2GridCols
						xx := x0 + idx%v2GridCols
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
