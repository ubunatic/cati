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
	"strings"

	"codeberg.org/ubunatic/cati/internal/halfblock"
)

// ── ANSI helpers ──────────────────────────────────────────────────────────────

const (
	ansiReset          = "\x1b[0m"
	ansiEraseLine      = "\x1b[2K"
	ansiCarriageReturn = "\r"
	ansiLinePrefix     = ansiEraseLine + ansiCarriageReturn
)

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
	// HalfblockThreshold: when > 0, a cell whose best colour-pair exact
	// coverage (how many of the 4 pixels match fg or bg exactly, 0–4) is
	// below this value falls back to halfblock encoding (▀/▄ from top/bottom
	// row averages).  Only applies when the cell has 3+ distinct colours.
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

// collectUnique returns the distinct non-transparent colours found in pixels.
func collectUnique(pixels [4]color.RGBA) []color.RGBA {
	var out []color.RGBA
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		found := false
		for _, u := range out {
			if eqRGB(p, u) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, p)
		}
	}
	return out
}

// pickBestPair selects fg and bg from candidates, scoring each pair by:
//   - coverage: pixels that exactly match one of the two colours (weight 4)
//   - continuity: bonus when a colour already appears in a neighbour cell (weight 1)
//
// The higher-count colour becomes fg.
func pickBestPair(pixels [4]color.RGBA, candidates []color.RGBA, left, above *quadCell) (fg, bg color.RGBA, hasBG bool) {
	if len(candidates) == 1 {
		return candidates[0], color.RGBA{}, false
	}

	type scored struct {
		a, b  color.RGBA
		score int
	}
	best := scored{a: candidates[0], b: candidates[1], score: -1}

	for i := range len(candidates) {
		for j := i + 1; j < len(candidates); j++ {
			ca, cb := candidates[i], candidates[j]

			coverage := 0
			for _, p := range pixels {
				if isTransparent(p) {
					continue
				}
				if eqRGB(p, ca) || eqRGB(p, cb) {
					coverage++
				}
			}

			continuity := 0
			for _, nb := range []*quadCell{left, above} {
				if nb == nil || nb.transparent {
					continue
				}
				if nb.hasFG && (eqRGB(nb.fg, ca) || eqRGB(nb.fg, cb)) {
					continuity++
				}
				if nb.hasBG && (eqRGB(nb.bg, ca) || eqRGB(nb.bg, cb)) {
					continuity++
				}
			}

			if s := coverage*4 + continuity; s > best.score {
				best = scored{a: ca, b: cb, score: s}
			}
		}
	}

	countA, countB := 0, 0
	for _, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if eqRGB(p, best.a) {
			countA++
		} else if eqRGB(p, best.b) {
			countB++
		}
	}
	if countA >= countB {
		return best.a, best.b, true
	}
	return best.b, best.a, true
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
	top := avgRGB(pixels[0], pixels[1])
	bot := avgRGB(pixels[2], pixels[3])
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
func splitHalfCell(pixels [4]color.RGBA, left, above *quadCell, withNeighbors bool) quadCell {
	top := avgRGB(pixels[0], pixels[1]) // UL+UR average → top colour
	bot := avgRGB(pixels[2], pixels[3]) // LL+LR average → bottom colour
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
		for _, nb := range []*quadCell{left, above} {
			if nb == nil || nb.transparent {
				continue
			}
			for _, c := range []struct {
				ok bool
				c  color.RGBA
			}{{nb.hasFG, nb.fg}, {nb.hasBG, nb.bg}} {
				if !c.ok || eqRGB(c.c, fg) {
					continue
				}
				if e := quantError(pixels, fg, c.c); e < bestErr {
					best = c.c
					bestErr = e
				}
			}
		}
		bg = best
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
		return splitHalfCell(pixels, left, above, opts.SplitHalfNeighbors)
	}

	unique := collectUnique(pixels)

	if len(unique) == 0 {
		return quadCell{ch: ' ', transparent: true}
	}

	var fg, bg color.RGBA
	hasBG := false

	switch len(unique) {
	case 1:
		fg = unique[0]
	case 2:
		fg, bg = unique[0], unique[1]
		hasBG = true
	default:
		fg, bg, hasBG = pickBestPair(pixels, unique, left, above)
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

// Render writes img to w as ANSI quadrant-block art with default options.
func Render(w io.Writer, img image.Image) error {
	return RenderOpts(w, img, Options{})
}

// RenderOpts writes img to w as ANSI quadrant-block art using the given options.
func RenderOpts(w io.Writer, img image.Image, opts Options) error {
	b := img.Bounds()
	pixW := b.Dx()
	pixH := b.Dy()

	tcCols := (pixW + 1) / 2
	trRows := (pixH + 1) / 2

	cells := make([]quadCell, tcCols*trRows)
	cellAt := func(tr, tc int) *quadCell {
		if tr < 0 || tc < 0 || tr >= trRows || tc >= tcCols {
			return nil
		}
		return &cells[tr*tcCols+tc]
	}

	for tr := range trRows {
		var sb strings.Builder
		sb.WriteString(ansiLinePrefix)

		for tc := range tcCols {
			py0 := b.Min.Y + tr*2
			py1 := py0 + 1
			px0 := b.Min.X + tc*2
			px1 := px0 + 1

			var pixels [4]color.RGBA
			pixels[0] = samplePixel(img, px0, py0, b, opts) // UL
			pixels[1] = samplePixel(img, px1, py0, b, opts) // UR
			pixels[2] = samplePixel(img, px0, py1, b, opts) // LL
			pixels[3] = samplePixel(img, px1, py1, b, opts) // LR

			// For ambiguous-blend modes: if first sampling gives 3+ colours,
			// re-sample with neighbourhood blending to reduce the colour count
			// before quantisation.
			if opts.Blend == BlendAmbiguous || opts.Blend == BlendAmbiguousWide {
				if len(collectUnique(pixels)) >= 3 {
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

			c := compileCell(pixels, cellAt(tr, tc-1), cellAt(tr-1, tc), opts)
			cells[tr*tcCols+tc] = c

			if c.transparent {
				sb.WriteRune(' ')
			} else {
				if c.hasBG {
					sb.WriteString(bgRGB(c.bg))
				}
				sb.WriteString(fgRGB(c.fg))
				sb.WriteRune(c.ch)
				sb.WriteString(ansiReset)
			}
		}

		if _, err := fmt.Fprintln(w, sb.String()); err != nil {
			return fmt.Errorf("quadblock render: %w", err)
		}
	}
	return nil
}

// charToMask reverses quadChar: given the Unicode character chosen by
// compileCell, return the 4-bit mask (UL=bit3, UR=bit2, LL=bit1, LR=bit0)
// that says which quadrants are fg.
var charToMask func(ch rune) uint8

func init() {
	m := make(map[rune]uint8, 16)
	for mask, ch := range quadChar {
		m[ch] = uint8(mask)
	}
	charToMask = func(ch rune) uint8 { return m[ch] }
}

// RenderToImage runs the same cell-compilation as RenderOpts but writes the
// result into an image.RGBA instead of ANSI escape codes. Each 2×2 pixel block
// in the output shows the fg/bg colours assigned by compileCell, so the image
// is a faithful reconstruction of exactly what the terminal would display.
// This is the correct test signal for SSIM computation.
func RenderToImage(img image.Image, opts Options) *image.RGBA {
	b := img.Bounds()
	pixW, pixH := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, pixW, pixH))

	tcCols := (pixW + 1) / 2
	trRows := (pixH + 1) / 2
	cells := make([]quadCell, tcCols*trRows)
	cellAt := func(tr, tc int) *quadCell {
		if tr < 0 || tc < 0 || tr >= trRows || tc >= tcCols {
			return nil
		}
		return &cells[tr*tcCols+tc]
	}

	// Quadrant pixel offsets within a 2×2 block: UL=0, UR=1, LL=2, LR=3.
	// Maps to bit positions: UL=bit3, UR=bit2, LL=bit1, LR=bit0.
	quadBit := [4]uint8{bitUL, bitUR, bitLL, bitLR}
	// dx,dy offsets for each quadrant index.
	qDX := [4]int{0, 1, 0, 1}
	qDY := [4]int{0, 0, 1, 1}

	for tr := range trRows {
		for tc := range tcCols {
			py0 := b.Min.Y + tr*2
			px0 := b.Min.X + tc*2

			var pixels [4]color.RGBA
			pixels[0] = samplePixel(img, px0, py0, b, opts)
			pixels[1] = samplePixel(img, px0+1, py0, b, opts)
			pixels[2] = samplePixel(img, px0, py0+1, b, opts)
			pixels[3] = samplePixel(img, px0+1, py0+1, b, opts)

			if opts.Blend == BlendAmbiguous || opts.Blend == BlendAmbiguousWide {
				if len(collectUnique(pixels)) >= 3 {
					radius := 1
					if opts.Blend == BlendAmbiguousWide {
						radius = 2
					}
					pixels[0] = blendedPixelR(img, px0, py0, b, radius)
					pixels[1] = blendedPixelR(img, px0+1, py0, b, radius)
					pixels[2] = blendedPixelR(img, px0, py0+1, b, radius)
					pixels[3] = blendedPixelR(img, px0+1, py0+1, b, radius)
				}
			}

			c := compileCell(pixels, cellAt(tr, tc-1), cellAt(tr-1, tc), opts)
			cells[tr*tcCols+tc] = c

			mask := charToMask(c.ch)
			bg := c.bg
			if !c.hasBG {
				bg = color.RGBA{} // transparent = terminal default bg
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
