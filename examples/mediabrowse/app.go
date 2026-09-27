package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/examples/filebrowser/filebrowser"
	"codeberg.org/ubunatic/loom/media"

	"ubunatic.com/cati/v1/halfblock"
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".webp": true, ".tiff": true, ".tif": true,
	".avif": true, ".svg": true, ".ico": true,
}

func isMediaFile(name string) bool {
	return imageExts[strings.ToLower(filepath.Ext(name))] || halfblock.IsVideo(name)
}

func openSystemViewer(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

type app struct {
	frame      *loom.Frame
	navigation *filebrowser.NavigationPane
	preview    *mediaPreviewPane
	dir        string
	themeName  string
	theme      loom.ThemeColors
	imagesOnly bool
	fullscreen bool
	boxHeight  int
	fps        float64
}

func newApp(dir, themeName string, initialMode media.Mode, fps float64, imagesOnly bool, initialHeight int, initialSelection ...string) (*app, error) {
	if initialHeight < 10 {
		initialHeight = 30
	}
	themeName, theme := resolveTheme(themeName)

	a := &app{
		themeName:  themeName,
		theme:      theme,
		imagesOnly: imagesOnly,
		boxHeight:  initialHeight,
		fps:        fps,
		preview:    newMediaPreviewPane(initialMode, fps),
	}

	border := loom.BoxBorder{
		TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
		Horizontal: "─", Vertical: "│", TitlePrefix: " ", TitleSuffix: " ",
	}

	a.frame = &loom.Frame{
		Gap: 1, Breakpoint: 65,
		Status: "[Tab] Pane  [↑↓] Move  [↵] View  [/] Search  [m] Mode  [p/P] Play/Pause  [f] Full  [i] Media  [F9] Theme  [F10/q] Quit",
		Boxes: []loom.Box{
			{ID: "files", Dynamic: true, MinWidth: 24, Height: a.boxHeight, Border: border},
			{ID: "preview", Dynamic: true, MinWidth: 32, Height: a.boxHeight, Border: border, Child: a.preview},
		},
		Actions: []loom.FrameAction{
			{ID: "quit-q", Action: "quit", Key: "q"},
			{ID: "quit-ctrl-q", Action: "quit", Key: "ctrl-q"},
			{ID: "quit-f10", Action: "quit", Key: "f10"},
		},
	}

	a.applyTheme(a.themeName, a.theme)

	selectName := ""
	if len(initialSelection) > 0 {
		selectName = initialSelection[0]
	}

	if err := a.open(dir, selectName); err != nil {
		return nil, err
	}
	return a, nil
}

func themeNames() []string {
	return loom.ThemeNames()
}

func resolveTheme(name string) (string, loom.ThemeColors) {
	if loom.ThemeExists(name) {
		return name, loom.Theme(name)
	}
	if loom.ThemeExists("mc") {
		return "mc", loom.Theme("mc")
	}
	return "plain", loom.Theme("plain")
}

func (a *app) Close() {
	if a.preview != nil {
		a.preview.Close()
	}
}

func (a *app) applyTheme(name string, theme loom.ThemeColors) {
	a.themeName, a.theme = name, theme
	a.frame.Style = theme.FrameStyle()
	for i := range a.frame.Boxes {
		a.frame.Boxes[i].Style = theme.BoxStyle()
	}
	if a.navigation != nil {
		a.navigation.ApplyTheme(theme)
	}
}

func (a *app) cycleTheme() {
	themes := themeNames()
	if len(themes) == 0 {
		return
	}
	for i, name := range themes {
		if name == a.themeName {
			next := themes[(i+1)%len(themes)]
			a.applyTheme(next, loom.SpeccedThemes[next])
			return
		}
	}
	name, theme := resolveTheme("")
	a.applyTheme(name, theme)
}

func (a *app) toggleFullscreen() {
	if box := a.frame.Box("files"); box != nil {
		box.Hidden = !box.Hidden
		a.fullscreen = box.Hidden
	}
	if a.fullscreen {
		a.frame.Status = "[f] Split  [m] Mode  [p/P] Play/Pause  [F9] Theme  [F10/q] Quit"
	} else {
		a.frame.Status = "[Tab] Pane  [↑↓] Move  [↵] View  [/] Search  [m] Mode  [p/P] Play/Pause  [f] Full  [i] Media  [F9] Theme  [F10/q] Quit"
	}
}

func (a *app) open(dir string, initialSelection ...string) error {
	selectName := ""
	if len(initialSelection) > 0 {
		selectName = initialSelection[0]
	}
	nav, err := filebrowser.NewNavigationPane(dir, filebrowser.NavigationPaneOptions{
		OnSelection: func(entry loom.FileEntry) {
			if a.navigation != nil {
				a.showEntry(entry)
			}
		},
		OnActivate: func(entry loom.FileEntry) { _ = openSystemViewer(entry.Path) },
		OnOpen: func(directory loom.Directory) {
			a.dir = directory.Path
			if a.imagesOnly {
				a.filterMediaItems()
			}
		},
	})
	if err != nil {
		return err
	}
	a.navigation = nav
	a.dir = nav.Directory().Path
	nav.List().ApplyTheme(a.theme)
	a.frame.Boxes[0].Child = nav
	if a.imagesOnly {
		a.filterMediaItems()
	}
	if selectName != "" {
		a.selectByName(selectName)
	}
	a.updatePreview()
	return nil
}

func (a *app) filterMediaItems() {
	if a.navigation == nil {
		return
	}
	directory := a.navigation.Directory()
	selected, hadSelection := a.navigation.Selected()
	items := make([]loom.Item, 0, len(directory.Entries))
	for _, entry := range directory.Entries {
		if a.imagesOnly && entry.Kind != loom.FileKindDirectory && !isMediaFile(entry.Name) {
			continue
		}
		desc := ""
		if entry.IsParent {
			desc = "parent directory"
		} else if entry.Kind == loom.FileKindDirectory {
			desc = "<dir>"
		} else if isMediaFile(entry.Name) && halfblock.IsVideo(entry.Name) {
			desc = "video"
		} else if isMediaFile(entry.Name) {
			desc = "image"
		}
		items = append(items, loom.Item{Name: entry.DisplayName(), Desc: desc})
	}
	a.navigation.List().SetItems(items)
	if hadSelection {
		for i, item := range items {
			if item.Name == selected.DisplayName() {
				a.navigation.List().SelectIndex(i)
				break
			}
		}
	}
}

func (a *app) showEntry(entry loom.FileEntry) {
	if a.navigation != nil {
		a.dir = a.navigation.Directory().Path
	}
	if entry.Kind == loom.FileKindDirectory {
		a.preview.SetMessage("<dir> " + entry.DisplayName())
		return
	}
	if !isMediaFile(entry.Path) {
		info, err := os.Stat(entry.Path)
		if err != nil {
			a.preview.SetMessage("Error: " + err.Error())
			return
		}
		a.preview.SetMessage(fmt.Sprintf("%s (%d bytes)", entry.DisplayName(), info.Size()))
		return
	}
	a.preview.SetPath(entry.Path)
}

func (a *app) selectByName(name string) {
	if a.navigation == nil {
		return
	}
	for i, item := range a.navigation.List().Items {
		if item.Name == name {
			a.navigation.List().SelectIndex(i)
			if entry, ok := a.navigation.Selected(); ok {
				a.showEntry(entry)
			}
			return
		}
	}
}

func (a *app) updatePreview() {
	if a.navigation == nil {
		a.preview.SetMessage("No selection")
		return
	}
	entry, ok := a.navigation.Selected()
	if !ok {
		a.preview.SetMessage("No selection")
		return
	}
	a.showEntry(entry)
}

func (a *app) Draw(c *loom.Canvas, r loom.Rect) {
	title := "Files"
	if a.imagesOnly {
		title = "▣ Media Files"
	}
	a.frame.Title = fmt.Sprintf("Browse %s  [theme: %s]", a.dir, a.themeName)
	a.frame.Boxes[0].Title = title
	a.frame.Boxes[1].Title = a.preview.Title()

	if focused := a.frame.FocusedBox(); focused != nil {
		if focused.ID == "files" {
			a.frame.Boxes[0].Title = "▶ " + title
		} else {
			a.frame.Boxes[1].Title = "▶ " + a.preview.Title()
		}
	}
	a.frame.Draw(c, r)
}

func (a *app) HandleKey(k loom.KeyEvent) bool {
	if a.navigation != nil && a.navigation.Searching() {
		quit := a.frame.HandleKey(k)
		a.updatePreview()
		return quit
	}

	if k.Is("f9") {
		a.cycleTheme()
		return false
	}
	if k.Is("f") {
		a.toggleFullscreen()
		return false
	}
	if k.Is("i") {
		a.imagesOnly = !a.imagesOnly
		if a.navigation != nil {
			a.filterMediaItems()
			if entry, ok := a.navigation.Selected(); ok {
				a.showEntry(entry)
			} else {
				a.preview.SetMessage("No selection")
			}
		}
		return false
	}
	if k.Is("m") {
		a.preview.CycleMode()
		return false
	}
	if k.Is("p") {
		a.preview.TogglePlay()
		return false
	}

	quit := a.frame.HandleKey(k)
	a.updatePreview()
	return quit
}

func (a *app) HandleMouse(m loom.MouseEvent) bool {
	quit := a.frame.HandleMouse(m)
	a.updatePreview()
	return quit
}

func (a *app) Tick(now time.Time) {
	if a.preview != nil {
		a.preview.Tick(now)
	}
}

func (a *app) TickInterval() time.Duration {
	if a.preview != nil {
		return a.preview.TickInterval()
	}
	return 0
}
