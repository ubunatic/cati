// Package quadblock renders images into the terminal using Unicode quadrant
// block characters (U+2596–U+259F) combined with 24-bit ANSI true-color
// escape sequences.
//
// Each terminal cell encodes a 2×2 pixel grid (UL, UR, LL, LR), doubling
// the horizontal resolution of half-block rendering.  The two-colour-per-cell
// constraint still applies: fg fills the marked quadrants, bg fills the rest.
//
// Use RenderOpts with an Options value to enable quality variants.
// Apply ReduceColors to the scaled image before rendering for palette modes.
package quadblock

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"ubunatic.com/cati/v1/core"
	"ubunatic.com/cati/v1/halfblock"
)

// ── ANSI helpers ──────────────────────────────────────────────────────────────

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K"
	ansiCarriageReturn = "\r"
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

func fgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func bgRGB(c color.RGBA) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

// ── Colour helpers ────────────────────────────────────────────────────────────

func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func isTransparent(c color.RGBA) bool { return c.A == 0 }

func eqRGB(a, b color.RGBA) bool { return a.R == b.R && a.G == b.G && a.B == b.B }

func colorDist2(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}

// avgRGB2 returns the arithmetic mean color of two pixels.
func avgRGB2(p1, p2 color.RGBA) color.RGBA {
	t1 := isTransparent(p1)
	t2 := isTransparent(p2)
	if t1 && t2 {
		return color.RGBA{}
	}
	if t1 {
		return p2
	}
	if t2 {
		return p1
	}
	return color.RGBA{
		R: uint8((int(p1.R) + int(p2.R)) / 2),
		G: uint8((int(p1.G) + int(p2.G)) / 2),
		B: uint8((int(p1.B) + int(p2.B)) / 2),
		A: 255,
	}
}

// avgRGB returns the arithmetic mean colour of the opaque pixels in the slice.
func avgRGB(pixels ...color.RGBA) color.RGBA {
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

// ── Options ───────────────────────────────────────────────────────────────────

// BlendMode controls when and how sub-pixel neighbourhood blending is applied.
type BlendMode int

const (
	// BlendNone samples each sub-pixel at its exact center (default).
	BlendNone BlendMode = iota
	// BlendAlways blends every sub-pixel with its 8 neighbours (3×3, weights 4:2:1).
	BlendAlways
	// BlendAmbiguous applies the 3×3 blend only when the cell has 3+ distinct colours.
	// Clean cells are left untouched; only ambiguous boundaries are smoothed.
	BlendAmbiguous
	// BlendAmbiguousWide applies a 5×5 (radius-2) blend on ambiguous cells.
	// Stronger smoothing over a larger neighbourhood; may soften fine edges.
	BlendAmbiguousWide
)

// Options configures quality trade-offs of the quad-block renderer.
type Options struct {
	// NoLinePrefix omits the erase-line and carriage-return prefix emitted
	// before each rendered line. Set this when composing output alongside
	// other content on the same terminal row.
	NoLinePrefix bool

	// HalfblockThreshold: when > 0, a cell whose best colour-pair exact
	// coverage (how many of the 4 pixels match fg or bg exactly, 0–4) is
	// below this value falls back to halfblock encoding (▀/▄ from top/bottom
	// row averages). For SplitHalf it also guards against unstable local masks.
	// It applies to cells with 3+ distinct colours in the general path.
	HalfblockThreshold int

	// Blend controls neighbourhood pixel blending.  See BlendMode constants.
	Blend BlendMode

	// SplitHalf derives the fg/bg colour pair from halfblock row-averages
	// (top-2-pixel avg vs bottom-2-pixel avg) and then applies the quad mask
	// for sub-cell precision.  Gives stable colours with higher spatial detail.
	SplitHalf bool

	// SplitHalfNeighbors extends SplitHalf: instead of always using the bottom
	// row average as bg, it also tries the fg/bg colours of already-rendered
	// left and above cells and picks whichever candidate yields the lowest
	// total quantisation error.  Has no effect when SplitHalf is false.
	SplitHalfNeighbors bool

	// LumSplit splits the 4 sub-pixels at their mean luminance (BT.601):
	// bright sub-pixels form the fg group, dark sub-pixels form the bg group.
	// Each group's colour is the average of the original pixel colours in that
	// group.  Gives stable colour regions driven by luminance structure.
	LumSplit bool

	// PCA2 selects fg/bg by projecting the 4 pixels onto the principal axis of
	// colour variance (power-iteration PCA on the 3×3 RGB covariance matrix).
	// Pixels above the mean projection form one group, below form the other.
	// Each group's colour is the mean of its member pixels.  This gives the
	// least-squares-optimal 2-colour linear partition for each cell.
	PCA2 bool

	// Diameter picks fg/bg by finding the two most distant pixels in RGB space,
	// grouping by nearest endpoint, and averaging each group. Equivalent to a
	// single-step k-means from the extremal initialisation. Fast and robust.
	Diameter bool

	// KMeans runs 2-centre k-means (initialised from the diameter endpoints)
	// for the given number of iterations. KMeans: 3 is usually sufficient for
	// convergence on 4 pixels. Finds the minimum-MSE 2-colour partition.
	KMeans int

	// EdgeSnap splits the 4 sub-pixels by the dominant luminance gradient
	// direction computed within the cell itself. The bright side of the gradient
	// becomes fg, the dark side becomes bg; each group's colour is its average.
	// This is most effective for cells that straddle a diagonal edge (PCB traces,
	// diagonal silhouettes) where other algorithms produce an averaged mis-aligned
	// colour. For nearly uniform cells it falls back to the diameter split.
	EdgeSnap bool
	// Rows constraints the number of terminal rows for scaling.
	Rows int

	// Jobs specifies the number of concurrent goroutines for rendering.
	Jobs int
	// OnProgress is called as rendered cells complete. Parallel rendering may
	// invoke it concurrently; callbacks should be quick and concurrency-safe.
	OnProgress func(core.Progress)
}

// ── Quadrant character lookup ─────────────────────────────────────────────────

// Quadrant bitmask: UL=bit3, UR=bit2, LL=bit1, LR=bit0.
const (
	bitUL = uint8(8) // 1000
	bitUR = uint8(4) // 0100
	bitLL = uint8(2) // 0010
	bitLR = uint8(1) // 0001
)

// quadChar maps a 4-bit mask to the Unicode character that renders those
// quadrants as fg colour.  Masks 5 (UR+LR) and 10 (UL+LL) have no exact
// Unicode codepoint; they are approximated with the nearest Hamming-1 char.
var quadChar = [16]rune{
	' ', // 0000: none → transparent / space
	'▗', // 0001: LR
	'▖', // 0010: LL
	'▄', // 0011: LL+LR  (bottom half)
	'▝', // 0100: UR
	'▟', // 0101: UR+LR  → approx ▟ (UR+LL+LR); no exact char for right column
	'▞', // 0110: UR+LL  (anti-diagonal)
	'▟', // 0111: UR+LL+LR
	'▘', // 1000: UL
	'▚', // 1001: UL+LR  (diagonal)
	'▙', // 1010: UL+LL  → approx ▙ (UL+LL+LR); no exact char for left column
	'▙', // 1011: UL+LL+LR
	'▀', // 1100: UL+UR  (top half)
	'▜', // 1101: UL+UR+LR
	'▛', // 1110: UL+UR+LL
	'█', // 1111: all
}

// ── Cell type ─────────────────────────────────────────────────────────────────

type quadCell struct {
	ch          rune
	fg, bg      color.RGBA
	hasFG       bool
	hasBG       bool
	transparent bool
}

// ── Colour quantisation ───────────────────────────────────────────────────────

// collectUnique returns the distinct non-transparent colours found in pixels
// written into a fixed-size array along with their frequencies and the count.
func collectUnique(pixels [4]color.RGBA) (out [4]color.RGBA, counts [4]int, n int) {
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		found := false
		for i := 0; i < n; i++ {
			if eqRGB(p, out[i]) {
				counts[i]++
				found = true
				break
			}
		}
		if !found {
			out[n] = p
			counts[n] = 1
			n++
		}
	}
	return out, counts, n
}

