package sparkline

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"strings"
)

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K"
	ansiCarriageReturn = "\r"
	ansiLinePrefix     = ansiEraseLine + ansiCarriageReturn
)

// ScaleToFit scales img to fit within cols×rows pixels while preserving
// aspect ratio.  This gives the pixel budget from which each terminal
// cell will analyze an 8×8 block (or fewer when the image is small).
func ScaleToFit(img image.Image, cols, rows int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return img
	}
	newW, newH := srcW, srcH
	if cols > 0 && cols < newW {
		newW = cols
		newH = max(1, srcH*newW/srcW)
	}
	if rows > 0 && newH > rows {
		newH = rows
		newW = max(1, srcW*newH/srcH)
	}
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

// Render writes img to w as ANSI block-element art (vertical orientation).
// img should be at the resolution computed by pixCols(=termCols*4) × pixRows(=termRows*8).
func Render(w io.Writer, img image.Image) error {
	b := img.Bounds()
	return RenderOpts(w, img, max(1, b.Dx()/4), max(1, b.Dy()/8), Vertical)
}

// RenderOpts writes img to w as ANSI block-element art.  The image should
// be at the resolution computed by pixCols(=termCols*4) × pixRows(=termRows*8)
// so that each cell covers a 4×8 pixel block.
// outCols and outRows are the number of terminal columns and rows to emit.
//
// For each cell the algorithm evaluates the active mode's glyph masks and picks
// the one that minimises total squared colour error.
func RenderOpts(w io.Writer, img image.Image, outCols, outRows int, mode Mode) error {
	b := img.Bounds()
	pixW := b.Dx()
	pixH := b.Dy()

	cellW := max(1, pixW/outCols)
	cellH := max(1, pixH/outRows)

	for tr := 0; tr < outRows; tr++ {
		var sb strings.Builder
		sb.WriteString(ansiLinePrefix)

		for tc := 0; tc < outCols; tc++ {
			x0 := b.Min.X + min(tc*cellW, pixW)
			x1 := b.Min.X + min(tc*cellW+cellW, pixW) - 1
			y0 := b.Min.Y + min(tr*cellH, pixH)
			y1 := b.Min.Y + min(tr*cellH+cellH, pixH) - 1
			if x1 < x0 || y1 < y0 {
				continue
			}

			cell := FindBestCell(img, b, x0, x1, y0, y1, mode)

			if cell.BG.A != 0 {
				sb.WriteString(bgRGB(cell.BG))
			}
			if cell.FG.A != 0 {
				sb.WriteString(fgRGB(cell.FG))
			}
			sb.WriteRune(cell.Ch)
			sb.WriteString(ansiReset)
		}

		if _, err := fmt.Fprintln(w, sb.String()); err != nil {
			return fmt.Errorf("sparkline render: %w", err)
		}
	}
	return nil
}

// RenderToImage runs the same cell selection as RenderOpts but writes the
// reconstructed glyph image instead of ANSI escape codes.
func RenderToImage(img image.Image, outCols, outRows int, mode Mode) image.Image {
	b := img.Bounds()
	pixW := b.Dx()
	pixH := b.Dy()

	cellW := max(1, pixW/outCols)
	cellH := max(1, pixH/outRows)

	dst := image.NewRGBA(b)

	for tr := 0; tr < outRows; tr++ {
		for tc := 0; tc < outCols; tc++ {
			x0 := b.Min.X + min(tc*cellW, pixW)
			x1 := b.Min.X + min(tc*cellW+cellW, pixW) - 1
			y0 := b.Min.Y + min(tr*cellH, pixH)
			y1 := b.Min.Y + min(tr*cellH+cellH, pixH) - 1
			if x1 < x0 || y1 < y0 {
				continue
			}

			cell := FindBestCell(img, b, x0, x1, y0, y1, mode)
			cw := x1 - x0 + 1
			ch := y1 - y0 + 1

			for y := y0; y <= y1; y++ {
				for x := x0; x <= x1; x++ {
					src := toRGBA(img.At(x, y))
					if src.A == 0 {
						dst.Set(x, y, color.RGBA{})
						continue
					}
					c := cell.BG
					if maskContains(cell.Ch, x-x0, y-y0, cw, ch) {
						c = cell.FG
					}
					dst.Set(x, y, c)
				}
			}
		}
	}
	return dst
}

type cellResult struct {
	Ch  rune
	FG  color.RGBA
	BG  color.RGBA
	Err float64
}

