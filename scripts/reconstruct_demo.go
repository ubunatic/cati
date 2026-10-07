//go:build ignore

// reconstruct_demo visually verifies Issue #092 (custom glyph mask reconstruction).
// Run: go run scripts/reconstruct_demo.go
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"ubunatic.com/cati/internal/metrics"
	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/sparkline"
)

func main() {
	fmt.Println("================================================================================")
	fmt.Println(" Visual Verification: Issue #092 (Custom Glyph Reconstruction Fidelity)")
	fmt.Println("================================================================================")

	// Demo 1: Octant Mode (2x4 lattice) - Asymmetric pixel patterns
	fmt.Println("\n[Demo 1: Octant Mode (oct, 2x4)] - Single subpixel at (0,1), red on blue")
	octRes, err := spec.ResolveGlyphSetExpression("octant")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve octant shapes: %v\n", err)
		return
	}
	octImg := image.NewRGBA(image.Rect(0, 0, 2, 4))
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 50, B: 200, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			if x == 0 && y == 1 {
				octImg.SetRGBA(x, y, red)
			} else {
				octImg.SetRGBA(x, y, blue)
			}
		}
	}
	showReconstruction("octant 1-cell", octImg, octRes.Shapes, octRes.Geometry.W, octRes.Geometry.H)

	// Demo 2: Vector Mode (vec6, 4x6 lattice) - Diagonal split pattern
	fmt.Println("\n[Demo 2: Vector Mode (vec6, 4x6)] - Diagonal upper-left triangle, green on purple")
	vecRes, err := spec.ResolveGlyphSetExpression("vec6")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve vector shapes: %v\n", err)
		return
	}
	vecImg := image.NewRGBA(image.Rect(0, 0, vecRes.Geometry.W, vecRes.Geometry.H))
	green := color.RGBA{R: 0, G: 220, B: 50, A: 255}
	purple := color.RGBA{R: 120, G: 20, B: 180, A: 255}
	for y := 0; y < vecRes.Geometry.H; y++ {
		for x := 0; x < vecRes.Geometry.W; x++ {
			if x+y < 3 {
				vecImg.SetRGBA(x, y, green)
			} else {
				vecImg.SetRGBA(x, y, purple)
			}
		}
	}
	showReconstruction("vector 1-cell", vecImg, vecRes.Shapes, vecRes.Geometry.W, vecRes.Geometry.H)

	// Demo 3: Multi-cell Octant Pattern (12x8 pixels = 6x2 cells)
	fmt.Println("\n[Demo 3: Octant Multi-cell Pattern (12x8 pixels -> 6x2 cells)]")
	multiImg := image.NewRGBA(image.Rect(0, 0, 12, 8))
	yellow := color.RGBA{R: 255, G: 220, B: 0, A: 255}
	dark := color.RGBA{R: 20, G: 20, B: 40, A: 255}
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			if (x+y)%3 == 0 || (x*y)%5 == 0 {
				multiImg.SetRGBA(x, y, yellow)
			} else {
				multiImg.SetRGBA(x, y, dark)
			}
		}
	}
	showReconstruction("octant 6x2 grid", multiImg, octRes.Shapes, octRes.Geometry.W, octRes.Geometry.H)
}

func showReconstruction(title string, src image.Image, shapes []spec.GlyphShape, cellW, cellH int) {
	b := src.Bounds()
	outCols := (b.Dx() + cellW - 1) / cellW
	outRows := (b.Dy() + cellH - 1) / cellH

	sparkShapes := make([]sparkline.Shape, len(shapes))
	for i, s := range shapes {
		sparkShapes[i] = sparkline.Shape{
			Ch:     s.Glyph,
			Width:  cellW,
			Height: cellH,
			Mask:   s.Mask,
		}
	}

	opts := sparkline.Options{
		CellW:        cellW,
		CellH:        cellH,
		AspectX:      1,
		Shapes:       sparkShapes,
		NoLinePrefix: true,
	}

	// 1. ANSI terminal render (the glyphs)
	var ansiBuf bytes.Buffer
	if err := sparkline.Render(&ansiBuf, src, outCols, opts); err != nil {
		fmt.Printf("Render error: %v\n", err)
		return
	}

	// 2. Reconstructed image
	reconstructed, err := sparkline.RenderToImageWithOptions(src, outCols, outRows, opts)
	if err != nil {
		fmt.Printf("Reconstruct error: %v\n", err)
		return
	}

	// 3. Render Source and Reconstructed via Halfblock at 1:1 pixel resolution
	var srcHalfBuf, reconHalfBuf bytes.Buffer
	halfblock.Render(&srcHalfBuf, src, 0, halfblock.Options{NoLinePrefix: true})
	halfblock.Render(&reconHalfBuf, reconstructed, 0, halfblock.Options{NoLinePrefix: true})

	ssimVal := metrics.SSIMLuminance(src, reconstructed)

	fmt.Printf("--- %s (SSIM: %.4f) ---\n", title, ssimVal)
	fmt.Printf("GLYPH OUTPUT (%d cols x %d rows):\n%s\n", outCols, outRows, ansiBuf.String())

	srcLines := strings.Split(strings.TrimRight(srcHalfBuf.String(), "\n"), "\n")
	reconLines := strings.Split(strings.TrimRight(reconHalfBuf.String(), "\n"), "\n")

	fmt.Println("SOURCE PIXELS (1:1)       RECONSTRUCTED PIXELS (1:1)")
	maxL := len(srcLines)
	if len(reconLines) > maxL {
		maxL = len(reconLines)
	}
	for i := 0; i < maxL; i++ {
		sLine := ""
		if i < len(srcLines) {
			sLine = srcLines[i]
		}
		rLine := ""
		if i < len(reconLines) {
			rLine = reconLines[i]
		}
		fmt.Printf("%-24s  %-24s\n", sLine, rLine)
	}
	fmt.Println()
}