// pickBestPair selects fg and bg from candidates, scoring each pair by:
//   - coverage: pixels that match one of the two colours (weight 4)
//   - continuity: bonus when a colour already appears in a neighbour cell (weight 1)
//
// The higher-count colour becomes fg.
func pickBestPair(candidates [4]color.RGBA, counts [4]int, n int, left, above *quadCell) (fg, bg color.RGBA, hasBG bool) {
	if n == 1 {
		return candidates[0], color.RGBA{}, false
	}

	var m [4]int
	if left != nil && !left.transparent {
		if left.hasFG {
			for k := 0; k < n; k++ {
				if eqRGB(candidates[k], left.fg) {
					m[k]++
					break
				}
			}
		}
		if left.hasBG {
			for k := 0; k < n; k++ {
				if eqRGB(candidates[k], left.bg) {
					m[k]++
					break
				}
			}
		}
	}
	if above != nil && !above.transparent {
		if above.hasFG {
			for k := 0; k < n; k++ {
				if eqRGB(candidates[k], above.fg) {
					m[k]++
					break
				}
			}
		}
		if above.hasBG {
			for k := 0; k < n; k++ {
				if eqRGB(candidates[k], above.bg) {
					m[k]++
					break
				}
			}
		}
	}

	var weights [4]int
	for k := 0; k < n; k++ {
		weights[k] = counts[k]*4 + m[k]
	}

	bestI, bestJ := 0, 1
	bestScore := weights[0] + weights[1]

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if s := weights[i] + weights[j]; s > bestScore {
				bestScore = s
				bestI, bestJ = i, j
			}
		}
	}

	if counts[bestI] >= counts[bestJ] {
		return candidates[bestI], candidates[bestJ], true
	}
	return candidates[bestJ], candidates[bestI], true
}

// exactCoverage counts how many of the 4 pixels match fg or bg exactly.
func exactCoverage(pixels [4]color.RGBA, fg, bg color.RGBA, hasBG bool) int {
	n := 0
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if eqRGB(p, fg) || (hasBG && eqRGB(p, bg)) {
			n++
		}
	}
	return n
}

// halfblockFallback encodes the cell using row-average colours and halfblock
// chars, trading quad precision for clean colours at ambiguous boundaries.
func halfblockFallback(pixels [4]color.RGBA) quadCell {
	top := avgRGB2(pixels[0], pixels[1])
	bot := avgRGB2(pixels[2], pixels[3])
	topT := isTransparent(top)
	botT := isTransparent(bot)
	switch {
	case topT && botT:
		return quadCell{ch: ' ', transparent: true}
	case topT:
		return maybeVerticalize(pixels, quadCell{ch: '▄', fg: bot, hasFG: true})
	case botT:
		return maybeVerticalize(pixels, quadCell{ch: '▀', fg: top, hasFG: true})
	default:
		if eqRGB(top, bot) {
			return maybeVerticalize(pixels, quadCell{ch: '█', fg: top, hasFG: true})
		}
		return maybeVerticalize(pixels, quadCell{ch: '▀', fg: top, bg: bot, hasFG: true, hasBG: true})
	}
}