type candidate struct {
	ch   rune
	mask func(x, y, w, h int) bool
}

func maskContains(ch rune, x, y, w, h int) bool {
	switch ch {
	case ' ':
		return false
	case '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█':
		level := map[rune]int{'▁': 1, '▂': 2, '▃': 3, '▄': 4, '▅': 5, '▆': 6, '▇': 7, '█': 8}[ch]
		return (h-y)*8 <= level*h
	case '▘':
		return x*2 < w && y*2 < h
	case '▝':
		return x*2 >= w && y*2 < h
	case '▖':
		return x*2 < w && y*2 >= h
	case '▗':
		return x*2 >= w && y*2 >= h
	case '▀':
		return y*2 < h
	case '▌':
		return x*2 < w
	case '▐':
		return x*2 >= w
	case '▚':
		return (x*2 < w && y*2 < h) || (x*2 >= w && y*2 >= h)
	case '▞':
		return (x*2 >= w && y*2 < h) || (x*2 < w && y*2 >= h)
	case '▛':
		return !(x*2 >= w && y*2 >= h)
	case '▜':
		return !(x*2 < w && y*2 >= h)
	case '▙':
		return !(x*2 >= w && y*2 < h)
	case '▟':
		return !(x*2 < w && y*2 < h)
	default:
		return true
	}
}

func verticalCandidates() []candidate {
	out := make([]candidate, 0, len(lowerBlocks))
	for i, ch := range lowerBlocks {
		k := i + 1
		out = append(out, candidate{
			ch: ch,
			mask: func(_ int, y int, _ int, h int) bool {
				return (h-y)*8 <= k*h
			},
		})
	}
	return out
}

func quadCandidates() []candidate {
	out := verticalCandidates()
	out = append(out,
		candidate{ch: ' ', mask: func(_, _, _, _ int) bool { return false }},
		candidate{ch: '▘', mask: quadMask(true, false, false, false)},
		candidate{ch: '▝', mask: quadMask(false, true, false, false)},
		candidate{ch: '▖', mask: quadMask(false, false, true, false)},
		candidate{ch: '▗', mask: quadMask(false, false, false, true)},
		candidate{ch: '▀', mask: quadMask(true, true, false, false)},
		candidate{ch: '▄', mask: quadMask(false, false, true, true)},
		candidate{ch: '▌', mask: quadMask(true, false, true, false)},
		candidate{ch: '▐', mask: quadMask(false, true, false, true)},
		candidate{ch: '▚', mask: quadMask(true, false, false, true)},
		candidate{ch: '▞', mask: quadMask(false, true, true, false)},
		candidate{ch: '▛', mask: quadMask(true, true, true, false)},
		candidate{ch: '▜', mask: quadMask(true, true, false, true)},
		candidate{ch: '▙', mask: quadMask(true, false, true, true)},
		candidate{ch: '▟', mask: quadMask(false, true, true, true)},
		candidate{ch: '█', mask: func(_, _, _, _ int) bool { return true }},
	)
	return out
}

func quadMask(ul, ur, ll, lr bool) func(x, y, w, h int) bool {
	return func(x, y, w, h int) bool {
		left := x*2 < w
		top := y*2 < h
		switch {
		case top && left:
			return ul
		case top && !left:
			return ur
		case !top && left:
			return ll
		default:
			return lr
		}
	}
}

// FindBestCell tries the active mode's glyph candidates for the pixel block
// [x0..x1] × [y0..y1] and returns the lowest-SSE reconstruction.
func FindBestCell(img image.Image, bounds image.Rectangle, x0, x1, y0, y1 int, mode Mode) cellResult {
	candidates := verticalCandidates()
	if mode == Quad {
		candidates = quadCandidates()
	}
	return findBestCandidate(img, bounds, x0, x1, y0, y1, candidates)
}

