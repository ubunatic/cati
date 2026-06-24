// Package quadblock renders images into the terminal using Unicode quadrant
// block characters (U+2596–U+259F) combined with 24-bit ANSI true-color
// escape sequences.
//
// Each terminal cell encodes a 2×2 pixel grid (UL, UR, LL, LR), doubling
// the horizontal resolution of half-block rendering.  The two-colour-per-cell
// constraint still applies: fg fills the marked quadrants, bg fills the rest.
//
// When a 2×2 block contains more than two distinct colours the renderer uses a
// neighbour-aware quantisation pass: every candidate colour pair is scored by
// coverage (how many pixels match exactly) and continuity (preference for
// colours already present in the left and above cells).  The pair with the
// highest combined score is chosen, keeping colour transitions smooth across
// cell boundaries.
package quadblock

import (
	"fmt"
	"image"
	"image/color"
	"io"
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
	transparent bool // no opaque pixels → plain space, no ANSI
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

	for i := 0; i < len(candidates); i++ {
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

	// Assign the more-common colour as fg.
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

// buildMask computes the 4-bit quadrant mask (UL=bit3, UR=bit2, LL=bit1, LR=bit0).
// Transparent pixels map to 0 (bg / terminal default).
// Non-transparent pixels are assigned to the nearer of fg/bg.
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

// compileCell converts a 2×2 pixel block into a terminal quadrant cell.
// left and above are the already-rendered cells to the left and above (may be nil).
func compileCell(pixels [4]color.RGBA, left, above *quadCell) quadCell {
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
	}

	mask := buildMask(pixels, fg, bg, hasBG)

	if mask == 0 {
		if !hasBG {
			return quadCell{ch: ' ', transparent: true}
		}
		// All opaque pixels quantised to bg; swap so fg colour fills all quadrants.
		fg, bg = bg, fg
		hasBG = false
		mask = 0b1111
	}

	c := quadCell{ch: quadChar[mask], fg: fg, hasFG: true}
	if hasBG {
		c.bg = bg
		c.hasBG = true
	}
	return c
}

// ── Scaling ───────────────────────────────────────────────────────────────────

// ScaleToFit scales img for quad rendering within the given terminal dimensions.
//
// Terminal cells are ~1:2 (W:H), so each quad pixel occupies cell_width/2 ×
// cell_width on screen — a 1:2 rectangle.  To compensate, the source image
// must be stretched 2× horizontally before rendering so that each source pixel
// maps to a square region.  ScaleToFit applies that stretch and then scales
// down to fit the pixel budget cols*2 × rows*2.  Upscaling the stretch is
// intentional; only downscaling beyond it is avoided.
// Pass 0 for either dimension to leave it unconstrained.
func ScaleToFit(img image.Image, cols, rows int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return img
	}

	maxW := cols * 2 // pixel budget: 2 px per terminal column
	maxH := rows * 2 // pixel budget: 2 px per terminal row

	// Treat the source as 2× wider when computing the scale factor so that the
	// resulting target width includes the aspect-ratio correction.
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

// ── Rendering ─────────────────────────────────────────────────────────────────

// safePixel returns the RGBA value at (x,y), or transparent if out of bounds.
func safePixel(img image.Image, x, y int, b image.Rectangle) color.RGBA {
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return color.RGBA{}
	}
	return toRGBA(img.At(x, y))
}

// Render writes img to w as ANSI quadrant-block art followed by a trailing
// newline per row.  Scale the image first with ScaleToFit if needed.
func Render(w io.Writer, img image.Image) error {
	b := img.Bounds()
	pixW := b.Dx()
	pixH := b.Dy()

	tcCols := (pixW + 1) / 2
	trRows := (pixH + 1) / 2

	// Cell grid for left/above neighbour lookup.
	cells := make([]quadCell, tcCols*trRows)
	cellAt := func(tr, tc int) *quadCell {
		if tr < 0 || tc < 0 || tr >= trRows || tc >= tcCols {
			return nil
		}
		return &cells[tr*tcCols+tc]
	}

	for tr := 0; tr < trRows; tr++ {
		var sb strings.Builder
		sb.WriteString(ansiLinePrefix)

		for tc := 0; tc < tcCols; tc++ {
			py0 := b.Min.Y + tr*2
			py1 := py0 + 1
			px0 := b.Min.X + tc*2
			px1 := px0 + 1

			var pixels [4]color.RGBA
			pixels[0] = safePixel(img, px0, py0, b) // UL
			pixels[1] = safePixel(img, px1, py0, b) // UR
			pixels[2] = safePixel(img, px0, py1, b) // LL
			pixels[3] = safePixel(img, px1, py1, b) // LR

			c := compileCell(pixels, cellAt(tr, tc-1), cellAt(tr-1, tc))
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