// splitHalfCell encodes the cell using halfblock row-averages as fg/bg colours,
// then applies the quad mask for sub-cell precision.
// When withNeighbors is true it also considers the fg/bg of left/above cells as
// candidate bg colours, picking the one with the lowest total quantisation error.
func splitHalfCell(pixels [4]color.RGBA, left, above *quadCell, withNeighbors bool, threshold int) quadCell {
	top := avgRGB2(pixels[0], pixels[1]) // UL+UR average → top colour
	bot := avgRGB2(pixels[2], pixels[3]) // LL+LR average → bottom colour
	topT := isTransparent(top)
	botT := isTransparent(bot)

	var fg, bg color.RGBA
	hasBG := false
	switch {
	case topT && botT:
		return quadCell{ch: ' ', transparent: true}
	case topT:
		fg = bot
	case botT:
		fg = top
	case eqRGB(top, bot):
		fg = top
	default:
		fg, bg, hasBG = top, bot, true
	}

	// When neighbor colors are requested, try each neighbor's fg/bg as an
	// alternative bg candidate and keep the one with lowest quantisation error.
	if withNeighbors && hasBG {
		best := bg
		bestErr := quantError(pixels, fg, bg)
		checkNB := func(nb *quadCell) {
			if nb == nil || nb.transparent {
				return
			}
			if nb.hasFG && !eqRGB(nb.fg, fg) {
				if e := quantError(pixels, fg, nb.fg); e < bestErr {
					best = nb.fg
					bestErr = e
				}
			}
			if nb.hasBG && !eqRGB(nb.bg, fg) {
				if e := quantError(pixels, fg, nb.bg); e < bestErr {
					best = nb.bg
					bestErr = e
				}
			}
		}
		checkNB(left)
		checkNB(above)
		bg = best
	}

	mask := buildMask(pixels, fg, bg, hasBG)
	// The split-half colour pair is deliberately stable, but applying a quad
	// mask to a poorly represented cell creates isolated noisy quadrants.  Let
	// callers request the same conservative halfblock fallback used by the
	// general quantiser; this was previously skipped by the SplitHalf fast path.
	if hasBG && threshold > 0 {
		_, _, uniqueN := collectUnique(pixels)
		if uniqueN > 2 && exactCoverage(pixels, fg, bg, true) < threshold {
			return halfblockFallback(pixels)
		}
	}
	if mask == 0 {
		if !hasBG {
			return quadCell{ch: ' ', transparent: true}
		}
		fg, bg = bg, fg
		hasBG = false
		mask = 0b1111
	}
	c := quadCell{ch: quadChar[mask], fg: fg, hasFG: true}
	if hasBG {
		c.bg = bg
		c.hasBG = true
	}
	return maybeVerticalize(pixels, c)
}

// compileCellLumSplit splits sub-pixels at their mean BT.601 luminance:
// bright pixels form the fg group, dark pixels form the bg group.
// Each group's colour is the average of its original pixel colours.
func compileCellLumSplit(pixels [4]color.RGBA) quadCell {
	lum := func(p color.RGBA) float64 {
		return 0.299*float64(p.R) + 0.587*float64(p.G) + 0.114*float64(p.B)
	}

	var sumL float64
	n := 0
	for _, p := range pixels {
		if !isTransparent(p) {
			sumL += lum(p)
			n++
		}
	}
	if n == 0 {
		return quadCell{ch: ' ', transparent: true}
	}
	if n == 1 {
		for _, p := range pixels {
			if !isTransparent(p) {
				return quadCell{ch: '█', fg: p, hasFG: true}
			}
		}
	}

	thresh := sumL / float64(n)

	var high, low []color.RGBA
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if lum(p) >= thresh {
			high = append(high, p)
		} else {
			low = append(low, p)
		}
	}

	if len(high) == 0 {
		high = low
		low = nil
	}
	fg := avgRGB(high...)

	if len(low) == 0 {
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: fg, hasFG: true})
	}

	bg := avgRGB(low...)
	if eqRGB(fg, bg) {
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: fg, hasFG: true})
	}

	mask := buildMask(pixels, fg, bg, true)
	if mask == 0 {
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: bg, hasFG: true})
	}
	return maybeVerticalize(pixels, quadCell{ch: quadChar[mask], fg: fg, bg: bg, hasFG: true, hasBG: true})
}

// quantError returns the sum of squared distances from each non-transparent
// pixel to its nearest colour among fg and bg.  Lower is better.
func quantError(pixels [4]color.RGBA, fg, bg color.RGBA) int {
	total := 0
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		dFG := colorDist2(p, fg)
		dBG := colorDist2(p, bg)
		if dFG < dBG {
			total += dFG
		} else {
			total += dBG
		}
	}
	return total
}

// buildMask computes the 4-bit quadrant mask (UL=bit3, UR=bit2, LL=bit1, LR=bit0).
func buildMask(pixels [4]color.RGBA, fg, bg color.RGBA, hasBG bool) uint8 {
	bits := [4]uint8{bitUL, bitUR, bitLL, bitLR}
	var mask uint8
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if !hasBG || colorDist2(p, fg) <= colorDist2(p, bg) {
			mask |= bits[i]
		}
	}
	return mask
}

