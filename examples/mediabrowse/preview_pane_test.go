package main

import (
	"errors"
	"path/filepath"
	"testing"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/media"
)

func withWidgetFactories(t *testing.T) {
	t.Helper()
	oldImage, oldVideo := loadImageWidget, newVideoWidget
	t.Cleanup(func() { loadImageWidget, newVideoWidget = oldImage, oldVideo })
}

func TestPreviewPaneConstructionStateAndMessages(t *testing.T) {
	p := newMediaPreviewPane(media.ModeHalfblock, 0)
	if p.fps != 24 || p.Title() != "Preview" {
		t.Fatalf("initial preview state: fps=%v title=%q", p.fps, p.Title())
	}
	p.SetMessage("No selection")
	if p.currentPath != "" || p.message != "No selection" {
		t.Fatalf("message state: path=%q message=%q", p.currentPath, p.message)
	}
	p.SetFocus(true)
	if !p.Focused() {
		t.Fatal("SetFocus(true) did not set focus")
	}
	p.SetFocus(false)
	if p.Focused() {
		t.Fatal("SetFocus(false) did not clear focus")
	}
}

func TestPreviewPaneModeCycleAndImageDispatch(t *testing.T) {
	withWidgetFactories(t)
	var modes []media.Mode
	loadImageWidget = func(_ string, mode media.Mode) (*media.Widget, error) {
		modes = append(modes, mode)
		return nil, nil
	}
	newVideoWidget = func(string, media.Mode, float64) (*media.Widget, error) {
		t.Fatal("still image was dispatched to video loader")
		return nil, nil
	}
	p := newMediaPreviewPane(media.ModeHalfblock, 30)
	p.SetPath("still.png")
	for _, want := range []media.Mode{media.ModeQuadblock, media.ModeSextant, media.ModeHalfblock} {
		p.CycleMode()
		if p.mode != want {
			t.Errorf("mode after cycle = %q, want %q", p.mode, want)
		}
	}
	if len(modes) != 4 {
		t.Fatalf("image loader called %d times, want initial load plus three mode changes", len(modes))
	}
	if p.Title() != "🖼️ still.png [halfblock]" {
		t.Errorf("still title = %q", p.Title())
	}
	p.Close()
	p.Close()
	if p.widget != nil {
		t.Fatal("Close left the widget set")
	}
}

func TestPreviewPaneVideoDispatchAndErrors(t *testing.T) {
	withWidgetFactories(t)
	imageCalls, videoCalls := 0, 0
	loadImageWidget = func(path string, mode media.Mode) (*media.Widget, error) {
		imageCalls++
		if filepath.Ext(path) != ".mp4" || mode != media.ModeHalfblock {
			t.Errorf("image loader args = %q, %q", path, mode)
		}
		return nil, nil
	}
	newVideoWidget = func(path string, mode media.Mode, fps float64) (*media.Widget, error) {
		videoCalls++
		if filepath.Ext(path) != ".mp4" || mode != media.ModeHalfblock || fps != 18 {
			t.Errorf("video loader args = %q, %q, %v", path, mode, fps)
		}
		return nil, nil
	}
	p := newMediaPreviewPane(media.ModeHalfblock, 18)
	p.SetPath("clip.mp4")
	if imageCalls != 1 || videoCalls != 0 || p.playing {
		t.Fatalf("paused video dispatch: image=%d video=%d playing=%v", imageCalls, videoCalls, p.playing)
	}
	p.TogglePlay()
	if imageCalls != 1 || videoCalls != 1 || !p.playing {
		t.Fatalf("playing video dispatch: image=%d video=%d playing=%v", imageCalls, videoCalls, p.playing)
	}
	if title := p.Title(); title != "🎬 clip.mp4 [halfblock, playing]" {
		t.Errorf("playing title = %q", title)
	}
	p.SetMessage("replaced")
	if p.currentPath != "" || p.message != "replaced" || p.playing != true {
		t.Errorf("SetMessage state: path=%q message=%q playing=%v", p.currentPath, p.message, p.playing)
	}

	loadImageWidget = func(string, media.Mode) (*media.Widget, error) { return nil, errors.New("bad image") }
	p.SetPath("broken.jpg")
	if p.message != "Load error: bad image" {
		t.Errorf("image error message = %q", p.message)
	}
}

func TestPreviewPaneKeyHandlingAndStillPlayNoop(t *testing.T) {
	withWidgetFactories(t)
	loadImageWidget = func(string, media.Mode) (*media.Widget, error) { return nil, nil }
	newVideoWidget = func(string, media.Mode, float64) (*media.Widget, error) {
		t.Fatal("still image play toggle attempted video load")
		return nil, nil
	}
	p := newMediaPreviewPane(media.ModeHalfblock, 24)
	p.SetPath("still.png")
	p.HandleKey(loom.KeyEvent{Text: "m"})
	if p.mode != media.ModeQuadblock {
		t.Fatalf("mode after key = %q", p.mode)
	}
	p.HandleKey(loom.KeyEvent{Text: "p"})
	if p.playing {
		t.Fatal("still image should not enter playing state")
	}
	if interval := p.TickInterval(); interval != 0 {
		t.Fatalf("empty test widget tick interval = %v, want zero", interval)
	}
}
