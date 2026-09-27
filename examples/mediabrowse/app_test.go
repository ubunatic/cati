package main

import (
	"os"
	"path/filepath"
	"sort"
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
	if a.themeName != "mc" || a.theme != loom.SpeccedThemes["mc"] {
		t.Fatalf("known theme was not resolved: name=%q theme=%+v", a.themeName, a.theme)
	}
	names := themeNames()
	if !sort.StringsAreSorted(names) || len(names) != len(loom.SpeccedThemes) {
		t.Fatalf("theme names are not a sorted list of Loom themes: %v", names)
	}
	if quit := a.HandleKey(loom.KeyEvent{Key: "f9"}); quit {
		t.Fatal("F9 unexpectedly quit")
	}
	mcIndex := sort.SearchStrings(names, "mc")
	wantAfterMC := names[(mcIndex+1)%len(names)]
	if a.themeName != wantAfterMC || a.theme != loom.SpeccedThemes[wantAfterMC] {
		t.Fatalf("theme after F9 = %q, want %q", a.themeName, wantAfterMC)
	}
	a.themeName = "not-a-theme"
	a.cycleTheme()
	if a.themeName != "mc" {
		t.Fatalf("unknown theme cycle resolved to %q, want mc", a.themeName)
	}
	if a.theme != loom.SpeccedThemes["mc"] {
		t.Fatalf("unknown theme cycle applied colors for %q", a.themeName)
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

func TestUnknownThemeFallsBackToKnownTheme(t *testing.T) {
	name, theme := resolveTheme("not-a-theme")
	if name != "mc" || theme != loom.SpeccedThemes["mc"] {
		t.Fatalf("resolveTheme() = %q/%+v, want mc/%+v", name, theme, loom.SpeccedThemes["mc"])
	}
	if _, ok := loom.SpeccedThemes[name]; !ok {
		t.Fatalf("fallback theme %q is not Loom-defined", name)
	}
}

func TestCycleThemeVisitsEverySpeccedThemeAndWraps(t *testing.T) {
	names := themeNames()
	if len(names) == 0 {
		t.Fatal("Loom has no specced themes")
	}
	a, err := newApp(t.TempDir(), names[0], media.ModeHalfblock, 24, false, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for i := 1; i <= len(names); i++ {
		a.cycleTheme()
		want := names[i%len(names)]
		if a.themeName != want || a.theme != loom.SpeccedThemes[want] {
			t.Fatalf("cycle %d = %q, want %q", i, a.themeName, want)
		}
	}
}