// compileCellPCA2 finds the least-squares-optimal 2-colour partition of the
// cell's pixels. It projects each pixel onto the first principal axis of the
// RGB covariance matrix (power iteration, 8 steps), splits at the mean
// projection, and uses per-group averages as fg/bg colours.
func compileCellPCA2(pixels [4]color.RGBA) quadCell {
	// Collect non-transparent pixels, tracking their quadrant index.
	var opaque [4]bool
	var n int
	for i, p := range pixels {
		if !isTransparent(p) {
			opaque[i] = true
			n++
		}
	}
	switch n {
	case 0:
		return quadCell{ch: ' ', transparent: true}
	case 1:
		for i, p := range pixels {
			if opaque[i] {
				bits := [4]uint8{bitUL, bitUR, bitLL, bitLR}
				return quadCell{ch: quadChar[bits[i]], fg: p, hasFG: true}
			}
		}
	}

	// Mean RGB.
	var mu [3]float64
	for i, p := range pixels {
		if opaque[i] {
			mu[0] += float64(p.R)
			mu[1] += float64(p.G)
			mu[2] += float64(p.B)
		}
	}
	fn := float64(n)
	mu[0] /= fn
	mu[1] /= fn
	mu[2] /= fn

	// 3×3 RGB covariance matrix.
	var cov [3][3]float64
	for i, p := range pixels {
		if !opaque[i] {
			continue
		}
		d := [3]float64{float64(p.R) - mu[0], float64(p.G) - mu[1], float64(p.B) - mu[2]}
		for r := range 3 {
			for c := range 3 {
				cov[r][c] += d[r] * d[c]
			}
		}
	}

	// Power iteration for the principal eigenvector (8 steps, starts at (1,1,1)).
	v := [3]float64{1, 1, 1}
	for range 8 {
		nv := [3]float64{
			cov[0][0]*v[0] + cov[0][1]*v[1] + cov[0][2]*v[2],
			cov[1][0]*v[0] + cov[1][1]*v[1] + cov[1][2]*v[2],
			cov[2][0]*v[0] + cov[2][1]*v[1] + cov[2][2]*v[2],
		}
		l := math.Sqrt(nv[0]*nv[0] + nv[1]*nv[1] + nv[2]*nv[2])
		if l < 1e-8 {
			break
		}
		v = [3]float64{nv[0] / l, nv[1] / l, nv[2] / l}
	}

	// Project each pixel onto v and split at mean projection.
	var projs [4]float64
	var projSum float64
	for i, p := range pixels {
		if opaque[i] {
			pr := v[0]*float64(p.R) + v[1]*float64(p.G) + v[2]*float64(p.B)
			projs[i] = pr
			projSum += pr
		}
	}
	mid := projSum / fn

	bits := [4]uint8{bitUL, bitUR, bitLL, bitLR}
	var mask uint8
	var fgPx, bgPx []color.RGBA
	for i, p := range pixels {
		if !opaque[i] {
			continue
		}
		if projs[i] >= mid {
			mask |= bits[i]
			fgPx = append(fgPx, p)
		} else {
			bgPx = append(bgPx, p)
		}
	}

	fg := avgRGB(fgPx...)
	if len(bgPx) == 0 {
		return maybeVerticalize(pixels, quadCell{ch: quadChar[0b1111], fg: fg, hasFG: true})
	}
	bg := avgRGB(bgPx...)
	if eqRGB(fg, bg) {
		return maybeVerticalize(pixels, quadCell{ch: quadChar[0b1111], fg: fg, hasFG: true})
	}
	return maybeVerticalize(pixels, quadCell{ch: quadChar[mask], fg: fg, bg: bg, hasFG: true, hasBG: true})
}

// compileCell converts a 2×2 pixel block into a terminal quadrant cell.
func compileCell(pixels [4]color.RGBA, left, above *quadCell, opts Options) quadCell {
	if opts.KMeans > 0 {
		return compileCellKMeans(pixels, opts.KMeans)
	}
	if opts.EdgeSnap {
		return compileCellEdgeSnap(pixels)
	}
	if opts.Diameter {
		return compileCellDiameter(pixels)
	}
	if opts.PCA2 {
		return compileCellPCA2(pixels)
	}
	if opts.LumSplit {
		return compileCellLumSplit(pixels)
	}
	if opts.SplitHalf {
		cell := splitHalfCell(pixels, left, above, opts.SplitHalfNeighbors, opts.HalfblockThreshold)
		return cell
	}

	uniqueArr, countsArr, numUnique := collectUnique(pixels)

	if numUnique == 0 {
		return quadCell{ch: ' ', transparent: true}
	}

	var fg, bg color.RGBA
	hasBG := false

	switch numUnique {
	case 1:
		fg = uniqueArr[0]
	case 2:
		fg, bg = uniqueArr[0], uniqueArr[1]
		hasBG = true
	default:
		fg, bg, hasBG = pickBestPair(uniqueArr, countsArr, numUnique, left, above)
		if opts.HalfblockThreshold > 0 {
			if exactCoverage(pixels, fg, bg, hasBG) < opts.HalfblockThreshold {
				return halfblockFallback(pixels)
			}
		}
	}

	mask := buildMask(pixels, fg, bg, hasBG)

	if mask == 0 {
		if !hasBG {
			return quadCell{ch: ' ', transparent: true}
		}
		fg, bg = bg, fg
		hasBG = false
		mask = 0b1111
	}

	c := quadCell{ch: quadChar[mask], fg: fg, hasFG: true}
	if hasBG {
		c.bg = bg
		c.hasBG = true
	}
	return maybeVerticalize(pixels, c)
}

