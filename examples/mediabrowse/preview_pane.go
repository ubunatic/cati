package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/media"

	"ubunatic.com/cati/v1/core"
	"ubunatic.com/cati/v1/halfblock"
)

type mediaPreviewPane struct {
	mu          sync.Mutex
	mode        media.Mode
	currentPath string
	message     string
	loading     bool
	loadingMsg  string
	widget      *media.Widget
	fps         float64
	playing     bool
	focused     bool
	lastRect    loom.Rect
	cancel      context.CancelFunc
	generation  uint64
}

var (
	loadImageWidget = func(ctx context.Context, path string, mode media.Mode, onProgress func(core.Progress)) (*media.Widget, error) {
		img, err := halfblock.LoadImageContext(ctx, path, onProgress)
		if err != nil {
			return nil, err
		}
		return media.NewImage(img, mode)
	}
	newVideoWidget = media.NewVideo
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
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancelLoadingLocked()
	p.closeWidgetLocked()
}

func (p *mediaPreviewPane) cancelLoadingLocked() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.loading = false
	p.loadingMsg = ""
}

func (p *mediaPreviewPane) closeWidgetLocked() {
	if p.widget != nil {
		p.widget.Close()
		p.widget = nil
	}
}

func (p *mediaPreviewPane) Title() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.currentPath == "" {
		return "Preview"
	}
	base := filepath.Base(p.currentPath)
	if halfblock.IsVideo(p.currentPath) {
		status := "paused"
		if p.playing {
			status = "playing"
		} else if p.loading {
			status = "loading..."
		}
		return fmt.Sprintf("🎬 %s [%s, %s]", base, p.mode, status)
	}
	if p.loading {
		return fmt.Sprintf("🖼️ %s [%s, loading...]", base, p.mode)
	}
	return fmt.Sprintf("🖼️ %s [%s]", base, p.mode)
}

func (p *mediaPreviewPane) SetMessage(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancelLoadingLocked()
	p.closeWidgetLocked()
	p.generation++
	p.currentPath = ""
	p.playing = false
	p.message = msg
}

func (p *mediaPreviewPane) SetPath(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if path == p.currentPath && (p.widget != nil || p.loading) {
		return
	}
	p.currentPath = path
	p.playing = false
	p.message = ""
	p.loadWidgetLocked()
}

func (p *mediaPreviewPane) loadWidgetLocked() {
	p.cancelLoadingLocked()
	p.closeWidgetLocked()
	p.generation++
	if p.currentPath == "" {
		return
	}
	if halfblock.IsVideo(p.currentPath) && p.playing {
		w, err := newVideoWidget(p.currentPath, p.mode, p.fps)
		if err != nil {
			p.message = "Video error: " + err.Error()
			return
		}
		p.widget = w
		return
	}

	p.loading = true
	p.loadingMsg = "Loading..."
	gen := p.generation
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	path := p.currentPath
	mode := p.mode

	go func(gen uint64, path string, mode media.Mode) {
		w, err := loadImageWidget(ctx, path, mode, func(prog core.Progress) {
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.generation != gen || !p.loading {
				return
			}
			if prog.Ratio > 0 {
				if prog.Stage != "" {
					p.loadingMsg = fmt.Sprintf("Loading %.0f%% (%s)", prog.Ratio*100, prog.Stage)
				} else {
					p.loadingMsg = fmt.Sprintf("Loading %.0f%%", prog.Ratio*100)
				}
			} else if prog.Stage != "" {
				p.loadingMsg = fmt.Sprintf("Loading (%s)...", prog.Stage)
			}
		})

		p.mu.Lock()
		defer p.mu.Unlock()
		if p.generation != gen {
			if w != nil {
				w.Close()
			}
			return
		}
		p.loading = false
		p.loadingMsg = ""
		p.cancel = nil
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			if halfblock.IsVideo(path) {
				p.message = "Preview error: " + err.Error()
			} else {
				p.message = "Load error: " + err.Error()
			}
			return
		}
		p.closeWidgetLocked()
		p.widget = w
	}(gen, path, mode)
}

func (p *mediaPreviewPane) CycleMode() {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch p.mode {
	case media.ModeHalfblock:
		p.mode = media.ModeQuadblock
	case media.ModeQuadblock:
		p.mode = media.ModeSextant
	default:
		p.mode = media.ModeHalfblock
	}
	p.loadWidgetLocked()
}

func (p *mediaPreviewPane) TogglePlay() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.currentPath == "" || !halfblock.IsVideo(p.currentPath) {
		return
	}
	p.playing = !p.playing
	p.loadWidgetLocked()
}

func (p *mediaPreviewPane) Draw(c *loom.Canvas, r loom.Rect) {
	p.mu.Lock()
	p.lastRect = r
	msg := p.message
	loading := p.loading
	loadingMsg := p.loadingMsg
	w := p.widget
	p.mu.Unlock()

	if msg != "" {
		c.Write(r.X, r.Y, msg, loom.Style{Dim: true})
		return
	}
	if loading {
		if loadingMsg == "" {
			loadingMsg = "Loading..."
		}
		c.Write(r.X, r.Y, loadingMsg, loom.Style{Dim: true})
		return
	}
	if w != nil {
		w.Draw(c, r)
		return
	}
	c.Write(r.X, r.Y, "No media loaded", loom.Style{Dim: true})
}

func (p *mediaPreviewPane) Tick(now time.Time) {
	p.mu.Lock()
	w := p.widget
	p.mu.Unlock()
	if w != nil {
		w.Tick(now)
	}
}

func (p *mediaPreviewPane) TickInterval() time.Duration {
	p.mu.Lock()
	w := p.widget
	loading := p.loading
	fps := p.fps
	playing := p.playing
	p.mu.Unlock()
	if w != nil {
		if interval := w.TickInterval(); interval > 0 {
			return interval
		}
	}
	if playing && fps > 0 {
		return time.Duration(float64(time.Second) / fps)
	}
	if loading {
		return 50 * time.Millisecond
	}
	return 0
}

func (p *mediaPreviewPane) HandleKey(k loom.KeyEvent) bool {
	if k.Is("m") {
		p.CycleMode()
		return false
	}
	if k.Is("p") {
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

func (p *mediaPreviewPane) Focused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.focused
}

func (p *mediaPreviewPane) SetFocus(focused bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.focused = focused
}
