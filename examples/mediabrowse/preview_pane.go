package main

import (
	"fmt"
	"path/filepath"
	"time"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/media"

	"ubunatic.com/cati/v1/halfblock"
)

type mediaPreviewPane struct {
	mode        media.Mode
	currentPath string
	message     string
	widget      *media.Widget
	fps         float64
	playing     bool
	focused     bool
	lastRect    loom.Rect
}

var (
	loadImageWidget = media.LoadImage
	newVideoWidget  = media.NewVideo
)

func newMediaPreviewPane(mode media.Mode, fps float64) *mediaPreviewPane {
	if fps <= 0 {
		fps = 24.0
	}
	return &mediaPreviewPane{
		mode: mode,
		fps:  fps,
	}
}

func (p *mediaPreviewPane) Close() {
	if p.widget != nil {
		p.widget.Close()
		p.widget = nil
	}
}

func (p *mediaPreviewPane) Title() string {
	if p.currentPath == "" {
		return "Preview"
	}
	base := filepath.Base(p.currentPath)
	if halfblock.IsVideo(p.currentPath) {
		status := "paused"
		if p.playing {
			status = "playing"
		}
		return fmt.Sprintf("🎬 %s [%s, %s]", base, p.mode, status)
	}
	return fmt.Sprintf("🖼️ %s [%s]", base, p.mode)
}

func (p *mediaPreviewPane) SetMessage(msg string) {
	p.Close()
	p.currentPath = ""
	p.message = msg
}

func (p *mediaPreviewPane) SetPath(path string) {
	if path == p.currentPath && p.widget != nil {
		return
	}
	p.currentPath = path
	p.message = ""
	p.loadWidget()
}

func (p *mediaPreviewPane) loadWidget() {
	p.Close()
	if p.currentPath == "" {
		return
	}
	if halfblock.IsVideo(p.currentPath) {
		if p.playing {
			w, err := newVideoWidget(p.currentPath, p.mode, p.fps)
			if err != nil {
				p.message = "Video error: " + err.Error()
				return
			}
			p.widget = w
		} else {
			w, err := loadImageWidget(p.currentPath, p.mode)
			if err != nil {
				p.message = "Preview error: " + err.Error()
				return
			}
			p.widget = w
		}
		return
	}

	w, err := loadImageWidget(p.currentPath, p.mode)
	if err != nil {
		p.message = "Load error: " + err.Error()
		return
	}
	p.widget = w
}

func (p *mediaPreviewPane) CycleMode() {
	switch p.mode {
	case media.ModeHalfblock:
		p.mode = media.ModeQuadblock
	case media.ModeQuadblock:
		p.mode = media.ModeSextant
	default:
		p.mode = media.ModeHalfblock
	}
	p.loadWidget()
}

func (p *mediaPreviewPane) TogglePlay() {
	if p.currentPath == "" || !halfblock.IsVideo(p.currentPath) {
		return
	}
	p.playing = !p.playing
	p.loadWidget()
}

func (p *mediaPreviewPane) Draw(c *loom.Canvas, r loom.Rect) {
	p.lastRect = r
	if p.message != "" {
		c.Write(r.X, r.Y, p.message, loom.Style{Dim: true})
		return
	}
	if p.widget != nil {
		p.widget.Draw(c, r)
		return
	}
	c.Write(r.X, r.Y, "No media loaded", loom.Style{Dim: true})
}

func (p *mediaPreviewPane) Tick(now time.Time) {
	if p.widget != nil {
		p.widget.Tick(now)
	}
}

func (p *mediaPreviewPane) TickInterval() time.Duration {
	if p.widget != nil {
		return p.widget.TickInterval()
	}
	return 0
}

func (p *mediaPreviewPane) HandleKey(k loom.KeyEvent) bool {
	switch k.Text {
	case "m", "M":
		p.CycleMode()
		return false
	case "p", "P":
		p.TogglePlay()
		return false
	}
	return false
}

func (p *mediaPreviewPane) HandleMouse(m loom.MouseEvent) bool {
	if m.Action == loom.MousePress && m.Button == loom.MouseLeft {
		p.TogglePlay()
	}
	return false
}

func (p *mediaPreviewPane) Focused() bool         { return p.focused }
func (p *mediaPreviewPane) SetFocus(focused bool) { p.focused = focused }