// ── Scaling ───────────────────────────────────────────────────────────────────

// ScaleToFit scales img for quad rendering within the given terminal dimensions.
// Applies a 2× horizontal stretch to compensate for the 1:2 quad-pixel aspect
// ratio (terminal cells are ~1:2 W:H, so quad pixels are narrow).
// Pass 0 for either dimension to leave it unconstrained.
func ScaleToFit(img image.Image, cols, rows int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return img
	}

	maxW := cols * 2
	maxH := rows * 2
	stretchedW := srcW * 2
	targetW, targetH := stretchedW, srcH

	if maxW > 0 && targetW > maxW {
		targetH = srcH * maxW / stretchedW
		targetW = maxW
	}
	if maxH > 0 && targetH > maxH {
		targetW = stretchedW * maxH / srcH
		targetH = maxH
	}
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}
	return halfblock.ScaleNN(img, targetW, targetH)
}

// ── Pixel sampling ────────────────────────────────────────────────────────────

// safePixel returns the RGBA value at (x,y), or transparent if out of bounds.
func safePixel(img image.Image, x, y int, b image.Rectangle) color.RGBA {
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return color.RGBA{}
	}
	if rgba, ok := img.(*image.RGBA); ok && core.Fastpath {
		p := rgba.Pix[rgba.PixOffset(x, y):]
		if p[3] == 0 {
			return color.RGBA{}
		}
		return color.RGBA{R: p[0], G: p[1], B: p[2], A: p[3]}
	}
	return toRGBA(img.At(x, y))
}

// blendedPixelR returns a distance-weighted average of the pixel at (x,y)
// and all pixels within the given radius.
// Weights: center=4, dist²≤2 (cardinal at r=1)=2, all others=1.
// Transparent pixels are excluded from the average.
func blendedPixelR(img image.Image, x, y int, b image.Rectangle, radius int) color.RGBA {
	var r, g, bl, tw int
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			dist2 := dx*dx + dy*dy
			var w int
			switch {
			case dist2 == 0:
				w = 4
			case dist2 <= 2:
				w = 2
			default:
				w = 1
			}
			px := safePixel(img, x+dx, y+dy, b)
			if isTransparent(px) {
				continue
			}
			r += int(px.R) * w
			g += int(px.G) * w
			bl += int(px.B) * w
			tw += w
		}
	}
	if tw == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r / tw), G: uint8(g / tw), B: uint8(bl / tw), A: 255}
}

// samplePixel returns the pixel colour at (x,y) according to opts.Blend.
// BlendAmbiguous / BlendAmbiguousWide are not handled here; they are applied
// in RenderOpts after the initial 4-pixel read detects ambiguity.
func samplePixel(img image.Image, x, y int, b image.Rectangle, opts Options) color.RGBA {
	if opts.Blend == BlendAlways {
		return blendedPixelR(img, x, y, b, 1)
	}
	return safePixel(img, x, y, b)
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// RenderToGrid scales and converts the image into a grid of terminal cells.
func RenderToGrid(img image.Image, cols int, opts Options) (*core.Grid, error) {
	var scaled image.Image
	if cols > 0 || opts.Rows > 0 {
		scaled = ScaleToFit(img, cols, opts.Rows)
	} else {
		scaled = img
	}

	b := scaled.Bounds()
	pixW := b.Dx()
	pixH := b.Dy()

	tcCols := (pixW + 1) / 2
	trRows := (pixH + 1) / 2

	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = 1
	}

	progress := core.NewProgressReporter(opts.OnProgress, tcCols*trRows)
	qCells, err := computeQuadCellsJ(scaled, b, opts, tcCols, trRows, jobs, progress)
	if err != nil {
		return nil, err
	}

	cells := make([][]core.Cell, trRows)
	for tr := 0; tr < trRows; tr++ {
		cells[tr] = make([]core.Cell, tcCols)
		for tc := 0; tc < tcCols; tc++ {
			qc := qCells[tr*tcCols+tc]
			cells[tr][tc] = core.Cell{
				Ch:          qc.ch,
				Fg:          qc.fg,
				Bg:          qc.bg,
				HasFg:       qc.hasFG,
				HasBg:       qc.hasBG,
				Transparent: qc.transparent,
			}
		}
	}

	return &core.Grid{
		Width:  tcCols,
		Height: trRows,
		Cells:  cells,
	}, nil
}

// Render writes img to w as ANSI quadrant-block art.
// By default each line starts with an erase-line/carriage-return prefix for
// standalone redraws; set Options.NoLinePrefix when composing output with
// other content on the same terminal row.
func Render(w io.Writer, img image.Image, cols int, opts Options) error {
	grid, err := RenderToGrid(img, cols, opts)
	if err != nil {
		return err
	}

	buf := make([]byte, 0, grid.Width*32)
	for y := 0; y < grid.Height; y++ {
		buf = buf[:0]
		if !opts.NoLinePrefix {
			buf = append(buf, ansiLinePrefix...)
		}
		for x := 0; x < grid.Width; x++ {
			c := grid.Cells[y][x]
			if c.Transparent {
				buf = append(buf, ' ')
				continue
			}
			if core.Fastpath {
				if c.HasBg {
					buf = appendBgRGB(buf, c.Bg)
				}
				buf = appendFgRGB(buf, c.Fg)
			} else {
				if c.HasBg {
					buf = append(buf, bgRGB(c.Bg)...)
				}
				buf = append(buf, fgRGB(c.Fg)...)
			}
			buf = utf8.AppendRune(buf, c.Ch)
			buf = append(buf, ansiReset...)
		}
		buf = append(buf, '\n')
		if _, err := w.Write(buf); err != nil {
			return fmt.Errorf("quadblock render: %w", err)
		}
	}
	return nil
}

