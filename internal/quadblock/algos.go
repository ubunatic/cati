package quadblock

import "image/color"

// ── Diameter split ────────────────────────────────────────────────────────────

// compileCellDiameter picks fg/bg by finding the two most distant pixels in
// RGB space (the "diameter"), assigning each of the 4 pixels to its nearer
// endpoint, then averaging each group into the final fg/bg colour pair.
// This is equivalent to a single-step k-means initialised from the extreme
// points and minimises per-cell MSE without iteration.
func compileCellDiameter(pixels [4]color.RGBA) quadCell {
	var op [4]color.RGBA
	n := 0
	for _, p := range pixels {
		if !isTransparent(p) {
			op[n] = p
			n++
		}
	}
	switch n {
	case 0:
		return quadCell{ch: ' ', transparent: true}
	case 1:
		return quadCell{ch: '█', fg: op[0], hasFG: true}
	}

	// Find diameter: pair with maximum squared RGB distance.
	a, b := op[0], op[1]
	maxD := colorDist2(a, b)
	for i := range n {
		for j := i + 1; j < n; j++ {
			if d := colorDist2(op[i], op[j]); d > maxD {
				maxD = d
				a, b = op[i], op[j]
			}
		}
	}
	if maxD == 0 {
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: a, hasFG: true})
	}

	fg, bg := kmeansStep(op[:n], a, b)
	return twoColorCell(pixels, fg, bg)
}

// ── K-means ───────────────────────────────────────────────────────────────────

// compileCellKMeans picks fg/bg via 2-centre k-means, initialised from the
// diameter endpoints and iterated `iters` times. Finds the minimum-MSE
// 2-colour partition for the 4 sub-pixels.
func compileCellKMeans(pixels [4]color.RGBA, iters int) quadCell {
	var op [4]color.RGBA
	n := 0
	for _, p := range pixels {
		if !isTransparent(p) {
			op[n] = p
			n++
		}
	}
	switch n {
	case 0:
		return quadCell{ch: ' ', transparent: true}
	case 1:
		return quadCell{ch: '█', fg: op[0], hasFG: true}
	}

	// Initialise from diameter.
	a, b := op[0], op[1]
	maxD := colorDist2(a, b)
	for i := range n {
		for j := i + 1; j < n; j++ {
			if d := colorDist2(op[i], op[j]); d > maxD {
				maxD = d
				a, b = op[i], op[j]
			}
		}
	}
	if maxD == 0 {
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: a, hasFG: true})
	}

	for range iters {
		na, nb := kmeansStep(op[:n], a, b)
		if eqRGB(na, a) && eqRGB(nb, b) {
			break // converged
		}
		a, b = na, nb
	}
	return twoColorCell(pixels, a, b)
}

// ── Edge-snap ─────────────────────────────────────────────────────────────────

// compileCellEdgeSnap splits the 2×2 sub-pixels by the dominant luminance
// gradient direction inferred from the cell itself. Pixels on the bright side
// of the gradient become the fg group; pixels on the dark side become the bg
// group. Each group's colour is the average of its members. The gradient-derived
// fg/bg pair is then passed to twoColorCell, which uses buildMask to assign the
// final per-quadrant fg/bg bits.
//
// This is most effective for cells that straddle a sharp edge at a diagonal
// angle that other algorithms would blur or mis-align — e.g. PCB traces or
// diagonal object silhouettes. For nearly uniform cells it falls back to
// compileCellDiameter.
//
// Derivation: pixel positions relative to cell centre are
//
//	UL=(−½,−½)  UR=(+½,−½)  LL=(−½,+½)  LR=(+½,+½)
//
// With gx=(right−left column sums) and gy=(bottom−top row sums), define
// a=gx+gy and b=gx−gy. The dot product of each position with the gradient
// reduces to:
//
//	dot(UL)=−a   dot(UR)=+b   dot(LL)=−b   dot(LR)=+a
//
// (factor of ½ dropped; sign is all that matters).
func compileCellEdgeSnap(pixels [4]color.RGBA) quadCell {
	var op [4]color.RGBA
	n := 0
	for _, p := range pixels {
		if !isTransparent(p) {
			op[n] = p
			n++
		}
	}
	switch n {
	case 0:
		return quadCell{ch: ' ', transparent: true}
	case 1:
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: op[0], hasFG: true})
	}

	// BT.709 luma for each quadrant; transparent pixels contribute 0.
	lumaOf := func(c color.RGBA) float64 {
		if isTransparent(c) {
			return 0
		}
		return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	}
	L := [4]float64{lumaOf(pixels[0]), lumaOf(pixels[1]), lumaOf(pixels[2]), lumaOf(pixels[3])}

	// 2×2 finite-difference gradient (each component in range ±510).
	gx := (L[1] + L[3]) - (L[0] + L[2]) // right column sum − left column sum
	gy := (L[2] + L[3]) - (L[0] + L[1]) // bottom row sum − top row sum

	// Nearly flat cell: gradient too small to determine a reliable direction.
	if gx*gx+gy*gy < 64 {
		return compileCellDiameter(pixels)
	}

	// Dot-product signs: positive → fg (bright side), negative → bg (dark side).
	// Exact ties (sign == 0) are left for twoColorCell/buildMask to resolve by
	// nearest-colour.
	a := gx + gy // sign gives UL(−a) and LR(+a) assignments
	b := gx - gy // sign gives UR(+b) and LL(−b) assignments
	signs := [4]float64{-a, b, -b, a}

	var fgR, fgG, fgB, bgR, bgG, bgB float64
	var fgN, bgN int
	for i, s := range signs {
		p := pixels[i]
		if isTransparent(p) {
			continue
		}
		if s > 0 {
			fgR += float64(p.R); fgG += float64(p.G); fgB += float64(p.B); fgN++
		} else if s < 0 {
			bgR += float64(p.R); bgG += float64(p.G); bgB += float64(p.B); bgN++
		}
		// s == 0: exactly on the edge line; let twoColorCell decide by nearest-colour
	}

	if fgN == 0 || bgN == 0 {
		// Degenerate: all pixels on the same side (e.g. all tie or all positive).
		return compileCellDiameter(pixels)
	}

	fg := color.RGBA{uint8(fgR / float64(fgN)), uint8(fgG / float64(fgN)), uint8(fgB / float64(fgN)), 255}
	bg := color.RGBA{uint8(bgR / float64(bgN)), uint8(bgG / float64(bgN)), uint8(bgB / float64(bgN)), 255}
	return twoColorCell(pixels, fg, bg)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// kmeansStep assigns each pixel in pts to the nearer of centroids ca/cb,
