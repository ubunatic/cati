package main

import (
	"fmt"
	"math"
	"time"

	"codeberg.org/ubunatic/loom"

	"ubunatic.com/cati/v1/core"
)

// imagePreview is a loom.Widget that renders an image file via cati's block
// renderers directly into the shared loom.Canvas, cell by cell.
// It bypasses loom.View on purpose: View strips inline ANSI from its lines,
// which would throw away the colors cati renders.
//
// Rendering is asynchronous: the worker goroutine produces results that Draw
// picks up on the next frame, so navigation is never stalled by a slow render.
//
// When focused (Tab to switch pane):
//   - Arrow keys / scroll-wheel pan the image.
//   - +/= zoom in, - zoom out, 0 resets zoom.
//   - m cycles render mode forward, Shift-M backwards.
type imagePreview struct {
	// desired state (set from the main goroutine via SetPath/SetMessage/SetMode)
	wantPath string
	wantMode renderMode
	message  string // shown when wantPath is empty (dirs, errors)

	// cached render result — only updated in Draw (main goroutine), so no mutex needed
	grid     *core.Grid
	gridMsg  string
	gridPath string
	gridCols int
	gridRows int
	gridMode renderMode
	gridDur   time.Duration
	gridSSIM  float64
	gridStats renderStats
	showInfo  bool

	// last submitted request — used to avoid duplicate submits
	subPath string
	subCols int
	subRows int
	subMode renderMode

	// async worker
	rend *renderer

	focused    bool
	panX, panY int
	zoomLevel  int

	// saved for mouse hit-testing
	lastH    int // height of preview rect last Draw
	modeBtns []modeBtn
}

func (p *imagePreview) ToggleInfo() {
	p.showInfo = !p.showInfo
}

type modeBtn struct {
	mode renderMode
	x, w int // 0-based x offset from left of inner rect, and width in runes
}

const (
	zoomMin  = -4
	zoomMax  = 8
	zoomBase = 1.5
)

func newImagePreview() *imagePreview {
	return &imagePreview{rend: newRenderer()}
}

func (p *imagePreview) zoomedCols(viewW int) int {
	cols := int(math.Round(float64(viewW) * math.Pow(zoomBase, float64(p.zoomLevel))))
	if cols < 1 {
		cols = 1
	}
	return cols
}

func (p *imagePreview) Focused() bool         { return p.focused }
func (p *imagePreview) SetFocus(focused bool) { p.focused = focused }

func (p *imagePreview) SetPath(path string) {
	if path == p.wantPath && path != "" {
		return
	}
	p.wantPath = path
	p.message = ""
	p.panX, p.panY = 0, 0
	p.zoomLevel = 0
	// Don't clear the cached grid — keep showing the previous image while
	// the new one renders (stale-while-loading pattern).
}

func (p *imagePreview) SetMessage(msg string) {
	p.wantPath = ""
	p.message = msg
	p.panX, p.panY = 0, 0
	p.zoomLevel = 0
}

func (p *imagePreview) SetMode(m renderMode) {
	if m == p.wantMode {
		return
	}
	p.wantMode = m
	// Force resubmit on next Draw by clearing the submitted state.
	p.subPath = ""
	p.grid = nil
}

func (p *imagePreview) CycleMode()     { p.SetMode(p.wantMode.Next()) }
func (p *imagePreview) CycleModePrev() { p.SetMode(p.wantMode.Prev()) }

// clampPan keeps pan offsets within the renderable area.
func (p *imagePreview) clampPan(viewW, viewH int) {
	if p.grid == nil {
		p.panX, p.panY = 0, 0
		return
	}
	if maxX := p.grid.Width - viewW; p.panX > maxX {
		if maxX < 0 {
			maxX = 0
		}
		p.panX = maxX
	}
	if p.panX < 0 {
		p.panX = 0
	}
	if maxY := p.grid.Height - viewH; p.panY > maxY {
		if maxY < 0 {
			maxY = 0
		}
		p.panY = maxY
	}
	if p.panY < 0 {
		p.panY = 0
	}
}