// RenderOpts is a compatibility wrapper for Render.
func RenderOpts(w io.Writer, img image.Image, opts Options) error {
	b := img.Bounds()
	cols := (b.Dx() + 1) / 2
	return Render(w, img, cols, opts)
}

// charToMask reverses quadChar: given the Unicode character chosen by
// compileCell, return the 4-bit mask (UL=bit3, UR=bit2, LL=bit1, LR=bit0)
// that says which quadrants are fg.
func charToMask(ch rune) uint8 {
	switch ch {
	case ' ':
		return 0
	case '▗':
		return 1
	case '▖':
		return 2
	case '▄':
		return 3
	case '▝':
		return 4
	case '▞':
		return 6
	case '▟':
		return 7 // or 5
	case '▘':
		return 8
	case '▚':
		return 9
	case '▙':
		return 11 // or 10
	case '▀':
		return 12
	case '▜':
		return 13
	case '▛':
		return 14
	case '█':
		return 15
	default:
		return 0
	}
}

// RenderToImage runs the same cell-compilation as RenderOpts but writes the
// result into an image.RGBA instead of ANSI escape codes. Each 2×2 pixel block
// in the output shows the fg/bg colours assigned by compileCell, so the image
// is a faithful reconstruction of exactly what the terminal would display.
// This is the correct test signal for SSIM computation.
func RenderToImage(img image.Image, opts Options) *image.RGBA {
	return RenderToImageJ(img, opts, opts.Jobs)
}

// RenderJ is a compatibility wrapper.
func RenderJ(w io.Writer, img image.Image, opts Options, jobs int) error {
	opts.Jobs = jobs
	b := img.Bounds()
	cols := (b.Dx() + 1) / 2
	return Render(w, img, cols, opts)
}

