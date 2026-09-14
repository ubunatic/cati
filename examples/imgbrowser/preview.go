package main

import (
	"codeberg.org/ubunatic/loom"

	"ubunatic.com/cati/v1/core"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/sextant"
)

// imagePreview is a loom.Widget that renders an image file via cati's
// sextant renderer directly into the shared loom.Canvas, cell by cell.
// It bypasses loom.View on purpose: View strips inline ANSI from its lines,
// which would throw away the colors cati renders.
type imagePreview struct {
	path    string // file to render; set by the browser on selection change
	message string // shown instead of a render, e.g. for non-images or errors

	renderedPath string
	renderedCols int
	grid         *core.Grid
	focused      bool
}

func (p *imagePreview) Focused() bool         { return p.focused }
func (p *imagePreview) SetFocus(focused bool) { p.focused = focused }

func (p *imagePreview) SetPath(path string) {
	if path == p.path {
		return
	}
	p.path, p.message, p.grid = path, "", nil
}

func (p *imagePreview) SetMessage(msg string) {
	p.path, p.message, p.grid = "", msg, nil
}

// ensureRendered (re)renders the current path at cols columns, caching the
// result so repeated Draw calls at the same width are free.
func (p *imagePreview) ensureRendered(cols int) {
	if p.path == "" || cols < 1 {
		return
	}
	if p.grid != nil && p.renderedPath == p.path && p.renderedCols == cols {
		return
	}
	img, err := halfblock.LoadImage(p.path)
	if err != nil {
		p.message, p.grid = "Not an image: "+err.Error(), nil
		p.renderedPath, p.renderedCols = p.path, cols
		return
	}
	grid, err := sextant.RenderToGrid(img, cols, sextant.Options{})
	if err != nil {
		p.message, p.grid = "Render failed: "+err.Error(), nil
		p.renderedPath, p.renderedCols = p.path, cols
		return
	}
	p.grid, p.message = grid, ""
	p.renderedPath, p.renderedCols = p.path, cols
}

func (p *imagePreview) Draw(c *loom.Canvas, r loom.Rect) {
	p.ensureRendered(r.W)
	if p.grid == nil {
		msg := p.message
		if msg == "" {
			msg = "No selection"
		}
		c.Write(r.X, r.Y, msg, loom.Style{})
		return
	}
	for y := 0; y < r.H && y < p.grid.Height; y++ {
		for x := 0; x < r.W && x < p.grid.Width; x++ {
			cell := p.grid.Cells[y][x]
			style := loom.Style{}
			if cell.HasFg {
				style.FG = loom.ColorRGB(cell.Fg.R, cell.Fg.G, cell.Fg.B)
			}
			if cell.HasBg {
				style.BG = loom.ColorRGB(cell.Bg.R, cell.Bg.G, cell.Bg.B)
			}
			ch := cell.Ch
			if ch == 0 {
				ch = ' '
			}
			c.Set(r.X+x, r.Y+y, loom.Cell{Text: string(ch), Style: style})
		}
	}
}

func (p *imagePreview) HandleKey(loom.KeyEvent) bool     { return false }
func (p *imagePreview) HandleMouse(loom.MouseEvent) bool { return false }

func (p *imagePreview) ContentHeight() int {
	if p.grid == nil {
		return 1
	}
	return p.grid.Height
}
