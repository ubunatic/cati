package sparkline_test

import (
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/sparkline"
)

func TestVectorFlatQuarterEdgeUsesStraightBar(t *testing.T) {
	vec, err := spec.ResolveGlyphSetExpression("vector")
	if err != nil {
		t.Fatal(err)
	}
	shapes := make([]sparkline.Shape, 0, len(vec.Shapes))
	for _, shape := range vec.Shapes {
		shapes = append(shapes, sparkline.Shape{Ch: shape.Glyph, Width: 4, Height: 4, Mask: shape.Mask})
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			c := color.RGBA{A: 255}
			if y == 0 {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	grid, err := sparkline.RenderToGrid(img, 1, sparkline.Options{Rows: 1, CellW: 4, CellH: 4, Shapes: shapes})
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
	wantFg, wantBg := img.RGBAAt(0, 0), img.RGBAAt(0, 1)
	if ch == '▆' {
		wantFg, wantBg = wantBg, wantFg
	}
	if !cell.HasFg || !cell.HasBg || cell.Fg != wantFg || cell.Bg != wantBg {
		t.Errorf("bar colours = %+v, want foreground %v and background %v (zero-error fit)", cell, wantFg, wantBg)
	}
}
