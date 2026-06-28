package testhelper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sparkline"
)

// GenerateGradients generates the horizontal and vertical gradients at 20x20, 4x4, 2x2, 1x1.
// Base colors are Blue (0,0,255) and Yellow (255,255,0).
func GenerateGradients(testdataDir string) error {
	sizes := []int{20, 4, 2, 1}
	for _, sz := range sizes {
		// Create subfolders
		horizDir := filepath.Join(testdataDir, fmt.Sprintf("demo_horiz_%dx%d", sz, sz))
		if err := os.MkdirAll(horizDir, 0755); err != nil {
			return err
		}
		vertiDir := filepath.Join(testdataDir, fmt.Sprintf("demo_verti_%dx%d", sz, sz))
		if err := os.MkdirAll(vertiDir, 0755); err != nil {
			return err
		}

		// Horiz
		horiz := image.NewRGBA(image.Rect(0, 0, sz, sz))
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				var col color.RGBA
				if sz == 1 {
					col = color.RGBA{R: 0, G: 0, B: 255, A: 255} // Blue
				} else {
					val := float64(x) / float64(sz-1)
					if y < sz/2 {
						// top half: Blue to Yellow
						col = color.RGBA{
							R: uint8(255 * val),
							G: uint8(255 * val),
							B: uint8(255 * (1 - val)),
							A: 255,
						}
					} else {
						// bottom half: Yellow to Blue
						col = color.RGBA{
							R: uint8(255 * (1 - val)),
							G: uint8(255 * (1 - val)),
							B: uint8(255 * val),
							A: 255,
						}
					}
				}
				horiz.Set(x, y, col)
			}
		}

		// Verti
		verti := image.NewRGBA(image.Rect(0, 0, sz, sz))
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				var col color.RGBA
				if sz == 1 {
					col = color.RGBA{R: 255, G: 255, B: 0, A: 255} // Yellow
				} else {
					val := float64(y) / float64(sz-1)
					if x < sz/2 {
						// left half: Yellow to Blue
						col = color.RGBA{
							R: uint8(255 * (1 - val)),
							G: uint8(255 * (1 - val)),
							B: uint8(255 * val),
							A: 255,
						}
					} else {
						// right half: Blue to Yellow
						col = color.RGBA{
							R: uint8(255 * val),
							G: uint8(255 * val),
							B: uint8(255 * (1 - val)),
							A: 255,
						}
					}
				}
				verti.Set(x, y, col)
			}
		}

		// Save source images
		if err := savePNG(filepath.Join(horizDir, "source.png"), horiz, map[string]string{
			"Description": "Horizontal gradient source",
		}); err != nil {
			return err
		}
		if err := savePNG(filepath.Join(vertiDir, "source.png"), verti, map[string]string{
			"Description": "Vertical gradient source",
		}); err != nil {
			return err
		}
	}
	return nil
}

// RenderToImage renders the image using the sparkline algorithm at the specified cell resolution,
// and returns a new image representing the rendered output.
func RenderToImage(img image.Image, outCols, outRows int, mode sparkline.Mode) image.Image {
	b := img.Bounds()
	pixW := b.Dx()
	pixH := b.Dy()

	cellW := max(1, pixW/outCols)
	cellH := max(1, pixH/outRows)

	dst := image.NewRGBA(b)

	for tr := 0; tr < outRows; tr++ {
		for tc := 0; tc < outCols; tc++ {
			x0 := b.Min.X + min(tc*cellW, pixW)
			x1 := b.Min.X + min(tc*cellW+cellW, pixW) - 1
			y0 := b.Min.Y + min(tr*cellH, pixH)
			y1 := b.Min.Y + min(tr*cellH+cellH, pixH) - 1
			if x1 < x0 || y1 < y0 {
				continue
			}

			cell := sparkline.FindBestCell(img, b, x0, x1, y0, y1, mode)

			cx0, cx1 := x0, x1
			cy0, cy1 := y0, y1
			cw := cx1 - cx0 + 1
			ch := cy1 - cy0 + 1

			for cy := cy0; cy <= cy1; cy++ {
				dy := cy - cy0
				for cx := cx0; cx <= cx1; cx++ {
					dx := cx - cx0

					isFg := cellMask(cell.Ch, dx, dy, cw, ch)

					var c color.Color
					if isFg {
						c = cell.FG
					} else {
						c = cell.BG
					}
					dst.Set(cx, cy, c)
				}
			}
		}
	}
	return dst
}

func cellMask(ch rune, x, y, w, h int) bool {
	switch ch {
	case ' ':
		return false
	case '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█':
		level := map[rune]int{'▁': 1, '▂': 2, '▃': 3, '▄': 4, '▅': 5, '▆': 6, '▇': 7, '█': 8}[ch]
		return (h-y)*8 <= level*h
	case '▘':
		return x*2 < w && y*2 < h
	case '▝':
		return x*2 >= w && y*2 < h
	case '▖':
		return x*2 < w && y*2 >= h
	case '▗':
		return x*2 >= w && y*2 >= h
	case '▀':
		return y*2 < h
	case '▌':
		return x*2 < w
	case '▐':
		return x*2 >= w
	case '▚':
		return (x*2 < w && y*2 < h) || (x*2 >= w && y*2 >= h)
	case '▞':
		return (x*2 >= w && y*2 < h) || (x*2 < w && y*2 >= h)
	case '▛':
		return !(x*2 >= w && y*2 >= h)
	case '▜':
		return !(x*2 < w && y*2 >= h)
	case '▙':
		return !(x*2 >= w && y*2 < h)
	case '▟':
		return !(x*2 < w && y*2 < h)
	default:
		return true
	}
}

// RenderHalfblock renders the image using the halfblock algorithm and scales it back to original bounds.
func RenderHalfblock(img image.Image, outCols, outRows int) image.Image {
	b := img.Bounds()
	scaled := halfblock.ScaleToFit(img, outCols, outRows)
	rendered := halfblock.RenderToImage(scaled)
	return scaleNN(rendered, b.Dx(), b.Dy())
}

// RenderQuadblock renders the image using the quadblock algorithm and scales it back to original bounds.
func RenderQuadblock(img image.Image, outCols, outRows int) image.Image {
	b := img.Bounds()
	scaled := quadblock.ScaleToFit(img, outCols, outRows)
	opts := quadblock.Options{
		KMeans: 3,
	}
	rendered := quadblock.RenderToImage(scaled, opts)
	return scaleNN(rendered, b.Dx(), b.Dy())
}

// scaleNN performs nearest-neighbor upscaling.
func scaleNN(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 || w < 1 || h < 1 {
		return img
	}
	if srcW == w && srcH == h {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := b.Min.Y + y*srcH/h
		for x := 0; x < w; x++ {
			srcX := b.Min.X + x*srcW/w
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func savePNG(path string, img image.Image, metadata map[string]string) error {
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
	if len(pngBytes) < 8 {
		return nil, fmt.Errorf("invalid png signature")
	}
	if string(pngBytes[0:8]) != "\x89PNG\r\n\x1a\n" {
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