// returns the averaged fg (ca group) and bg (cb group) centroids.
func kmeansStep(pts []color.RGBA, ca, cb color.RGBA) (fg, bg color.RGBA) {
	var rA, gA, bA, rB, gB, bB float64
	var nA, nB int
	for _, p := range pts {
		if colorDist2(p, ca) <= colorDist2(p, cb) {
			rA += float64(p.R); gA += float64(p.G); bA += float64(p.B); nA++
		} else {
			rB += float64(p.R); gB += float64(p.G); bB += float64(p.B); nB++
		}
	}
	if nA == 0 {
		return cb, ca // degenerate: both assigned to cb
	}
	if nB == 0 {
		return ca, ca
	}
	fg = color.RGBA{R: uint8(rA / float64(nA)), G: uint8(gA / float64(nA)), B: uint8(bA / float64(nA)), A: 255}
	bg = color.RGBA{R: uint8(rB / float64(nB)), G: uint8(gB / float64(nB)), B: uint8(bB / float64(nB)), A: 255}
	return
}

// twoColorCell builds a quadCell from a known fg/bg pair, letting buildMask
// determine per-quadrant assignment by nearest-colour.
func twoColorCell(pixels [4]color.RGBA, fg, bg color.RGBA) quadCell {
	if eqRGB(fg, bg) {
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: fg, hasFG: true})
	}
	mask := buildMask(pixels, fg, bg, true)
	switch mask {
	case 0:
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: bg, hasFG: true})
	case 0b1111:
		return maybeVerticalize(pixels, quadCell{ch: '█', fg: fg, hasFG: true})
	}
	return maybeVerticalize(pixels, quadCell{ch: quadChar[mask], fg: fg, bg: bg, hasFG: true, hasBG: true})
}

// cellError returns the sum of squared distances from each source pixel to the
// colour the rendered cell would display at that quadrant.
func cellError(pixels [4]color.RGBA, c quadCell) int {
	if c.transparent {
		return 0
	}
	if c.ch == ' ' {
		return 0
	}
	switch c.ch {
	case '▌':
		return sideError(pixels, true, c)
	case '▐':
		return sideError(pixels, false, c)
	}

	mask := charToMask(c.ch)
	if c.ch != ' ' && mask == 0 && !c.transparent {
		// Unknown glyph; treat as a poor fit.
		return 1 << 30
	}

	total := 0
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		if mask&(1<<(3-i)) != 0 {
			total += colorDist2(p, c.fg)
		} else if c.hasBG {
			total += colorDist2(p, c.bg)
		} else {
			total += colorDist2(p, c.fg)
		}
	}
	return total
}

// sideError evaluates a vertical split where fg occupies either the left or
// right column and bg occupies the opposite side.
func sideError(pixels [4]color.RGBA, fgLeft bool, c quadCell) int {
	var total int
	for i, p := range pixels {
		if isTransparent(p) {
			continue
		}
		isFg := (i == 0 || i == 2)
		if !fgLeft {
			isFg = !isFg
		}
		if isFg {
			total += colorDist2(p, c.fg)
		} else if c.hasBG {
			total += colorDist2(p, c.bg)
		} else {
			total += colorDist2(p, c.fg)
		}
	}
	return total
}

// maybeVerticalize replaces a cell with ▌/▐ when a vertical split fits the
// source pixels better than the current encoding.
func maybeVerticalize(pixels [4]color.RGBA, c quadCell) quadCell {
	if c.transparent {
		return c
	}

	best := c
	bestErr := cellError(pixels, c)

	left := avgRGB(pixels[0], pixels[2])
	right := avgRGB(pixels[1], pixels[3])
	if isTransparent(left) || isTransparent(right) || eqRGB(left, right) {
		return best
	}

	leftCell := quadCell{ch: '▌', fg: left, bg: right, hasFG: true, hasBG: true}
	if err := cellError(pixels, leftCell); err < bestErr {
		best = leftCell
		bestErr = err
	}

	rightCell := quadCell{ch: '▐', fg: right, bg: left, hasFG: true, hasBG: true}
	if err := cellError(pixels, rightCell); err < bestErr {
		best = rightCell
	}

	return best
}
