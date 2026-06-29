package testhelper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// GenerateFixtures creates special regression fixture images under testdataDir.
// Each image is placed in its own subdirectory as source.png.
//
// Fixtures:
//   - solid_red_4x4/source.png: 4×4 opaque red image (regression: partial-char transparency)
func GenerateFixtures(testdataDir string) error {
	fixtures := []struct {
		dir string
		img *image.RGBA
	}{
		{
			"solid_red_4x4",
			func() *image.RGBA {
				img := image.NewRGBA(image.Rect(0, 0, 4, 4))
				for y := range 4 {
					for x := range 4 {
						img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
					}
				}
				return img
			}(),
		},
	}
	for _, f := range fixtures {
		dir := filepath.Join(testdataDir, f.dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := SavePNG(filepath.Join(dir, "source.png"), f.img, map[string]string{
			"Description": f.dir,
		}); err != nil {
			return err
		}
	}
	return nil
}

// GenerateGeometrics creates 20×20 geometric test images under testdataDir.
// Each image uses pure saturated colours so block-char algorithms have
// unambiguous ground truth at every cell boundary.
//
// Images generated:
//   - demo_diag_20x20:    red/blue split along the 45° diagonal (top-left red)
//   - demo_circle_20x20:  yellow filled disc (r=8) on blue background
//   - demo_checker_20x20: red/blue checkerboard with 4×4-pixel cells
//   - demo_cross_20x20:   yellow 4-pixel-wide cross on blue background
func GenerateGeometrics(testdataDir string) error {
	const sz = 20
	red  := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	yell := color.RGBA{R: 255, G: 255, A: 255}

	type geom struct {
		dir string
		img *image.RGBA
	}

	geoms := []geom{
		{
			"demo_diag_20x20",
			func() *image.RGBA {
				img := image.NewRGBA(image.Rect(0, 0, sz, sz))
				for y := range sz {
					for x := range sz {
						if x+y < sz {
							img.SetRGBA(x, y, red)
						} else {
							img.SetRGBA(x, y, blue)
						}
					}
				}
				return img
			}(),
		},
		{
			"demo_circle_20x20",
			func() *image.RGBA {
				img := image.NewRGBA(image.Rect(0, 0, sz, sz))
				cx, cy, r := 9.5, 9.5, 8.0
				for y := range sz {
					for x := range sz {
						dx := float64(x) - cx
						dy := float64(y) - cy
						if math.Sqrt(dx*dx+dy*dy) <= r {
							img.SetRGBA(x, y, yell)
						} else {
							img.SetRGBA(x, y, blue)
						}
					}
				}
				return img
			}(),
		},
		{
			"demo_checker_20x20",
			func() *image.RGBA {
				img := image.NewRGBA(image.Rect(0, 0, sz, sz))
				for y := range sz {
					for x := range sz {
						if (x/4+y/4)%2 == 0 {
							img.SetRGBA(x, y, red)
						} else {
							img.SetRGBA(x, y, blue)
						}
					}
				}
				return img
			}(),
		},
		{
			"demo_cross_20x20",
			func() *image.RGBA {
				img := image.NewRGBA(image.Rect(0, 0, sz, sz))
				for y := range sz {
					for x := range sz {
						onBar := (x >= 8 && x <= 11) || (y >= 8 && y <= 11)
						if onBar {
							img.SetRGBA(x, y, yell)
						} else {
							img.SetRGBA(x, y, blue)
						}
					}
				}
				return img
			}(),
		},
	}

	for _, g := range geoms {
		dir := filepath.Join(testdataDir, g.dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := SavePNG(filepath.Join(dir, "source.png"), g.img, map[string]string{
			"Description": g.dir,
		}); err != nil {
			return err
		}
	}
	return nil
}

// GenerateGradients generates the horizontal and vertical gradients at 20x20, 4x4, 2x2, 1x1.
// Base colors are Blue (0,0,255) and Yellow (255,255,0).
func GenerateGradients(testdataDir string) error {
	sizes := []int{20, 4, 2, 1}
	for _, sz := range sizes {
		horizDir := filepath.Join(testdataDir, fmt.Sprintf("demo_horiz_%dx%d", sz, sz))
		if err := os.MkdirAll(horizDir, 0755); err != nil {
			return err
		}
		vertiDir := filepath.Join(testdataDir, fmt.Sprintf("demo_verti_%dx%d", sz, sz))
		if err := os.MkdirAll(vertiDir, 0755); err != nil {
			return err
		}

		horiz := image.NewRGBA(image.Rect(0, 0, sz, sz))
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				var col color.RGBA
				if sz == 1 {
					col = color.RGBA{R: 0, G: 0, B: 255, A: 255}
				} else {
					val := float64(x) / float64(sz-1)
					if y < sz/2 {
						col = color.RGBA{R: uint8(255 * val), G: uint8(255 * val), B: uint8(255 * (1 - val)), A: 255}
					} else {
						col = color.RGBA{R: uint8(255 * (1 - val)), G: uint8(255 * (1 - val)), B: uint8(255 * val), A: 255}
					}
				}
				horiz.Set(x, y, col)
			}
		}

		verti := image.NewRGBA(image.Rect(0, 0, sz, sz))
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				var col color.RGBA
				if sz == 1 {
					col = color.RGBA{R: 255, G: 255, B: 0, A: 255}
				} else {
					val := float64(y) / float64(sz-1)
					if x < sz/2 {
						col = color.RGBA{R: uint8(255 * (1 - val)), G: uint8(255 * (1 - val)), B: uint8(255 * val), A: 255}
					} else {
						col = color.RGBA{R: uint8(255 * val), G: uint8(255 * val), B: uint8(255 * (1 - val)), A: 255}
					}
				}
				verti.Set(x, y, col)
			}
		}

		if err := SavePNG(filepath.Join(horizDir, "source.png"), horiz, map[string]string{
			"Description": "Horizontal gradient source",
		}); err != nil {
			return err
		}
		if err := SavePNG(filepath.Join(vertiDir, "source.png"), verti, map[string]string{
			"Description": "Vertical gradient source",
		}); err != nil {
			return err
		}
	}
	return nil
}

