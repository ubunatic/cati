package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/media"
)

func writeFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasItem(a *app, name string) bool {
	for _, item := range a.list.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func TestIsMediaFile(t *testing.T) {
	for _, name := range []string{"photo.JPG", "image.svg", "clip.MP4", "movie.webm"} {
		if !isMediaFile(name) {
			t.Errorf("isMediaFile(%q) = false", name)
		}
	}
	for _, name := range []string{"notes.txt", "archive.zip", "README"} {
		if isMediaFile(name) {
			t.Errorf("isMediaFile(%q) = true", name)
		}
	}
}

func TestAppDirectoryNavigationAndMediaFiltering(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(root, "photo.PNG"))
	writeFixture(t, filepath.Join(root, "clip.mp4"))
	writeFixture(t, filepath.Join(root, "notes.txt"))

	a, err := newApp(root, "mc", media.ModeHalfblock, 24, false, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if !hasItem(a, "child") || !hasItem(a, "photo.PNG") || !hasItem(a, "clip.mp4") || !hasItem(a, "notes.txt") {
		t.Fatalf("unfiltered entries = %+v", a.list.Items)
	}
	if a.list.OnSelect == nil {
		t.Fatal("directory activation callback is missing")
	}
	a.list.OnSelect(loom.Item{Name: "child"})
	if a.dir != child {
		t.Fatalf("navigated directory = %q, want %q", a.dir, child)
	}
	if !hasItem(a, "..") {
		t.Fatal("nested directory list does not contain parent entry")
	}
	a.list.OnSelect(loom.Item{Name: ".."})
	if a.dir != root {
		t.Fatalf("parent navigation directory = %q, want %q", a.dir, root)
	}

	a.imagesOnly = true
	if err := a.open(root); err != nil {
		t.Fatal(err)
	}
	if !hasItem(a, "child") || !hasItem(a, "photo.PNG") || !hasItem(a, "clip.mp4") || hasItem(a, "notes.txt") {
		t.Fatalf("media-filtered entries = %+v", a.list.Items)
	}
}

func TestThemeResolutionCyclingAndFullscreenKeys(t *testing.T) {
	a, err := newApp(t.TempDir(), "mc", media.ModeHalfblock, 24, false, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.themeName != "mc" || a.theme == (loom.ThemeColors{}) {
		t.Fatalf("known theme was not resolved: name=%q theme=%+v", a.themeName, a.theme)
	}
	if quit := a.HandleKey(loom.KeyEvent{Key: "f9"}); quit {
		t.Fatal("F9 unexpectedly quit")
	}
	if a.themeName != "solarized-dark" {
		t.Fatalf("theme after F9 = %q", a.themeName)
	}
	a.themeName = "not-a-theme"
	a.cycleTheme()
	if a.themeName != "mc" {
		t.Fatalf("unknown theme cycle resolved to %q, want mc", a.themeName)
	}
	if quit := a.HandleKey(loom.KeyEvent{Key: "f"}); quit {
		t.Fatal("fullscreen key unexpectedly quit")
	}
	if !a.fullscreen || !a.frame.Boxes[0].Hidden || !strings.Contains(a.frame.Status, "[f] Split") {
		t.Fatalf("fullscreen state not applied: fullscreen=%v hidden=%v status=%q", a.fullscreen, a.frame.Boxes[0].Hidden, a.frame.Status)
	}
	a.HandleKey(loom.KeyEvent{Key: "f"})
	if a.fullscreen || a.frame.Boxes[0].Hidden || !strings.Contains(a.frame.Status, "[f] Full") {
		t.Fatalf("split state not restored: fullscreen=%v hidden=%v status=%q", a.fullscreen, a.frame.Boxes[0].Hidden, a.frame.Status)
	}

	a.imagesOnly = true
	if quit := a.HandleKey(loom.KeyEvent{Text: "i"}); quit {
		t.Fatal("media-filter key unexpectedly quit")
	}
	if a.imagesOnly {
		t.Fatal("i key should toggle media-only filtering off")
	}
}
