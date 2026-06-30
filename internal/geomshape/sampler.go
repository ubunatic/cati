package geomshape

import (
	"fmt"
	"image"
	"image/color"
)

// Sampler selects the block partitioning strategy used before glyph scoring.
//
// SamplerLegacy keeps the current 2x2 midpoint split.
// SamplerV2 is an isolated copy that uses a diagonal-biased corner sampler so
// the richer-geometry work can evolve without changing the legacy path.
type Sampler int

const (
	SamplerLegacy Sampler = iota
	SamplerV2
)

func (s Sampler) String() string {
	switch s {
	case SamplerV2:
		return "v2"
	default:
		return "legacy"
	}
}

func sampleBlockWithSampler(img image.Image, x0, x1, y0, y1 int, sampler Sampler) [4]color.RGBA {
	switch sampler {
	case SamplerV2:
		return sampleBlockV2(img, x0, x1, y0, y1)
	default:
		return sampleBlockLegacy(img, x0, x1, y0, y1)
	}
}

// sampleBlockLegacy keeps the current midpoint split intact.
func sampleBlockLegacy(img image.Image, x0, x1, y0, y1 int) [4]color.RGBA {
	var pixels [4]color.RGBA
	if x1 <= x0 || y1 <= y0 {
		return pixels
	}
	midX := x0 + (x1-x0)/2
	midY := y0 + (y1-y0)/2
	pixels[0] = avgRegion(img, x0, midX, y0, midY)
	pixels[1] = avgRegion(img, midX, x1, y0, midY)
	pixels[2] = avgRegion(img, x0, midX, midY, y1)
	pixels[3] = avgRegion(img, midX, x1, midY, y1)
	return pixels
}

// sampleBlockV2 is a copied sampler that assigns each source pixel to the
// nearest corner of the current block. It is intentionally isolated so the
// new geometry work can tune the partitioning without affecting legacy output.
func sampleBlockV2(img image.Image, x0, x1, y0, y1 int) [4]color.RGBA {
	var pixels [4]color.RGBA
	if x1 <= x0 || y1 <= y0 {
		return pixels
	}
	type accum struct {
		r int
		g int
		b int
		n int
	}
	var sums [4]accum
	w := float64(x1 - x0)
	h := float64(y1 - y0)
	if w <= 0 || h <= 0 {
		return pixels
	}
	for y := y0; y < y1; y++ {
		fy := (float64(y-y0) + 0.5) / h
		for x := x0; x < x1; x++ {
			fx := (float64(x-x0) + 0.5) / w
			idx := nearestCornerIndex(fx, fy)
			p := toRGBA(img.At(x, y))
			if isTransparent(p) {
				continue
			}
			sums[idx].r += int(p.R)
			sums[idx].g += int(p.G)
			sums[idx].b += int(p.B)
			sums[idx].n++
		}
	}
	for i := range sums {
		if sums[i].n == 0 {
			continue
		}
		pixels[i] = color.RGBA{
			R: uint8(sums[i].r / sums[i].n),
			G: uint8(sums[i].g / sums[i].n),
			B: uint8(sums[i].b / sums[i].n),
			A: 255,
		}
	}
	return pixels
}

func nearestCornerIndex(fx, fy float64) int {
	// Corner order is UL, UR, LL, LR to match the existing bit layout.
	dist := [4]float64{
		fx*fx + fy*fy,
		(fx-1)*(fx-1) + fy*fy,
		fx*fx + (fy-1)*(fy-1),
		(fx-1)*(fx-1) + (fy-1)*(fy-1),
	}
	best := 0
	bestDist := dist[0]
	for i := 1; i < len(dist); i++ {
		if dist[i] < bestDist {
			best = i
			bestDist = dist[i]
		}
	}
	return best
}

func (s Sampler) goString() string { return fmt.Sprintf("geomshape.Sampler(%s)", s.String()) }
