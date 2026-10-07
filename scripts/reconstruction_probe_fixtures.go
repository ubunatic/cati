//go:build ignore

// Generate independent expected pixels for the three reconstruction probes.
// All three are exact. The vector probe is one vec6 cell (4x6): its green
// upper-left triangle (x+y<3) is the complement of the U+1FB44 mask, so the
// glyph reproduces the source with purple foreground on green background.
package main

import (
	"image"
	"image/color"
	"log"
	"os"

	"ubunatic.com/cati/v1/sparkline/testhelper"
)

func main() {
	const dir = "testdata/reconstruction_probes"
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(err)
	}
	cases := []struct {
		name, mode, geometry string
		rows                 []string
		fg, bg               color.RGBA
	}{
		{"probe", "oct", "2x4", []string{"00", "10", "00", "00"}, color.RGBA{R: 255, A: 255}, color.RGBA{G: 50, B: 200, A: 255}},
		{"diag", "vec6", "4x6", []string{"1110", "1100", "1000", "0000", "0000", "0000"}, color.RGBA{G: 220, B: 50, A: 255}, color.RGBA{R: 120, G: 20, B: 180, A: 255}},
		{"grid", "oct", "2x4", []string{
			"111111111111", "101001001011", "110011010010", "100101100110",
			"101001001011", "111111111111", "100101100110", "101001001011",
		}, color.RGBA{R: 255, G: 220, A: 255}, color.RGBA{R: 20, G: 20, B: 40, A: 255}},
	}
	for _, c := range cases {
		img := image.NewRGBA(image.Rect(0, 0, len(c.rows[0]), len(c.rows)))
		for y, row := range c.rows {
			for x, bit := range row {
				v := c.bg
				if bit == '1' {
					v = c.fg
				}
				img.SetRGBA(x, y, v)
			}
		}
		if err := testhelper.SavePNG(dir+"/"+c.name+".png", img, map[string]string{
			"Algorithm": "glyph-probe", "Mode": c.mode, "Pattern": c.name, "Geometry": c.geometry,
			"Description": "Independent expected reconstruction at 1:1 pixels; all probes exact; no resampling",
		}); err != nil {
			log.Fatal(err)
		}
	}
}
