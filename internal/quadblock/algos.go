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
		return quadCell{ch: '█', fg: a, hasFG: true}
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
		return quadCell{ch: '█', fg: a, hasFG: true}
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
		return quadCell{ch: '█', fg: fg, hasFG: true}
	}
	mask := buildMask(pixels, fg, bg, true)
	switch mask {
	case 0:
		return quadCell{ch: '█', fg: bg, hasFG: true}
	case 0b1111:
		return quadCell{ch: '█', fg: fg, hasFG: true}
	}
	return quadCell{ch: quadChar[mask], fg: fg, bg: bg, hasFG: true, hasBG: true}
}
