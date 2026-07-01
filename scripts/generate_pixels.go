package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Paths
	root := "."
	pngPath := filepath.Join(root, "website", "cati_0001.png")
	htmlPath := filepath.Join(root, "website", "index.html")

	// Allow overrides via CLI args
	if len(os.Args) > 1 {
		pngPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		htmlPath = os.Args[2]
	}

	fmt.Printf("Reading source PNG: %s...\n", pngPath)
	file, err := os.Open(pngPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening PNG: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding PNG: %v\n", err)
		os.Exit(1)
	}

	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	var sb strings.Builder
	sb.WriteString("  const pixelColors = [\n")

	for y := 0; y < height; y++ {
		sb.WriteString("    ")
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// Convert 16-bit to 8-bit color values
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			a8 := uint8(a >> 8)

			var val string
			if a8 < 16 {
				val = ""
			} else {
				val = fmt.Sprintf("#%02x%02x%02x", r8, g8, b8)
			}

			sb.WriteString(fmt.Sprintf("%q", val))
			if !(y == height-1 && x == width-1) {
				sb.WriteString(",")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  ];")

	newColorsJS := sb.String()

	fmt.Printf("Reading HTML file: %s...\n", htmlPath)
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading HTML file: %v\n", err)
		os.Exit(1)
	}

	startMarker := []byte("// PIXELS_START")
	endMarker := []byte("// PIXELS_END")

	startIndex := bytes.Index(htmlContent, startMarker)
	endIndex := bytes.Index(htmlContent, endMarker)

	if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
		fmt.Fprintf(os.Stderr, "Error: Could not find valid markers '// PIXELS_START' and '// PIXELS_END' in %s\n", htmlPath)
		os.Exit(1)
	}

	// Build the new HTML content
	var newHTML bytes.Buffer
	newHTML.Write(htmlContent[:startIndex+len(startMarker)])
	newHTML.WriteString("\n")
	newHTML.WriteString(newColorsJS)
	newHTML.WriteString("\n  ")
	newHTML.Write(htmlContent[endIndex:])

	fmt.Printf("Updating HTML file: %s...\n", htmlPath)
	err = os.WriteFile(htmlPath, newHTML.Bytes(), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing HTML file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Success! Pixel colors inlined successfully.")
}
