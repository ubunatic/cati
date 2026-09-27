package main

import (
	"context"
	"errors"
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/media"

	"ubunatic.com/cati/v1/core"
)

func withWidgetFactories(t *testing.T) {
	t.Helper()
	oldImage, oldVideo := loadImageWidget, newVideoWidget
	t.Cleanup(func() { loadImageWidget, newVideoWidget = oldImage, oldVideo })
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	start := time.Now()
	for time.Since(start) < timeout {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
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
	loadImageWidget = func(_ context.Context, _ string, mode media.Mode, _ func(core.Progress)) (*media.Widget, error) {
		modes = append(modes, mode)
		return nil, nil
	}
	newVideoWidget = func(string, media.Mode, float64) (*media.Widget, error) {
		t.Fatal("still image was dispatched to video loader")
		return nil, nil
	}
	p := newMediaPreviewPane(media.ModeHalfblock, 30)
	p.SetPath("still.png")
	waitForCondition(t, 200*time.Millisecond, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return !p.loading && len(modes) == 1
	})

	for _, want := range []media.Mode{media.ModeQuadblock, media.ModeSextant, media.ModeHalfblock} {
		p.CycleMode()
		waitForCondition(t, 200*time.Millisecond, func() bool {
			p.mu.Lock()
			defer p.mu.Unlock()
			return !p.loading && p.mode == want
		})
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
	loadImageWidget = func(_ context.Context, path string, mode media.Mode, _ func(core.Progress)) (*media.Widget, error) {
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
	waitForCondition(t, 200*time.Millisecond, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return !p.loading && imageCalls == 1
	})
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
	if p.currentPath != "" || p.message != "replaced" || p.playing {
		t.Errorf("SetMessage state: path=%q message=%q playing=%v", p.currentPath, p.message, p.playing)
	}

	loadImageWidget = func(_ context.Context, _ string, _ media.Mode, _ func(core.Progress)) (*media.Widget, error) {
		return nil, errors.New("bad image")
	}
	p.SetPath("broken.jpg")
	waitForCondition(t, 200*time.Millisecond, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return !p.loading && p.message == "Load error: bad image"
	})
}

func TestPreviewPaneProgressAndCancellation(t *testing.T) {
	withWidgetFactories(t)
	unblock := make(chan struct{})
	firstStarted := make(chan struct{})

	loadImageWidget = func(ctx context.Context, path string, mode media.Mode, onProgress func(core.Progress)) (*media.Widget, error) {
		if path == "slow.png" {
			close(firstStarted)
			onProgress(core.Progress{Stage: "decoding", Ratio: 0.42})
			select {
			case <-unblock:
				return nil, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return nil, nil
	}

	p := newMediaPreviewPane(media.ModeHalfblock, 24)
	p.SetPath("slow.png")
	<-firstStarted

	waitForCondition(t, 200*time.Millisecond, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.loading && strings.Contains(p.loadingMsg, "42%")
	})

	c := loom.NewCanvas(40, 10)
	p.Draw(c, loom.Rect{W: 40, H: 10})
	if !strings.Contains(p.Title(), "loading...") {
		t.Errorf("title during load = %q, want loading indicator", p.Title())
	}
	if p.TickInterval() != 50*time.Millisecond {
		t.Errorf("loading tick interval = %v, want 50ms", p.TickInterval())
	}

	// Setting a new path cancels the previous slow load
	p.SetPath("fast.png")
	waitForCondition(t, 200*time.Millisecond, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return !p.loading && p.currentPath == "fast.png"
	})
	close(unblock)
}

func TestPreviewPaneKeyHandlingAndStillPlayNoop(t *testing.T) {
	withWidgetFactories(t)
	loadImageWidget = func(_ context.Context, _ string, _ media.Mode, _ func(core.Progress)) (*media.Widget, error) {
		return nil, nil
	}
	newVideoWidget = func(string, media.Mode, float64) (*media.Widget, error) {
		t.Fatal("still image play toggle attempted video load")
		return nil, nil
	}
	p := newMediaPreviewPane(media.ModeHalfblock, 24)
	p.SetPath("still.png")
	waitForCondition(t, 200*time.Millisecond, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return !p.loading
	})
	p.HandleKey(loom.KeyEvent{Text: "m"})
	waitForCondition(t, 200*time.Millisecond, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return !p.loading && p.mode == media.ModeQuadblock
	})
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

func TestPreviewPaneVideoPlayDuringAsyncStillLoad(t *testing.T) {
	withWidgetFactories(t)
	stillStarted := make(chan struct{})
	unblockStill := make(chan struct{})

	loadImageWidget = func(ctx context.Context, path string, mode media.Mode, onProgress func(core.Progress)) (*media.Widget, error) {
		close(stillStarted)
		<-unblockStill
		return nil, errors.New("simulated background failure after cancel")
	}

	fakeVideoWidget, err := media.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)), media.ModeHalfblock)
	if err != nil {
		t.Fatal(err)
	}
	newVideoWidget = func(path string, mode media.Mode, fps float64) (*media.Widget, error) {
		return fakeVideoWidget, nil
	}

	p := newMediaPreviewPane(media.ModeHalfblock, 24)
	p.SetPath("clip.mp4")
	<-stillStarted

	// Trigger play while still frame load is in-flight
	p.TogglePlay()
	if !p.playing {
		t.Fatal("expected playing to be true after TogglePlay")
	}
	if p.widget != fakeVideoWidget {
		t.Fatal("expected video widget to be set")
	}

	// Unblock the still frame goroutine and ensure it does not discard the video widget
	close(unblockStill)
	time.Sleep(20 * time.Millisecond)

	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing {
		t.Fatal("playing was corrupted by background goroutine")
	}
	if p.widget != fakeVideoWidget {
		t.Fatal("video widget was closed/replaced by background goroutine")
	}
	if p.message != "" {
		t.Fatalf("unexpected message set: %q", p.message)
	}
}