// RenderToImageJ is a worker-aware version of RenderToImage.
func RenderToImageJ(img image.Image, opts Options, jobs int) *image.RGBA {
	if jobs <= 0 {
		jobs = 1
	}
	b := img.Bounds()
	pixW, pixH := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, pixW, pixH))

	tcCols := (pixW + 1) / 2
	trRows := (pixH + 1) / 2
	progress := core.NewProgressReporter(opts.OnProgress, tcCols*trRows)
	cells, err := computeQuadCellsJ(img, b, opts, tcCols, trRows, jobs, progress)
	if err != nil {
		return dst
	}

	if core.Fastpath {
		for tr := 0; tr < trRows; tr++ {
			py0 := tr * 2
			py1 := py0 + 1
			row0Off := py0 * dst.Stride
			row1Off := py1 * dst.Stride
			for tc := 0; tc < tcCols; tc++ {
				c := cells[tr*tcCols+tc]
				px0 := tc * 2
				px1 := px0 + 1

				if px0 >= pixW || py0 >= pixH {
					continue
				}

				mask := charToMask(c.ch)
				bg := c.bg
				if !c.hasBG {
					bg = color.RGBA{}
				}
				fg := c.fg
				if !c.hasFG {
					fg = bg
				}

				switch c.ch {
				case '▌':
					off00 := row0Off + px0*4
					off01 := row0Off + px1*4
					off10 := row1Off + px0*4
					off11 := row1Off + px1*4

					if px0 < pixW && py0 < pixH {
						src := safePixel(img, b.Min.X+px0, b.Min.Y+py0, b)
						if src.A != 0 {
							dst.Pix[off00] = fg.R
							dst.Pix[off00+1] = fg.G
							dst.Pix[off00+2] = fg.B
							dst.Pix[off00+3] = fg.A
						}
					}
					if px1 < pixW && py0 < pixH {
						src := safePixel(img, b.Min.X+px1, b.Min.Y+py0, b)
						if src.A != 0 {
							dst.Pix[off01] = bg.R
							dst.Pix[off01+1] = bg.G
							dst.Pix[off01+2] = bg.B
							dst.Pix[off01+3] = bg.A
						}
					}
					if px0 < pixW && py1 < pixH {
						src := safePixel(img, b.Min.X+px0, b.Min.Y+py1, b)
						if src.A != 0 {
							dst.Pix[off10] = fg.R
							dst.Pix[off10+1] = fg.G
							dst.Pix[off10+2] = fg.B
							dst.Pix[off10+3] = fg.A
						}
					}
					if px1 < pixW && py1 < pixH {
						src := safePixel(img, b.Min.X+px1, b.Min.Y+py1, b)
						if src.A != 0 {
							dst.Pix[off11] = bg.R
							dst.Pix[off11+1] = bg.G
							dst.Pix[off11+2] = bg.B
							dst.Pix[off11+3] = bg.A
						}
					}
				case '▐':
					off00 := row0Off + px0*4
					off01 := row0Off + px1*4
					off10 := row1Off + px0*4
					off11 := row1Off + px1*4

					if px0 < pixW && py0 < pixH {
						src := safePixel(img, b.Min.X+px0, b.Min.Y+py0, b)
						if src.A != 0 {
							dst.Pix[off00] = bg.R
							dst.Pix[off00+1] = bg.G
							dst.Pix[off00+2] = bg.B
							dst.Pix[off00+3] = bg.A
						}
					}
					if px1 < pixW && py0 < pixH {
						src := safePixel(img, b.Min.X+px1, b.Min.Y+py0, b)
						if src.A != 0 {
							dst.Pix[off01] = fg.R
							dst.Pix[off01+1] = fg.G
							dst.Pix[off01+2] = fg.B
							dst.Pix[off01+3] = fg.A
						}
					}
					if px0 < pixW && py1 < pixH {
						src := safePixel(img, b.Min.X+px0, b.Min.Y+py1, b)
						if src.A != 0 {
							dst.Pix[off10] = bg.R
							dst.Pix[off10+1] = bg.G
							dst.Pix[off10+2] = bg.B
							dst.Pix[off10+3] = bg.A
						}
					}
					if px1 < pixW && py1 < pixH {
						src := safePixel(img, b.Min.X+px1, b.Min.Y+py1, b)
						if src.A != 0 {
							dst.Pix[off11] = fg.R
							dst.Pix[off11+1] = fg.G
							dst.Pix[off11+2] = fg.B
							dst.Pix[off11+3] = fg.A
						}
					}
				default:
					qBit := [4]uint8{bitUL, bitUR, bitLL, bitLR}
					qPX := [4]int{px0, px1, px0, px1}
					qPY := [4]int{py0, py0, py1, py1}
					qOff := [4]int{
						row0Off + px0*4,
						row0Off + px1*4,
						row1Off + px0*4,
						row1Off + px1*4,
					}

					for q := 0; q < 4; q++ {
						px, py := qPX[q], qPY[q]
						if px < pixW && py < pixH {
							src := safePixel(img, b.Min.X+px, b.Min.Y+py, b)
							if src.A != 0 {
								target := bg
								if mask&qBit[q] != 0 {
									target = fg
								}
								off := qOff[q]
								dst.Pix[off] = target.R
								dst.Pix[off+1] = target.G
								dst.Pix[off+2] = target.B
								dst.Pix[off+3] = target.A
							}
						}
					}
				}
			}
		}
		return dst
	}

	quadBit := [4]uint8{bitUL, bitUR, bitLL, bitLR}
	qDX := [4]int{0, 1, 0, 1}
	qDY := [4]int{0, 0, 1, 1}

	for tr := 0; tr < trRows; tr++ {
		for tc := 0; tc < tcCols; tc++ {
			c := cells[tr*tcCols+tc]
			px0 := b.Min.X + tc*2
			py0 := b.Min.Y + tr*2
			if px0 >= pixW || py0 >= pixH {
				continue
			}

			mask := charToMask(c.ch)
			bg := c.bg
			if !c.hasBG {
				bg = color.RGBA{}
			}
			fg := c.fg
			if !c.hasFG {
				fg = bg
			}
			switch c.ch {
			case '▌':
				for q, dx := range []int{0, 1, 0, 1} {
					dy := qDY[q]
					px, py := tc*2+dx, tr*2+dy
					if px < pixW && py < pixH {
						src := safePixel(img, b.Min.X+px, b.Min.Y+py, b)
						if src.A == 0 {
							dst.SetRGBA(px, py, color.RGBA{})
						} else if dx == 0 {
							dst.SetRGBA(px, py, fg)
						} else {
							dst.SetRGBA(px, py, bg)
						}
					}
				}
			case '▐':
				for q, dx := range []int{0, 1, 0, 1} {
					dy := qDY[q]
					px, py := tc*2+dx, tr*2+dy
					if px < pixW && py < pixH {
						src := safePixel(img, b.Min.X+px, b.Min.Y+py, b)
						if src.A == 0 {
							dst.SetRGBA(px, py, color.RGBA{})
						} else if dx == 1 {
							dst.SetRGBA(px, py, fg)
						} else {
							dst.SetRGBA(px, py, bg)
						}
					}
				}
			default:
				for q, bit := range quadBit {
					dx, dy := qDX[q], qDY[q]
					px, py := tc*2+dx, tr*2+dy
					if px < pixW && py < pixH {
						src := safePixel(img, b.Min.X+px, b.Min.Y+py, b)
						if src.A == 0 {
							dst.SetRGBA(px, py, color.RGBA{})
						} else if mask&bit != 0 {
							dst.SetRGBA(px, py, fg)
						} else {
							dst.SetRGBA(px, py, bg)
						}
					}
				}
			}
		}
	}
	return dst
}

func computeQuadCellsJ(img image.Image, b image.Rectangle, opts Options, tcCols, trRows, jobs int, progress *core.ProgressReporter) ([]quadCell, error) {
	cells := make([]quadCell, tcCols*trRows)
	if jobs <= 1 || tcCols == 0 || trRows == 0 {
		for tr := 0; tr < trRows; tr++ {
			for tc := 0; tc < tcCols; tc++ {
				cells[tr*tcCols+tc] = computeQuadCell(img, b, opts, cells, tr, tc, tcCols, trRows)
				if opts.OnProgress != nil {
					progress.Done()
				}
			}
		}
		return cells, nil
	}

	workerN := jobs
	if workerN > core.MaxWorkers() {
		workerN = core.MaxWorkers()
	}
	if workerN > 10 {
		workerN = 10
	}
	if workerN > trRows {
		workerN = trRows
	}

	rowProgress := make([]atomic.Int32, trRows)
	var nextRow atomic.Int32
	var wg sync.WaitGroup

	for range workerN {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				tr := int(nextRow.Add(1) - 1)
				if tr >= trRows {
					return
				}
				for tc := 0; tc < tcCols; tc++ {
					if tr > 0 {
						target := int32(tc + 1)
						for rowProgress[tr-1].Load() < target {
							runtime.Gosched()
						}
					}
					cells[tr*tcCols+tc] = computeQuadCell(img, b, opts, cells, tr, tc, tcCols, trRows)
					rowProgress[tr].Store(int32(tc + 1))
					if opts.OnProgress != nil {
						progress.Done()
					}
				}
			}
		}()
	}

	wg.Wait()
	return cells, nil
}

