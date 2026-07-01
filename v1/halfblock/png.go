package halfblock

import (
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"os"
)

// LoadImage opens a still image (PNG/JPEG), rasterizes an SVG using
// rsvg-convert, or, for video files, extracts and returns the first frame
// using ffmpeg.
func LoadImage(path string) (image.Image, error) {
	return LoadImageWithTarget(path, 0, 0)
}

// LoadImageWithTarget is like LoadImage, but SVG inputs are rasterized to fit
// within maxWidth×maxHeight pixels. Raster image and video inputs keep their
// native decoded dimensions; callers should scale them after loading.
func LoadImageWithTarget(path string, maxWidth, maxHeight int) (image.Image, error) {
	if IsVideo(path) {
		return LoadVideoFrame(path)
	}
	if IsSVG(path) {
		return RasterizeSVGWithTarget(path, maxWidth, maxHeight)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

// LoadPNG is an alias for LoadImage kept for backwards compatibility and tests.
var LoadPNG = LoadImage

// loadPNG is the unexported alias used in tests within this package.
var loadPNG = LoadImage