// SavePNG encodes img as PNG and injects tEXt metadata chunks before writing to path.
func SavePNG(path string, img image.Image, metadata map[string]string) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	finalBytes, err := injectMetadata(buf.Bytes(), metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(path, finalBytes, 0644)
}

func makeTextChunk(keyword, text string) []byte {
	data := append([]byte(keyword), 0)
	data = append(data, []byte(text)...)
	length := uint32(len(data))
	chunkType := []byte("tEXt")
	h := crc32.NewIEEE()
	h.Write(chunkType)
	h.Write(data)
	crc := h.Sum32()
	buf := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(buf[0:4], length)
	copy(buf[4:8], chunkType)
	copy(buf[8:8+len(data)], data)
	binary.BigEndian.PutUint32(buf[8+len(data):12+len(data)], crc)
	return buf
}

func injectMetadata(pngBytes []byte, metadata map[string]string) ([]byte, error) {
	if len(pngBytes) < 8 || string(pngBytes[0:8]) != "\x89PNG\r\n\x1a\n" {
		return nil, fmt.Errorf("invalid png signature")
	}
	var chunks []byte
	chunks = append(chunks, pngBytes[0:8]...)
	idx := 8
	for idx < len(pngBytes) {
		if idx+8 > len(pngBytes) {
			return nil, fmt.Errorf("unexpected EOF")
		}
		length := binary.BigEndian.Uint32(pngBytes[idx : idx+4])
		chunkType := string(pngBytes[idx+4 : idx+8])
		totalLen := int(12 + length)
		if idx+totalLen > len(pngBytes) {
			return nil, fmt.Errorf("unexpected EOF inside chunk")
		}
		if chunkType == "IEND" {
			for k, v := range metadata {
				chunks = append(chunks, makeTextChunk(k, v)...)
			}
		}
		chunks = append(chunks, pngBytes[idx:idx+totalLen]...)
		idx += totalLen
	}
	return chunks, nil
}