func (p *imagePreview) Draw(c *loom.Canvas, r loom.Rect) {
	p.lastH = r.H

	// Poll for a completed async render.
	if res := p.rend.poll(); res != nil {
		if res.path == p.wantPath && res.mode == p.wantMode {
			p.grid, p.gridMsg, p.gridDur, p.gridSSIM, p.gridStats = res.grid, res.msg, res.dur, res.ssim, res.stats
			p.gridPath, p.gridCols, p.gridRows, p.gridMode = res.path, res.cols, res.rows, res.mode
		}
	}

	// The image area is the rect minus the bottom mode-bar row.
	imgH := r.H - 1
	if imgH < 0 {
		imgH = 0
	}
	p.drawModeBar(c, r.X, r.Y+r.H-1, r.W)

	// Non-image state (directory, error, no selection).
	if p.wantPath == "" {
		msg := p.message
		if msg == "" {
			msg = "No selection"
		}
		c.Write(r.X, r.Y, msg, loom.Style{})
		return
	}

	// When zoomLevel is 0 (default), fit within both width (r.W) and visible height (imgH).
	// When zoomed, scale columns and leave rows unconstrained for pan navigation.
	var wantCols, wantRows int
	if p.zoomLevel == 0 {
		wantCols = r.W
		wantRows = imgH
	} else {
		wantCols = p.zoomedCols(r.W)
		wantRows = 0
	}

	// Submit a render if the cached result is stale or missing.
	needsRender := p.gridPath != p.wantPath || p.gridCols != wantCols || p.gridRows != wantRows || p.gridMode != p.wantMode
	alreadyPending := p.subPath == p.wantPath && p.subCols == wantCols && p.subRows == wantRows && p.subMode == p.wantMode
	if needsRender && !alreadyPending {
		p.rend.submit(p.wantPath, wantCols, wantRows, p.wantMode)
		p.subPath, p.subCols, p.subRows, p.subMode = p.wantPath, wantCols, wantRows, p.wantMode
	}

	// If Info View is active, render stats table instead of the image pixels.
	if p.showInfo {
		p.drawInfoView(c, r.X, r.Y, r.W, imgH)
		return
	}

	// No grid yet — show loading or error message.
	if p.grid == nil {
		msg := p.gridMsg
		if msg == "" {
			msg = "rendering…"
		}
		c.Write(r.X, r.Y, msg, loom.Style{Dim: msg == "rendering…"})
		return
	}

	// Zoom hint in bottom-right of the image area (above the mode bar).
	if p.zoomLevel != 0 {
		pct := int(math.Round(math.Pow(zoomBase, float64(p.zoomLevel)) * 100))
		hint := fmt.Sprintf(" %d%% ", pct)
		hw := len([]rune(hint))
		hintY := r.Y + imgH - 1
		if r.W >= hw && hintY >= r.Y {
			c.Write(r.X+r.W-hw, hintY, hint, loom.Style{Dim: true})
		}
	}

	// Dim "…" in top-left while a new render is in-flight (stale image shown).
	if needsRender {
		c.Write(r.X, r.Y, "…", loom.Style{Dim: true})
	}

	p.clampPan(r.W, imgH)
	for y := 0; y < imgH; y++ {
		gy := p.panY + y
		if gy >= p.grid.Height {
			break
		}
		for x := 0; x < r.W; x++ {
			gx := p.panX + x
			if gx >= p.grid.Width {
				break
			}
			cell := p.grid.Cells[gy][gx]
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

func (p *imagePreview) drawInfoView(c *loom.Canvas, x, y, w, h int) {
	st := p.gridStats
	var lines []string

	// File info
	sizeKB := float64(st.fileSize) / 1024.0
	lines = append(lines, fmt.Sprintf("File:       %s", p.wantPath))
	if st.fileSize > 0 {
		lines = append(lines, fmt.Sprintf("Size:       %.1f KB (%d bytes)", sizeKB, st.fileSize))
	}
	if st.origW > 0 && st.origH > 0 {
		lines = append(lines, fmt.Sprintf("Dimensions: %d × %d px", st.origW, st.origH))
	}
	lines = append(lines, "")

	// Render & Mode stats
	lines = append(lines, fmt.Sprintf("Mode:       %s (cell %d×%d)", p.wantMode.String(), st.cellW, st.cellH))
	if st.gridW > 0 && st.gridH > 0 {
		lines = append(lines, fmt.Sprintf("Grid size:  %d × %d cells", st.gridW, st.gridH))
	}
	if st.subW > 0 && st.subH > 0 {
		lines = append(lines, fmt.Sprintf("Sub-pixels: %d × %d px", st.subW, st.subH))
	}
	if st.origW > 0 && st.origH > 0 && st.subW > 0 && st.subH > 0 {
		scaleW := float64(st.subW) / float64(st.origW) * 100.0
		scaleH := float64(st.subH) / float64(st.origH) * 100.0
		areaPct := (float64(st.subW*st.subH) / float64(st.origW*st.origH)) * 100.0
		lines = append(lines, fmt.Sprintf("Scaling:    W: %.1f%%  H: %.1f%%  (area: %.1f%%)", scaleW, scaleH, areaPct))
	}
	if st.dur > 0 {
		durStr := fmt.Sprintf("%dms", st.dur.Milliseconds())
		if st.dur.Milliseconds() == 0 {
			durStr = fmt.Sprintf("%.2fms", float64(st.dur.Microseconds())/1000.0)
		}
		lines = append(lines, fmt.Sprintf("Duration:   %s", durStr))
	}
	if st.ssim > 0 {
		lines = append(lines, fmt.Sprintf("SSIM score: %.4f", st.ssim))
	}

	for idx, line := range lines {
		if idx >= h {
			break
		}
		style := loom.Style{}
		if idx == 0 || idx == 4 {
			style.Bold = true
		}
		c.Write(x, y+idx, line, style)
	}
}


// drawModeBar renders the [six] [half] [quad] [all] button row along with render time and SSIM,
// and records button bounds for mouse hit-testing.
func (p *imagePreview) drawModeBar(c *loom.Canvas, x, y, w int) {
	p.modeBtns = p.modeBtns[:0]
	xOff := 0
	for m := modeSix; m <= modeAll; m++ {
		label := "[" + m.String() + "]"
		lw := len([]rune(label))
		if xOff+lw > w {
			break
		}
		style := loom.Style{Dim: true}
		if m == p.wantMode {
			style = loom.Style{Bold: true}
		}
		c.Write(x+xOff, y, label, style)
		p.modeBtns = append(p.modeBtns, modeBtn{mode: m, x: xOff, w: lw})
		xOff += lw + 1 // 1-space gap between buttons
	}

	// Show render time and SSIM score after mode selectors if space permits.
	if p.gridDur > 0 && xOff+5 <= w {
		durStr := fmt.Sprintf("%dms", p.gridDur.Milliseconds())
		if p.gridDur.Milliseconds() == 0 {
			durStr = fmt.Sprintf("%.1fms", float64(p.gridDur.Microseconds())/1000.0)
		}
		info := durStr
		if p.gridSSIM > 0 {
			if xOff+len(durStr)+1+10 <= w {
				info = fmt.Sprintf("%s ssim=%.2f", durStr, p.gridSSIM)
			} else if xOff+len(durStr)+1+5 <= w {
				info = fmt.Sprintf("%s %.2f", durStr, p.gridSSIM)
			}
		}
		c.Write(x+xOff, y, info, loom.Style{Dim: true})
	}
}

// applyZoomKey handles +/=/−/0 zoom steps. Called from HandleKey (preview
// pane focused) and from the browser (zoom forwarded from the files pane).
func (p *imagePreview) applyZoomKey(text string) {
	switch text {
	case "+", "=":
		if p.zoomLevel < zoomMax {
			p.zoomLevel++
			p.subPath = "" // force resubmit at new size
		}
	case "-":
		if p.zoomLevel > zoomMin {
			p.zoomLevel--
			p.subPath = ""
		}
	case "0":
		p.zoomLevel = 0
		p.panX, p.panY = 0, 0
		p.subPath = ""
	}
}

// HandleKey pans and zooms the image when the preview pane is focused.
//
//	Arrow keys — pan 1 cell/row     PgUp/PgDn — pan 10 rows
//	Home       — reset pan          m/M       — cycle render mode forward/backward
//	+/=        — zoom in            -         — zoom out      0 — reset zoom
func (p *imagePreview) HandleKey(e loom.KeyEvent) bool {
	if !p.focused {
		return false
	}
	const pageStep = 10
	switch e.Key {
	case "up":
		p.panY--
	case "down":
		p.panY++
	case "left":
		p.panX--
	case "right":
		p.panX++
	case "pgup", "pageup":
		p.panY -= pageStep
	case "pgdn", "pgdown", "pagedown":
		p.panY += pageStep
	case "home":
		p.panX, p.panY = 0, 0
	case "shift-m", "shift-M":
		p.CycleModePrev()
		return false
	}
	switch e.Text {
	case "#":
		p.ToggleInfo()
	case "m":
		p.CycleMode()
	case "M":
		p.CycleModePrev()
	default:
		p.applyZoomKey(e.Text)
	}
	return false
}

// HandleMouse pans with the scroll wheel and handles mode-bar clicks.
func (p *imagePreview) HandleMouse(e loom.MouseEvent) bool {
	switch e.Action {
	case loom.MouseScrollUp:
		p.panY--
	case loom.MouseScrollDown:
		p.panY++
	case loom.MousePress:
		if e.Button == loom.MouseLeft && e.Y == p.lastH {
			// Click in the mode bar (last row, 1-based coords from inner rect).
			clickX := e.X - 1 // convert to 0-based
			for _, btn := range p.modeBtns {
				if clickX >= btn.x && clickX < btn.x+btn.w {
					p.SetMode(btn.mode)
					return false
				}
			}
		}
	}
	return false
}

func (p *imagePreview) ContentHeight() int {
	if p.grid == nil {
		return 2 // image row + mode bar
	}
	return p.grid.Height + 1 // +1 for mode bar
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
