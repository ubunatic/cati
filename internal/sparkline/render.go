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

// Render writes img to w as ANSI block-element art (lower-block orientation).
func Render(w io.Writer, img image.Image) error {
	b := img.Bounds()
	return RenderOpts(w, img, max(1, b.Dx()/8), max(1, b.Dy()/8), LowerHorizontal)
}

// RenderOpts writes img to w as ANSI block-element art.  The image should
// be at the resolution computed by pixCols(=termCols*8) × pixRows(=termRows*8)
// so that each cell can analyze up to an 8×8 pixel block.
// outCols and outRows are the number of terminal columns and rows to emit.
//
// For each cell the algorithm analyses every possible split level (0..7) and
// picks the one that minimises total squared colour error.  Per-mode rules:
//
//	LowerHorizontal: FG = bar (bottom), BG = empty (top)
//	UpperHorizontal: FG = empty (bottom), BG = bar (top) — inverted via swap
//	LeftVertical:    FG = bar (left),   BG = empty (right)
//	RightVertical:   FG = empty (left),   BG = bar (right) — inverted via swap
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
			x0 := min(tc*cellW, pixW)
			x1 := min(x0+cellW, pixW) - 1
			y0 := min(tr*cellH, pixH)
			y1 := min(y0+cellH, pixH) - 1
			if x1 < x0 || y1 < y0 {
				continue
			}

			bestK, barColor, emptyColor, _ := findOptimalSplit(img, b, x0, x1, y0, y1, mode)

			switch mode {
			case LowerHorizontal, LeftVertical:
				// natural alignment: FG = bar, BG = empty
				sb.WriteString(bgRGB(emptyColor))
				sb.WriteString(fgRGB(barColor))
			case UpperHorizontal, RightVertical:
				// inverted alignment: FG = empty, BG = bar
				sb.WriteString(bgRGB(barColor))
				sb.WriteString(fgRGB(emptyColor))
			}
			ch := blockChar(mode, bestK)
			sb.WriteRune(ch)
			sb.WriteString(ansiReset)
		}

		if _, err := fmt.Fprintln(w, sb.String()); err != nil {
			return fmt.Errorf("sparkline render: %w", err)
		}
	}
	return nil
}

// findOptimalSplit tries all character levels 0..7 for the pixel block
// [x0..x1] × [y0..y1] and returns:
//
//	bestK     — the character index (0..7) giving the lowest SSE
//	barColor  — average colour of the bar (filled) region
//	emptyColor — average colour of the empty region
//	bestErr   — total squared error
//
// Per-mode pixel ordering and region semantics:
//
//	LowerHorizontal: top→bottom, bar=bottom pixels, empty=top pixels
//	UpperHorizontal: top→bottom, bar=top pixels,  empty=bottom pixels
//	LeftVertical:    left→right, bar=left pixels,  empty=right pixels
//	RightVertical:   left→right, bar=right pixels, empty=left pixels
func findOptimalSplit(img image.Image, bounds image.Rectangle, x0, x1, y0, y1 int, mode Mode) (bestK int, barColor, emptyColor color.RGBA, bestErr float64) {
	blockW := x1 - x0 + 1
	blockH := y1 - y0 + 1
	n := blockW * blockH
	if n <= 0 {
		return 0, color.RGBA{}, color.RGBA{}, math.MaxFloat64
	}

	pixels := readBlock(img, x0, x1, y0, y1)

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

		if firstIsBar {
			return err, fgAvg, bgAvg
		}
		return err, fgAvg, bgAvg
	}

	for ci := 0; ci < 8; ci++ {
		var err float64
		var bar, empty color.RGBA

		switch mode {
		case LowerHorizontal:
			// bar at bottom = last region (second)
			// firstIsBar = false → first=empty, last=bar
			err, bar, empty = trySplit(ci, false)
		case UpperHorizontal:
			// bar at top = first region
			// firstIsBar = true  → first=bar, last=empty
			err, bar, empty = trySplit(ci, true)
		case LeftVertical:
			// bar at left = first region
			// firstIsBar = true  → first=bar, last=empty
			err, bar, empty = trySplit(ci, true)
		case RightVertical:
			// bar at right = last region
			// firstIsBar = false → first=empty, last=bar
			err, bar, empty = trySplit(ci, false)
		}

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

func readBlock(img image.Image, x0, x1, y0, y1 int) []color.RGBA {
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
	case LowerHorizontal, UpperHorizontal:
		return lowerBlocks[k]
	case LeftVertical, RightVertical:
		return leftBlocks[k]
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
