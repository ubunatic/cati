// Package imgutil provides pure image-geometry utilities: aspect-ratio-preserving
// fitting and sub-image extraction.
//
// All functions depend only on image and math — no I/O, no terminal interaction,
// no project-internal dependencies.
package imgutil

import "image"

// FitPixelDims returns the largest w×h that preserves the srcW×srcH aspect
// ratio while fitting inside maxW×maxH. It never upscales.
// Returns at least 1×1 when both max dims are positive.
func FitPixelDims(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW == 0 || srcH == 0 {
		return max(1, maxW), max(1, maxH)
	}
	w, h := srcW, srcH
	if w > maxW {
		h = h * maxW / w
		w = maxW
	}
	if h > maxH {
		w = w * maxH / h
		h = maxH
	}
	return max(1, w), max(1, h)
}

// CropImage returns the w×h sub-image of img starting at pixel (x, y).
// Uses SubImage for zero-copy when img supports it (e.g. *image.RGBA);
// otherwise copies pixel-by-pixel into a new *image.RGBA.
func CropImage(img image.Image, x, y, w, h int) image.Image {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	b := img.Bounds()
	r := image.Rect(b.Min.X+x, b.Min.Y+y, b.Min.X+x+w, b.Min.Y+y+h)
	if si, ok := img.(subImager); ok {
		return si.SubImage(r)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			dst.Set(dx, dy, img.At(r.Min.X+dx, r.Min.Y+dy))
		}
	}
	return dst
}
