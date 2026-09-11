package sparkline

import (
	"image"
	"image/color"
	"math"
	"math/bits"
)

type mask128 struct {
	lo uint64
	hi uint64
}

type bitCandidate struct {
	ch   rune
	mask mask128
}

// bitCandidatesTable stores precomputed bitCandidate slices for fixed mode and block geometry.
var (
	verticalBitCandidates4x8  = makeBitCandidates(verticalCandidates, 4, 8)
	halfSplitBitCandidates2x2 = makeBitCandidates(halfSplitCandidates, 2, 2)
	halfSplitBitCandidates4x8 = makeBitCandidates(halfSplitCandidates, 4, 8)
	sparkBitCandidates4x8     = makeBitCandidates(sparkCandidates, 4, 8)
	quadBitCandidates4x8      = makeBitCandidates(sparkQuadCandidates, 4, 8)
	sextantBitCandidates2x3   = makeBitCandidates(sextantCandidates, 2, 3)
	sextantBitCandidates4x8   = makeBitCandidates(sextantCandidates, 4, 8)
	sixHalfBitCandidates2x6   = makeBitCandidates(sixHalfCandidates, 2, 6)
	sixHalfBitCandidates4x8   = makeBitCandidates(sixHalfCandidates, 4, 8)
	bestBitCandidates4x24     = makeBitCandidates(bestCandidates, 4, 24)
	bestBitCandidates4x8      = makeBitCandidates(bestCandidates, 4, 8)
)

func makeBitCandidates(candidates []candidate, w, h int) []bitCandidate {
	out := make([]bitCandidate, len(candidates))
	for i, c := range candidates {
		var m mask128
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if c.mask(x, y, w, h) {
					idx := y*w + x
					if idx < 64 {
						m.lo |= uint64(1) << idx
					} else if idx < 128 {
						m.hi |= uint64(1) << (idx - 64)
					}
				}
			}
		}
		out[i] = bitCandidate{ch: c.ch, mask: m}
	}
	return out
}

func getPrecomputedBitCandidates(mode Mode, w, h int) []bitCandidate {
	switch mode {
	case Vertical:
		if w == 4 && h == 8 {
			return verticalBitCandidates4x8
		}
	case HalfSplit:
		if w == 2 && h == 2 {
			return halfSplitBitCandidates2x2
		}
		if w == 4 && h == 8 {
			return halfSplitBitCandidates4x8
		}
	case Spark:
		if w == 4 && h == 8 {
			return sparkBitCandidates4x8
		}
	case Quad:
		if w == 4 && h == 8 {
			return quadBitCandidates4x8
		}
	case Sextant:
		if w == 2 && h == 3 {
			return sextantBitCandidates2x3
		}
		if w == 4 && h == 8 {
			return sextantBitCandidates4x8
		}
	case SixHalf:
		if w == 2 && h == 6 {
			return sixHalfBitCandidates2x6
		}
		if w == 4 && h == 8 {
			return sixHalfBitCandidates4x8
		}
	case Best:
		if w == 4 && h == 24 {
			return bestBitCandidates4x24
		}
		if w == 4 && h == 8 {
			return bestBitCandidates4x8
		}
	}
	return nil
}

