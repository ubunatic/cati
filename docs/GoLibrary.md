---
title: "Go Library API"
weight: 12
---

# Using Cati as a Go Library

Cati can be imported as a Go library to render images or videos directly in your terminal-based applications and TUI frameworks (like Bubbletea or `tcell`).

## Installation

```bash
go get codeberg.org/ubunatic/cati
```

## Usage Example

Below is a complete, compileable example of using Cati as a library:

<!-- GO_EXAMPLE_START -->
```go
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"
)

func main() {
	// Create a simple test image (a diagonal red line on blue background)
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if x == y {
				img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
			}
		}
	}

	fmt.Println("--- Example 1: Rendering ANSI directly to Stdout ---")
	// Render using the halfblock algorithm at 20 terminal columns width.
	// Width is mandatory (20). Height is unconstrained (Opts.Rows = 0).
	err := halfblock.Render(os.Stdout, img, 20, halfblock.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Example 2: Rendering to a core.Grid (for TUIs) ---")
	// Render using the quadblock algorithm with edge-snap enabled.
	opts := quadblock.Options{
		EdgeSnap: true,
	}
	grid, err := quadblock.RenderToGrid(img, 20, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering to grid: %v\n", err)
		os.Exit(1)
	}

	// Print out the grid cell runes (ignoring colors for simplicity in stdout)
	for y, row := range grid.Cells {
		fmt.Printf("Row %02d: ", y)
		for _, cell := range row {
			fmt.Printf("%c", cell.Ch)
		}
		fmt.Println()
	}
}
```
<!-- GO_EXAMPLE_END -->

## Loading SVGs

`halfblock.LoadImage(path)` supports PNG, JPEG, SVG, and first-frame video
loading. SVGs are rasterized through `rsvg-convert`; callers that already know
their render pixel budget should prefer `halfblock.LoadImageWithTarget(path,
maxWidth, maxHeight)` so vector inputs are rasterized directly to the target
box instead of through the default 2048px long-edge fallback.