// computeQuadCell compiles a single quad block cell at position (tc, tr).
func computeQuadCell(img image.Image, b image.Rectangle, opts Options, cells []quadCell, tr, tc, tcCols, trRows int) quadCell {
	py0 := b.Min.Y + tr*2
	py1 := py0 + 1
	px0 := b.Min.X + tc*2
	px1 := px0 + 1

	var pixels [4]color.RGBA
	if core.Fastpath && opts.Blend == BlendNone {
		if rgba, ok := img.(*image.RGBA); ok {
			minX, minY := rgba.Rect.Min.X, rgba.Rect.Min.Y
			maxX, maxY := rgba.Rect.Max.X, rgba.Rect.Max.Y
			if px0 >= minX && px1 < maxX && py0 >= minY && py1 < maxY {
				stride := rgba.Stride
				pix := rgba.Pix
				off0 := (py0-minY)*stride + (px0-minX)*4
				off1 := (py1-minY)*stride + (px0-minX)*4
				if a := pix[off0+3]; a != 0 {
					pixels[0] = color.RGBA{R: pix[off0], G: pix[off0+1], B: pix[off0+2], A: a}
				}
				if a := pix[off0+7]; a != 0 {
					pixels[1] = color.RGBA{R: pix[off0+4], G: pix[off0+5], B: pix[off0+6], A: a}
				}
				if a := pix[off1+3]; a != 0 {
					pixels[2] = color.RGBA{R: pix[off1], G: pix[off1+1], B: pix[off1+2], A: a}
				}
				if a := pix[off1+7]; a != 0 {
					pixels[3] = color.RGBA{R: pix[off1+4], G: pix[off1+5], B: pix[off1+6], A: a}
				}
			} else {
				pixels[0] = safePixel(rgba, px0, py0, b)
				pixels[1] = safePixel(rgba, px1, py0, b)
				pixels[2] = safePixel(rgba, px0, py1, b)
				pixels[3] = safePixel(rgba, px1, py1, b)
			}
		} else if nrgba, ok := img.(*image.NRGBA); ok {
			minX, minY := nrgba.Rect.Min.X, nrgba.Rect.Min.Y
			maxX, maxY := nrgba.Rect.Max.X, nrgba.Rect.Max.Y
			if px0 >= minX && px1 < maxX && py0 >= minY && py1 < maxY {
				stride := nrgba.Stride
				pix := nrgba.Pix
				off0 := (py0-minY)*stride + (px0-minX)*4
				off1 := (py1-minY)*stride + (px0-minX)*4
				if a := pix[off0+3]; a != 0 {
					pixels[0] = color.RGBA{R: pix[off0], G: pix[off0+1], B: pix[off0+2], A: a}
				}
				if a := pix[off0+7]; a != 0 {
					pixels[1] = color.RGBA{R: pix[off0+4], G: pix[off0+5], B: pix[off0+6], A: a}
				}
				if a := pix[off1+3]; a != 0 {
					pixels[2] = color.RGBA{R: pix[off1], G: pix[off1+1], B: pix[off1+2], A: a}
				}
				if a := pix[off1+7]; a != 0 {
					pixels[3] = color.RGBA{R: pix[off1+4], G: pix[off1+5], B: pix[off1+6], A: a}
				}
			} else {
				pixels[0] = safePixel(nrgba, px0, py0, b)
				pixels[1] = safePixel(nrgba, px1, py0, b)
				pixels[2] = safePixel(nrgba, px0, py1, b)
				pixels[3] = safePixel(nrgba, px1, py1, b)
			}
		} else {
			pixels[0] = samplePixel(img, px0, py0, b, opts)
			pixels[1] = samplePixel(img, px1, py0, b, opts)
			pixels[2] = samplePixel(img, px0, py1, b, opts)
			pixels[3] = samplePixel(img, px1, py1, b, opts)
		}
	} else {
		pixels[0] = samplePixel(img, px0, py0, b, opts)
		pixels[1] = samplePixel(img, px1, py0, b, opts)
		pixels[2] = samplePixel(img, px0, py1, b, opts)
		pixels[3] = samplePixel(img, px1, py1, b, opts)
	}

	if opts.Blend == BlendAmbiguous || opts.Blend == BlendAmbiguousWide {
		if _, _, n := collectUnique(pixels); n >= 3 {
			radius := 1
			if opts.Blend == BlendAmbiguousWide {
				radius = 2
			}
			pixels[0] = blendedPixelR(img, px0, py0, b, radius)
			pixels[1] = blendedPixelR(img, px1, py0, b, radius)
			pixels[2] = blendedPixelR(img, px0, py1, b, radius)
			pixels[3] = blendedPixelR(img, px1, py1, b, radius)
		}
	}

	var leftCell, aboveCell *quadCell
	if tc > 0 {
		leftCell = &cells[tr*tcCols+tc-1]
	}
	if tr > 0 {
		aboveCell = &cells[(tr-1)*tcCols+tc]
	}

	return compileCell(pixels, leftCell, aboveCell, opts)
}