func findBestCandidateFast(img image.Image, x0, x1, y0, y1 int, bitCands []bitCandidate) cellResult {
	blockW := x1 - x0 + 1
	blockH := y1 - y0 + 1
	n := blockW * blockH
	if n <= 0 {
		return cellResult{Ch: ' ', Err: math.MaxFloat64}
	}
	if n > 128 {
		return cellResult{Ch: ' ', Err: math.MaxFloat64}
	}

	var pr, pg, pb [128]uint8
	var opaque0, opaque1 uint64
	var totalSumR, totalSumG, totalSumB uint32
	var totalSqSum int64

	if rgba, ok := img.(*image.RGBA); ok {
		pix := rgba.Pix
		idx := 0
		for y := y0; y <= y1; y++ {
			rowOff := rgba.PixOffset(x0, y)
			for x := 0; x < blockW; x++ {
				off := rowOff + (x << 2)
				r := pix[off+0]
				g := pix[off+1]
				b := pix[off+2]
				a := pix[off+3]
				pr[idx] = r
				pg[idx] = g
				pb[idx] = b
				if a != 0 {
					if idx < 64 {
						opaque0 |= uint64(1) << idx
					} else {
						opaque1 |= uint64(1) << (idx - 64)
					}
					ur := uint32(r)
					ug := uint32(g)
					ub := uint32(b)
					totalSumR += ur
					totalSumG += ug
					totalSumB += ub
					totalSqSum += int64(ur*ur + ug*ug + ub*ub)
				}
				idx++
			}
		}
	} else {
		idx := 0
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				c := rgbaAt(img, x, y)
				pr[idx] = c.R
				pg[idx] = c.G
				pb[idx] = c.B
				if c.A != 0 {
					if idx < 64 {
						opaque0 |= uint64(1) << idx
					} else {
						opaque1 |= uint64(1) << (idx - 64)
					}
					ur := uint32(c.R)
					ug := uint32(c.G)
					ub := uint32(c.B)
					totalSumR += ur
					totalSumG += ug
					totalSumB += ub
					totalSqSum += int64(ur*ur + ug*ug + ub*ub)
				}
				idx++
			}
		}
	}

	var validMask0, validMask1 uint64
	if n <= 64 {
		if n == 64 {
			validMask0 = ^uint64(0)
		} else {
			validMask0 = (uint64(1) << n) - 1
		}
	} else {
		validMask0 = ^uint64(0)
		if n == 128 {
			validMask1 = ^uint64(0)
		} else {
			validMask1 = (uint64(1) << (n - 64)) - 1
		}
	}

	trans0 := (^opaque0) & validMask0
	trans1 := (^opaque1) & validMask1

	bestErrInt := int64(math.MaxInt64)
	bestFGTrans := math.MaxInt
	bestSplitPenalty := math.MaxInt
	var bestCh rune = ' '
	var bestFG, bestBG color.RGBA

	const transparentPixelCost = 195075 // 3 * 255 * 255

	if n <= 64 {
		// Optimized kernel for blocks with up to 64 pixels (4x8, 2x2, 2x3, 2x6, etc.)
		for _, cand := range bitCands {
			candM0 := cand.mask.lo
			fgOpaque0 := candM0 & opaque0
			bgOpaque0 := (^candM0) & opaque0

			fgN := bits.OnesCount64(fgOpaque0)
			bgN := bits.OnesCount64(bgOpaque0)

			fgTrans := bits.OnesCount64(candM0 & trans0)
			bgTrans := bits.OnesCount64((^candM0) & trans0)

			if fgN == 0 && bgN > 0 && cand.ch != ' ' {
				continue
			}

			var fgSumR, fgSumG, fgSumB uint32
			for b := fgOpaque0; b != 0; {
				tz := bits.TrailingZeros64(b)
				fgSumR += uint32(pr[tz])
				fgSumG += uint32(pg[tz])
				fgSumB += uint32(pb[tz])
				b &= b - 1
			}

			var fgAvgR, fgAvgG, fgAvgB uint8
			var bgAvgR, bgAvgG, bgAvgB uint8
			if fgN > 0 {
				fgAvgR = uint8(fgSumR / uint32(fgN))
				fgAvgG = uint8(fgSumG / uint32(fgN))
				fgAvgB = uint8(fgSumB / uint32(fgN))
			}
			var bgSumR, bgSumG, bgSumB uint32
			if bgN > 0 {
				bgSumR = totalSumR - fgSumR
				bgSumG = totalSumG - fgSumG
				bgSumB = totalSumB - fgSumB
				bgAvgR = uint8(bgSumR / uint32(bgN))
				bgAvgG = uint8(bgSumG / uint32(bgN))
				bgAvgB = uint8(bgSumB / uint32(bgN))
			}

			errInt := totalSqSum
			if fgN > 0 {
				uFGR := uint32(fgAvgR)
				uFGG := uint32(fgAvgG)
				uFGB := uint32(fgAvgB)
				errInt -= int64(2 * (uFGR*fgSumR + uFGG*fgSumG + uFGB*fgSumB))
				errInt += int64(fgN) * int64(uFGR*uFGR + uFGG*uFGG + uFGB*uFGB)
			}
			if bgN > 0 {
				uBGR := uint32(bgAvgR)
				uBGG := uint32(bgAvgG)
				uBGB := uint32(bgAvgB)
				errInt -= int64(2 * (uBGR*bgSumR + uBGG*bgSumG + uBGB*bgSumB))
				errInt += int64(bgN) * int64(uBGR*uBGR + uBGG*uBGG + uBGB*uBGB)
			}

			if bgN > 0 {
				errInt += int64(bgTrans) * transparentPixelCost
			}
			if fgN > 0 || bgN > 0 {
				errInt += int64(fgTrans) * transparentPixelCost
			}

			splitPenalty := 0
			if fgN > 0 && bgN > 0 {
				splitPenalty = 1
			}

			better := (errInt < bestErrInt) ||
				(errInt == bestErrInt && fgTrans < bestFGTrans) ||
				(errInt == bestErrInt && fgTrans == bestFGTrans && splitPenalty < bestSplitPenalty)

			if better {
				bestErrInt = errInt
				bestFGTrans = fgTrans
				bestSplitPenalty = splitPenalty
				bestCh = cand.ch
				if fgN > 0 {
					bestFG = color.RGBA{R: fgAvgR, G: fgAvgG, B: fgAvgB, A: 255}
				} else {
					bestFG = color.RGBA{}
				}
				if bgN > 0 {
					bestBG = color.RGBA{R: bgAvgR, G: bgAvgG, B: bgAvgB, A: 255}
				} else {
					bestBG = color.RGBA{}
				}
			}
		}
	} else {
		// 128-bit kernel for blocks up to 128 pixels (e.g. 4x24)
		for _, cand := range bitCands {
			candM0 := cand.mask.lo
			candM1 := cand.mask.hi
			fgOpaque0 := candM0 & opaque0
			fgOpaque1 := candM1 & opaque1
			bgOpaque0 := (^candM0) & opaque0
			bgOpaque1 := (^candM1) & opaque1

			fgN := bits.OnesCount64(fgOpaque0) + bits.OnesCount64(fgOpaque1)
			bgN := bits.OnesCount64(bgOpaque0) + bits.OnesCount64(bgOpaque1)

			fgTrans := bits.OnesCount64(candM0&trans0) + bits.OnesCount64(candM1&trans1)
			bgTrans := bits.OnesCount64((^candM0)&trans0) + bits.OnesCount64((^candM1)&trans1)

			if fgN == 0 && bgN > 0 && cand.ch != ' ' {
				continue
			}

			var fgSumR, fgSumG, fgSumB uint32
			for b := fgOpaque0; b != 0; {
				tz := bits.TrailingZeros64(b)
				fgSumR += uint32(pr[tz])
				fgSumG += uint32(pg[tz])
				fgSumB += uint32(pb[tz])
				b &= b - 1
			}
			for b := fgOpaque1; b != 0; {
				tz := bits.TrailingZeros64(b) + 64
				fgSumR += uint32(pr[tz])
				fgSumG += uint32(pg[tz])
				fgSumB += uint32(pb[tz])
				b &= b - 1
			}

			var fgAvgR, fgAvgG, fgAvgB uint8
			var bgAvgR, bgAvgG, bgAvgB uint8
			if fgN > 0 {
				fgAvgR = uint8(fgSumR / uint32(fgN))
				fgAvgG = uint8(fgSumG / uint32(fgN))
				fgAvgB = uint8(fgSumB / uint32(fgN))
			}
			var bgSumR, bgSumG, bgSumB uint32
			if bgN > 0 {
				bgSumR = totalSumR - fgSumR
				bgSumG = totalSumG - fgSumG
				bgSumB = totalSumB - fgSumB
				bgAvgR = uint8(bgSumR / uint32(bgN))
				bgAvgG = uint8(bgSumG / uint32(bgN))
				bgAvgB = uint8(bgSumB / uint32(bgN))
			}

			errInt := totalSqSum
			if fgN > 0 {
				uFGR := uint32(fgAvgR)
				uFGG := uint32(fgAvgG)
				uFGB := uint32(fgAvgB)
				errInt -= int64(2 * (uFGR*fgSumR + uFGG*fgSumG + uFGB*fgSumB))
				errInt += int64(fgN) * int64(uFGR*uFGR + uFGG*uFGG + uFGB*uFGB)
			}
			if bgN > 0 {
				uBGR := uint32(bgAvgR)
				uBGG := uint32(bgAvgG)
				uBGB := uint32(bgAvgB)
				errInt -= int64(2 * (uBGR*bgSumR + uBGG*bgSumG + uBGB*bgSumB))
				errInt += int64(bgN) * int64(uBGR*uBGR + uBGG*uBGG + uBGB*uBGB)
			}

			if bgN > 0 {
				errInt += int64(bgTrans) * transparentPixelCost
			}
			if fgN > 0 || bgN > 0 {
				errInt += int64(fgTrans) * transparentPixelCost
			}

			splitPenalty := 0
			if fgN > 0 && bgN > 0 {
				splitPenalty = 1
			}

			better := (errInt < bestErrInt) ||
				(errInt == bestErrInt && fgTrans < bestFGTrans) ||
				(errInt == bestErrInt && fgTrans == bestFGTrans && splitPenalty < bestSplitPenalty)

			if better {
				bestErrInt = errInt
				bestFGTrans = fgTrans
				bestSplitPenalty = splitPenalty
				bestCh = cand.ch
				if fgN > 0 {
					bestFG = color.RGBA{R: fgAvgR, G: fgAvgG, B: fgAvgB, A: 255}
				} else {
					bestFG = color.RGBA{}
				}
				if bgN > 0 {
					bestBG = color.RGBA{R: bgAvgR, G: bgAvgG, B: bgAvgB, A: 255}
				} else {
					bestBG = color.RGBA{}
				}
			}
		}
	}

	return cellResult{
		Ch:  bestCh,
		FG:  bestFG,
		BG:  bestBG,
		Err: float64(bestErrInt),
	}
}
