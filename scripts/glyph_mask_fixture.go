//go:build ignore

// Generate the independent asymmetric glyph-card fixture from explicit rows.
package main

import (
	"image"
	"image/color"
	"log"
	"os"

	"ubunatic.com/cati/v1/sparkline/testhelper"
)

func main() {
	const dir = "testdata/glyph_masks"
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(err)
	}
	rows := []string{"10", "01", "10"}
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for y, row := range rows {
		for x, bit := range row {
			c := color.RGBA{A: 255}
			if bit == '1' {
				c.R = 255
				c.G = 255
				c.B = 255
			}
			img.SetRGBA(x, y, c)
		}
	}
	if err := testhelper.SavePNG(dir+"/asymmetric_2x3.png", img, map[string]string{
		"Algorithm": "glyph-mask-card", "Geometry": "2x3", "Mask": "100110",
		"Description": "Independent row-major asymmetric mask; ASCII and halfblock previews; odd final row padded with background",
	}); err != nil {
		log.Fatal(err)
	}
}
