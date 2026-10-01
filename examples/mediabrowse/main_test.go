package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/loom"
	"ubunatic.com/loom/media"
)

func resetFlags() {
	flagTheme = ""
	flagMode = ""
	flagFPS = 0
	flagImagesOnly = false
}

func TestRootCommandFlagsAndForwarding(t *testing.T) {
	resetFlags()
	var gotDir string
	var got options
	cmd := newRootCmdWithRunner(func(dir string, opts options) error {
		gotDir, got = dir, opts
		return nil
	})
	cmd.SetArgs([]string{"--theme", "mc-dark", "--mode", "sextant", "--fps", "12.5", "--images-only", "./media"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotDir != "./media" {
		t.Fatalf("directory = %q, want ./media", gotDir)
	}
	if got != (options{themeName: "mc-dark", modeStr: "sextant", fps: 12.5, imagesOnly: true}) {
		t.Fatalf("options = %+v", got)
	}
	if cmd.Use != "mediabrowse [path]" {
		t.Errorf("usage = %q", cmd.Use)
	}
	for _, name := range []string{"theme", "mode", "fps", "images-only"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}
}

func TestThemeFlagHelpListsSpeccedThemes(t *testing.T) {
	resetFlags()
	cmd := newRootCmdWithRunner(func(string, options) error { return nil })
	usage := cmd.Flags().Lookup("theme").Usage
	for name := range loom.SpeccedThemes {
		if !strings.Contains(usage, name) {
			t.Errorf("--theme help %q does not list Loom theme %q", usage, name)
		}
	}
	if strings.Contains(usage, "solarized") || strings.Contains(usage, "monokai") || strings.Contains(usage, "nord") {
		t.Errorf("--theme help contains unsupported themes: %q", usage)
	}
}

func TestRunBrowserWithFilePath(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "subdir")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(child, "target.png")
	if err := os.WriteFile(targetFile, []byte("pngdata"), 0o600); err != nil {
		t.Fatal(err)
	}

	a, err := newApp(child, "mc", media.ModeHalfblock, 24, false, 20, "target.png")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if a.dir != child {
		t.Fatalf("app directory = %q, want %q", a.dir, child)
	}
	selected, ok := a.navigation.Selected()
	if !ok || filepath.Base(selected.Path) != "target.png" {
		t.Fatalf("selected entry = %+v (ok=%v), want target.png", selected, ok)
	}
	if a.preview.currentPath != targetFile {
		t.Fatalf("preview path = %q, want %q", a.preview.currentPath, targetFile)
	}
}

func TestRootCommandDefaultsAndArgumentErrors(t *testing.T) {
	resetFlags()
	called := false
	cmd := newRootCmdWithRunner(func(dir string, opts options) error {
		called = true
		if dir != "." || opts.themeName != "mc" || opts.modeStr != "halfblock" || opts.fps != 24 || opts.imagesOnly {
			t.Errorf("default invocation = dir %q, options %+v", dir, opts)
		}
		return nil
	})
	cmd.SetArgs(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("runner was not called")
	}

	resetFlags()
	called = false
	cmd = newRootCmdWithRunner(func(string, options) error { called = true; return nil })
	cmd.SetArgs([]string{"one", "two"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected too-many-arguments error")
	}
	if called {
		t.Fatal("runner called despite invalid arguments")
	}
}

func TestRootCommandReportsInvalidDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cmd := newRootCmd()
	cmd.SetArgs([]string{missing})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("Execute() error = %v, want missing directory error", err)
	}
}

func TestRealVideoPlaybackIntegration(t *testing.T) {
	videoPath := filepath.Join("..", "..", "testdata", "baby-60p-nn.webm")
	if _, err := os.Stat(videoPath); err != nil {
		t.Skip("testdata/baby-60p-nn.webm not found, skipping real video test")
	}

	a, err := newApp(filepath.Dir(videoPath), "mc", media.ModeHalfblock, 24, false, 20, filepath.Base(videoPath))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	// Initial selection shows paused or loading
	if !strings.Contains(a.preview.Title(), "baby-60p-nn.webm") {
		t.Fatalf("preview title = %q", a.preview.Title())
	}

	// 1. Toggle play
	a.ConsumeKey(loom.KeyEvent{Text: "p"})
	if !a.preview.playing {
		t.Fatal("expected video to enter playing state")
	}

	// Tick multiple times and draw
	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)
		a.Tick(time.Now())
	}

	c := loom.NewCanvas(60, 20)
	a.Draw(c, loom.Rect{W: 60, H: 20})
	if title := a.preview.Title(); !strings.Contains(title, "playing") {
		t.Fatalf("preview title = %q, want playing", title)
	}
	if interval := a.TickInterval(); interval <= 0 {
		t.Fatalf("playing tick interval = %v, want > 0", interval)
	}

	// 2. Pause
	a.ConsumeKey(loom.KeyEvent{Text: "P"})
	if a.preview.playing {
		t.Fatal("expected video to be paused")
	}
	if a.preview.loading {
		t.Fatal("pause set preview loading to true")
	}
	if title := a.preview.Title(); !strings.Contains(title, "paused") {
		t.Fatalf("paused preview title = %q, want paused", title)
	}
	if interval := a.TickInterval(); interval != 0 {
		t.Fatalf("paused tick interval = %v, want 0", interval)
	}

	// 3. Resume
	a.ConsumeKey(loom.KeyEvent{Text: "p"})
	if !a.preview.playing {
		t.Fatal("expected video to resume playing")
	}
	if a.preview.loading {
		t.Fatal("resume set preview loading to true")
	}
	if title := a.preview.Title(); !strings.Contains(title, "playing") {
		t.Fatalf("resumed preview title = %q, want playing", title)
	}
	for i := 0; i < 3; i++ {
		time.Sleep(10 * time.Millisecond)
		a.Tick(time.Now())
	}
	a.Draw(c, loom.Rect{W: 60, H: 20})
}
