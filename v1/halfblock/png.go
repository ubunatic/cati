package halfblock

import (
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"os"
)

// LoadImage opens a still image (PNG/JPEG) or, for video files, extracts
// and returns the first frame using ffmpeg.
func LoadImage(path string) (image.Image, error) {
	if IsVideo(path) {
		return LoadVideoFrame(path)
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