func findBestCandidate(img image.Image, _ image.Rectangle, x0, x1, y0, y1 int, candidates []candidate) cellResult {
	blockW := x1 - x0 + 1
	blockH := y1 - y0 + 1
	if blockW <= 0 || blockH <= 0 {
		return cellResult{Ch: ' ', Err: math.MaxFloat64}
	}

	best := cellResult{Ch: ' ', Err: math.MaxFloat64}
	bestFGTransparent := math.MaxInt
	bestSplitPenalty := math.MaxInt
	for _, cand := range candidates {
		var fgSum, bgSum acc
		var fgN, bgN, fgTransparent, bgTransparent int
		for y := 0; y < blockH; y++ {
			for x := 0; x < blockW; x++ {
				p := toRGBA(img.At(x0+x, y0+y))
				isFG := cand.mask(x, y, blockW, blockH)
				if p.A == 0 {
					if isFG {
						fgTransparent++
					} else {
						bgTransparent++
					}
					continue
				}
				if isFG {
					fgSum.r += float64(p.R)
					fgSum.g += float64(p.G)
					fgSum.b += float64(p.B)
					fgN++
				} else {
					bgSum.r += float64(p.R)
					bgSum.g += float64(p.G)
					bgSum.b += float64(p.B)
					bgN++
				}
			}
		}

		// avgColorOpaque returns transparent (A=0) when the region has no
		// opaque pixels; RenderOpts/RenderToImage treat A=0 as "no sequence".
		fgAvg := avgColorOpaque(fgSum, fgN)
		bgAvg := avgColorOpaque(bgSum, bgN)

		var err float64
		for y := 0; y < blockH; y++ {
			for x := 0; x < blockW; x++ {
				p := toRGBA(img.At(x0+x, y0+y))
				if p.A == 0 {
					continue // transparent pixels contribute no error
				}
				if cand.mask(x, y, blockW, blockH) {
					err += sqDist(p, fgAvg)
				} else {
					err += sqDist(p, bgAvg)
				}
			}
		}

		// Add a large per-pixel penalty for transparent source pixels that would
		// be covered by an emitted BG or FG colour sequence.  This makes
		// characters that map transparent pixels to the emitted region more
		// expensive than characters that isolate them in a non-emitted region
		// (bgAvg.A==0 → no BG sequence → no terminal colour bleeds into what
		// should be transparent rows).  The constant is 3×255² = the maximum
		// possible squared colour distance, large enough to dominate any real
		// colour error but without causing overflow on float64.
		const transparentPixelCost = 3 * 255 * 255
		if bgAvg.A != 0 {
			err += float64(bgTransparent) * transparentPixelCost
		}
		if fgAvg.A != 0 {
			err += float64(fgTransparent) * transparentPixelCost
		}

		// Secondary tiebreakers (applied in order when primary SSE+cost ties):
		// 1. Prefer fewer transparent pixels in the FG region (▀ over ▁).
		// 2. Prefer chars that emit at most one colour sequence (█ over ▁▂…▇
		//    when the block is a solid colour): splitPenalty=0 when BG is
		//    transparent (not emitted), 1 when both FG and BG are emitted.
		splitPenalty := 0
		if fgAvg.A != 0 && bgAvg.A != 0 {
			splitPenalty = 1
		}
		better := err < best.Err ||
			(err == best.Err && fgTransparent < bestFGTransparent) ||
			(err == best.Err && fgTransparent == bestFGTransparent && splitPenalty < bestSplitPenalty)
		if better {
			best = cellResult{Ch: cand.ch, FG: fgAvg, BG: bgAvg, Err: err}
			bestFGTransparent = fgTransparent
			bestSplitPenalty = splitPenalty
		}
	}
	return best
}

// avgColorOpaque returns the average of n opaque pixels. Returns transparent
// (A=0) when n==0 so callers can skip the ANSI colour sequence entirely and
// let the terminal's default background show through.
func avgColorOpaque(sum acc, n int) color.RGBA {
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{
		R: uint8(sum.r / float64(n)),
		G: uint8(sum.g / float64(n)),
		B: uint8(sum.b / float64(n)),
		A: 255,
	}
}

