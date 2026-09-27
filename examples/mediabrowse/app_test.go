package main

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/media"

	"ubunatic.com/cati/v1/core"
)

func writeFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasItem(a *app, name string) bool {
	for _, item := range a.navigation.List().Items {
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
		t.Fatalf("unfiltered entries = %+v", a.navigation.List().Items)
	}
	if a.navigation.List().OnSelect == nil {
		t.Fatal("directory activation callback is missing")
	}
	a.navigation.List().OnSelect(loom.Item{Name: "child"})
	if a.dir != child {
		t.Fatalf("navigated directory = %q, want %q", a.dir, child)
	}
	if !hasItem(a, "..") {
		t.Fatal("nested directory list does not contain parent entry")
	}
	a.navigation.List().OnSelect(loom.Item{Name: ".."})
	if a.dir != root {
		t.Fatalf("parent navigation directory = %q, want %q", a.dir, root)
	}

	a.imagesOnly = true
	a.filterMediaItems()
	if !hasItem(a, "child") || !hasItem(a, "photo.PNG") || !hasItem(a, "clip.mp4") || hasItem(a, "notes.txt") {
		t.Fatalf("media-filtered entries = %+v", a.navigation.List().Items)
	}
	a.imagesOnly = false
	a.filterMediaItems()
	if !hasItem(a, "notes.txt") {
		t.Fatal("disabling media-only filter did not restore non-media entries")
	}
}

func TestNavigationPaneUsesSlashGatedSearchAndDrivesPreview(t *testing.T) {
	root := t.TempDir()
	imagePath := filepath.Join(root, "photo.png")
	writeFixture(t, imagePath)
	writeFixture(t, filepath.Join(root, "notes.txt"))
	a, err := newApp(root, "mc", media.ModeHalfblock, 24, false, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.navigation == nil || a.frame.Boxes[0].Child != a.navigation {
		t.Fatal("frame is not hosting Loom NavigationPane")
	}
	if a.navigation.List().Query() != "" {
		t.Fatalf("initial search query = %q", a.navigation.List().Query())
	}
	if a.navigation.List().OnSelect == nil {
		t.Fatal("navigation pane is missing its file activation callback")
	}
	a.navigation.HandleKey(loom.KeyEvent{Text: "p"})
	if a.navigation.List().Query() != "" {
		t.Fatalf("typed app key started search: %q", a.navigation.List().Query())
	}
	a.navigation.HandleKey(loom.KeyEvent{Key: "/", Text: "/"})
	if !a.navigation.Searching() {
		t.Fatal("slash did not enter search mode")
	}
	a.navigation.HandleKey(loom.KeyEvent{Text: "photo"})
	if a.navigation.List().Query() != "photo" {
		t.Fatalf("search query = %q, want photo", a.navigation.List().Query())
	}
	a.navigation.HandleKey(loom.KeyEvent{Key: "esc"})
	if a.navigation.Searching() || a.navigation.List().Query() != "" {
		t.Fatal("escape did not clear and close gated search")
	}

	a.selectByName("notes.txt")
	a.navigation.HandleKey(loom.KeyEvent{Key: "down"})
	if a.preview.currentPath != imagePath {
		t.Fatalf("preview path = %q, want %q", a.preview.currentPath, imagePath)
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
	// Test fullscreen with Text: "f" (standard terminal key event)
	if quit := a.HandleKey(loom.KeyEvent{Text: "f"}); quit {
		t.Fatal("fullscreen text key unexpectedly quit")
	}
	if !a.fullscreen || !a.frame.Boxes[0].Hidden || !strings.Contains(a.frame.Status, "[f] Split") {
		t.Fatalf("fullscreen state not applied: fullscreen=%v hidden=%v status=%q", a.fullscreen, a.frame.Boxes[0].Hidden, a.frame.Status)
	}
	a.HandleKey(loom.KeyEvent{Text: "f"})
	if a.fullscreen || a.frame.Boxes[0].Hidden || !strings.Contains(a.frame.Status, "[f] Full") {
		t.Fatalf("split state not restored: fullscreen=%v hidden=%v status=%q", a.fullscreen, a.frame.Boxes[0].Hidden, a.frame.Status)
	}

	if quit := a.HandleKey(loom.KeyEvent{Text: "i"}); quit {
		t.Fatal("media-filter key unexpectedly quit")
	}
	if !a.imagesOnly {
		t.Fatal("i key should toggle media-only filtering on")
	}
}

func TestAppKeyRoutingAcrossPanesAndSearch(t *testing.T) {
	withWidgetFactories(t)
	loadImageWidget = func(_ context.Context, _ string, mode media.Mode, _ func(core.Progress)) (*media.Widget, error) {
		return media.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)), mode)
	}
	newVideoWidget = func(_ string, mode media.Mode, _ float64) (*media.Widget, error) {
		return media.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)), mode)
	}

	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "clip.mp4"))
	a, err := newApp(root, "mc", media.ModeHalfblock, 24, false, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	// Initial mode is Halfblock
	if a.preview.mode != media.ModeHalfblock {
		t.Fatalf("initial mode = %q", a.preview.mode)
	}

	// 'm' toggles mode
	a.HandleKey(loom.KeyEvent{Text: "m"})
	if a.preview.mode != media.ModeQuadblock {
		t.Fatalf("mode after 'm' = %q, want quadblock", a.preview.mode)
	}

	// 'p' toggles play on selected video
	a.HandleKey(loom.KeyEvent{Text: "p"})
	if !a.preview.playing {
		t.Fatal("expected playing to be true after 'p'")
	}
	a.HandleKey(loom.KeyEvent{Text: "p"})
	if a.preview.playing {
		t.Fatal("expected playing to be false after second 'p'")
	}

	// Switch focus to preview pane (Tab)
	a.frame.FocusNext()
	if focused := a.frame.FocusedBox(); focused == nil || focused.ID != "preview" {
		t.Fatalf("focused box = %v, want preview", focused)
	}

	// 'p' and 'm' and 'f' still work when preview is focused
	a.HandleKey(loom.KeyEvent{Text: "p"})
	if !a.preview.playing {
		t.Fatal("expected playing to work while preview is focused")
	}
	a.HandleKey(loom.KeyEvent{Text: "f"})
	if !a.fullscreen {
		t.Fatal("expected fullscreen to toggle while preview is focused")
	}
	a.HandleKey(loom.KeyEvent{Text: "f"})
	if a.fullscreen {
		t.Fatal("expected fullscreen to restore while preview is focused")
	}

	// Enter search mode and verify 'p', 'm', 'f', 'i' go to search query instead of triggering actions
	a.HandleKey(loom.KeyEvent{Key: "tab"})
	if focused := a.frame.FocusedBox(); focused == nil || focused.ID != "files" {
		t.Fatalf("focused box after tab = %v, want files", focused)
	}
	a.HandleKey(loom.KeyEvent{Text: "/"})
	if !a.navigation.Searching() {
		t.Fatal("expected navigation to be in search mode")
	}
	a.HandleKey(loom.KeyEvent{Text: "p"})
	if a.navigation.List().Query() != "p" {
		t.Fatalf("search query = %q, want p", a.navigation.List().Query())
	}
	// Play state should not have changed during search typing
	if !a.preview.playing {
		t.Fatal("search typing unexpectedly modified play state")
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
