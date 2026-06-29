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
)

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