// FindOptimalSplit tries all character levels 0..7 for the pixel block
// [x0..x1] × [y0..y1] and returns:
//
//	bestK     — the character index (0..7) giving the lowest SSE
//	barColor  — average colour of the bar (filled) region
//	emptyColor — average colour of the empty region
//	bestErr   — total squared error
//
// Per-mode pixel ordering and region semantics:
//
//	Vertical: top→bottom, bar=bottom pixels, empty=top pixels
//
// Deprecated: use FindBestCell. This remains for tests and golden helpers that
// need the old scalar split result.
func FindOptimalSplit(img image.Image, bounds image.Rectangle, x0, x1, y0, y1 int, mode Mode) (bestK int, barColor, emptyColor color.RGBA, bestErr float64) {
	blockW := x1 - x0 + 1
	blockH := y1 - y0 + 1
	n := blockW * blockH
	if n <= 0 {
		return 0, color.RGBA{}, color.RGBA{}, math.MaxFloat64
	}

	pixels := readBlock(img, x0, x1, y0, y1, mode)

	pref := prefixSums(pixels)
	sum := pref[n]

	bestErr = math.MaxFloat64

	// trySplit evaluates character index ci and returns its SSE.
	// splitAt = number of pixels assigned to the FIRST region.
	trySplit := func(ci int, firstIsBar bool) (float64, color.RGBA, color.RGBA) {
		// bar occupies (ci+1)/8 of the cell.
		barN := n * (ci + 1) / 8
		if barN == 0 || barN >= n {
			return math.MaxFloat64, color.RGBA{}, color.RGBA{}
		}
		emptyN := n - barN

		var fgSum, bgSum acc
		var fgN, bgN int
		if firstIsBar {
			// first barN pixels = bar, last emptyN = empty
			fgSum, fgN = prefAcc(pref, 0, barN)
			bgSum, bgN = prefAcc(pref, barN, n)
		} else {
			// first emptyN pixels = empty, last barN = bar
			fgSum, fgN = prefAcc(pref, emptyN, n)
			bgSum, bgN = prefAcc(pref, 0, emptyN)
		}

		fgAvg := color.RGBA{
			R: uint8(fgSum.r / float64(fgN)),
			G: uint8(fgSum.g / float64(fgN)),
			B: uint8(fgSum.b / float64(fgN)),
			A: 255,
		}
		bgAvg := color.RGBA{
			R: uint8(bgSum.r / float64(bgN)),
			G: uint8(bgSum.g / float64(bgN)),
			B: uint8(bgSum.b / float64(bgN)),
			A: 255,
		}

		var err float64
		var p color.RGBA
		for i := 0; i < n; i++ {
			p = pixels[i]
			if firstIsBar {
				if i < barN {
					err += sqDist(p, fgAvg)
				} else {
					err += sqDist(p, bgAvg)
				}
			} else {
				if i < emptyN {
					err += sqDist(p, bgAvg)
				} else {
					err += sqDist(p, fgAvg)
				}
			}
		}

		return err, fgAvg, bgAvg
	}

	for ci := 0; ci < 8; ci++ {
		var err float64
		var bar, empty color.RGBA

		// bar at bottom = last region (second)
		// firstIsBar = false → first=empty, last=bar
		err, bar, empty = trySplit(ci, false)

		if err < bestErr {
			bestErr = err
			bestK = ci
			barColor = bar
			emptyColor = empty
		}
	}

	if bestErr == math.MaxFloat64 {
		avg := color.RGBA{
			R: uint8(sum.r / float64(n)),
			G: uint8(sum.g / float64(n)),
			B: uint8(sum.b / float64(n)),
			A: 255,
		}
		return 7, avg, avg, 0
	}

	return
}

type acc struct{ r, g, b float64 }

func readBlock(img image.Image, x0, x1, y0, y1 int, mode Mode) []color.RGBA {
	n := (x1 - x0 + 1) * (y1 - y0 + 1)
	p := make([]color.RGBA, 0, n)
	for by := y0; by <= y1; by++ {
		for bx := x0; bx <= x1; bx++ {
			p = append(p, toRGBA(img.At(bx, by)))
		}
	}
	return p
}

func prefixSums(pixels []color.RGBA) []acc {
	pref := make([]acc, len(pixels)+1)
	var sum acc
	for i, p := range pixels {
		sum.r += float64(p.R)
		sum.g += float64(p.G)
		sum.b += float64(p.B)
		pref[i+1] = sum
	}
	return pref
}

func prefAcc(pref []acc, start, end int) (acc, int) {
	a := pref[end]
	a.r -= pref[start].r
	a.g -= pref[start].g
	a.b -= pref[start].b
	return a, end - start
}

// blockChar returns the rune for mode and character index k (0..7).
func blockChar(mode Mode, k int) rune {
	if k < 0 {
		k = 0
	}
	if k > 7 {
		k = 7
	}
	switch mode {
	case Vertical:
		return lowerBlocks[k]
	default:
		return lowerBlocks[k]
	}
}

// sqDist returns the squared Euclidean distance between two colours.
func sqDist(a, b color.RGBA) float64 {
	dr := float64(int(a.R) - int(b.R))
	dg := float64(int(a.G) - int(b.G))
	db := float64(int(a.B) - int(b.B))
	return dr*dr + dg*dg + db*db
}

func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
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

func fgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func bgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}
