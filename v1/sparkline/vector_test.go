package sparkline_test

import (
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/sparkline"
)

func TestVectorFlatQuarterEdgeUsesStraightBar(t *testing.T) {
	// vec8 (4x8): a quarter of the cell height is a whole number of rows.
	vec, err := spec.ResolveGlyphSetExpression("vec8")
	if err != nil {
		t.Fatal(err)
	}
	w, h := vec.Geometry.W, vec.Geometry.H
	if h%4 != 0 {
		t.Fatalf("vec8 geometry %dx%d: height must be divisible by 4", w, h)
	}
	shapes := make([]sparkline.Shape, 0, len(vec.Shapes))
	for _, shape := range vec.Shapes {
		shapes = append(shapes, sparkline.Shape{Ch: shape.Glyph, Width: w, Height: h, Mask: shape.Mask})
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{A: 255}
			if y < h/4 {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	grid, err := sparkline.RenderToGrid(img, 1, sparkline.Options{Rows: 1, CellW: w, CellH: h, Shapes: shapes})
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Cells) != 1 || len(grid.Cells[0]) != 1 {
		t.Fatalf("grid dimensions = %dx%d, want one cell", grid.Width, grid.Height)
	}
	ch := grid.Cells[0][0].Ch
	if ch >= 0x1FB3C && ch <= 0x1FB67 {
		t.Fatalf("flat top quarter selected diagonal %U (%c)", ch, ch)
	}
	// Either coverage polarity is exact: upper 1/4 or inverted lower 3/4.
	if ch != 0x1FB82 && ch != '▆' {
		t.Fatalf("flat top quarter selected %U (%c), want a straight quarter bar", ch, ch)
	}
	cell := grid.Cells[0][0]
	wantFg, wantBg := img.RGBAAt(0, 0), img.RGBAAt(0, h-1)
	if ch == '▆' {
		wantFg, wantBg = wantBg, wantFg
	}
	if !cell.HasFg || !cell.HasBg || cell.Fg != wantFg || cell.Bg != wantBg {
		t.Errorf("bar colours = %+v, want foreground %v and background %v (zero-error fit)", cell, wantFg, wantBg)
	}
}
