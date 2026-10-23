//go:build ignore

// quadblock_fastpath_demo shows issue 094 directly in terminal ANSI output.
// Run: go run scripts/quadblock_fastpath_demo.go
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"

	"ubunatic.com/cati/v1/quadblock"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	img := image.NewNRGBA(image.Rect(0, 0, 33, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 33; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
		}
	}
	var output bytes.Buffer
	// cols=0 preserves NRGBA; CLI scaling would convert it to RGBA.
	if err := quadblock.Render(&output, img, 0, quadblock.Options{NoLinePrefix: true, Jobs: 4}); err != nil {
		return err
	}
	fmt.Println("Same source: uniform NRGBA (200,100,50,128), 33x8 pixels.")
	fmt.Print(output.String())
	return nil
}
