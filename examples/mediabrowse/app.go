package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"codeberg.org/ubunatic/loom"
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
	list       *loom.Choice
	preview    *mediaPreviewPane
	dir        string
	paths      map[string]string
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
	theme := loom.Theme(themeName)
	if theme == (loom.ThemeColors{}) {
		themeName = "mc"
		theme = loom.Theme(themeName)
	}

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
	if a.list != nil {
		a.list.ApplyTheme(theme)
	}
}

func (a *app) cycleTheme() {
	themes := []string{"mc", "solarized-dark", "solarized-light", "monokai", "dracula", "nord", "gruvbox"}
	for i, name := range themes {
		if name == a.themeName {
			next := themes[(i+1)%len(themes)]
			a.applyTheme(next, loom.Theme(next))
			return
		}
	}
	a.applyTheme("mc", loom.Theme("mc"))
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	items := make([]loom.Item, 0, len(entries)+1)
	paths := make(map[string]string, len(entries)+1)

	if parent := filepath.Dir(dir); parent != dir {
		items = append(items, loom.Item{Name: "..", Desc: "parent directory"})
		paths[".."] = parent
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)
		if entry.IsDir() {
			items = append(items, loom.Item{Name: name, Desc: "<dir>"})
			paths[name] = fullPath
		} else {
			if a.imagesOnly && !isMediaFile(name) {
				continue
			}
			desc := ""
			if isMediaFile(name) {
				if halfblock.IsVideo(name) {
					desc = "video"
				} else {
					desc = "image"
				}
			}
			items = append(items, loom.Item{Name: name, Desc: desc})
			paths[name] = fullPath
		}
	}

	list := loom.NewChoice(items)
	list.SelectOnlyOnClick = true
	list.DoubleClickToActivate = true
	list.Prompt = "filter> "
	list.Placeholder = "[/] search"
	list.ApplyTheme(a.theme)

	list.OnSelect = func(item loom.Item) {
		p := paths[item.Name]
		if info, err := os.Stat(p); err == nil {
			if info.IsDir() {
				prevBase := filepath.Base(a.dir)
				if err := a.open(p); err == nil && item.Name == ".." {
					a.selectByName(prevBase)
				}
			} else {
				_ = openSystemViewer(p)
			}
		}
	}

	a.dir, a.paths, a.list = dir, paths, list
	a.frame.Boxes[0].Child = list

	if len(initialSelection) > 0 && initialSelection[0] != "" {
		a.selectByName(initialSelection[0])
	}

	a.updatePreview()
	return nil
}

func (a *app) selectByName(name string) {
	for i, item := range a.list.Items {
		if item.Name == name {
			for j := 0; j < i; j++ {
				a.list.HandleKey(loom.KeyEvent{Key: "down"})
			}
			return
		}
	}
}

func (a *app) updatePreview() {
	item, ok := a.list.Selected()
	if !ok {
		a.preview.SetMessage("No selection")
		return
	}
	path := a.paths[item.Name]
	info, err := os.Stat(path)
	if err != nil {
		a.preview.SetMessage("Error: " + err.Error())
		return
	}
	if info.IsDir() {
		a.preview.SetMessage("<dir> " + item.Name)
		return
	}
	if !isMediaFile(path) {
		a.preview.SetMessage(fmt.Sprintf("%s (%d bytes)", item.Name, info.Size()))
		return
	}
	a.preview.SetPath(path)
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
	switch k.Key {
	case "f9":
		a.cycleTheme()
		return false
	case "f", "F":
		a.toggleFullscreen()
		return false
	}

	filesFocused := true
	if focused := a.frame.FocusedBox(); focused != nil {
		filesFocused = (focused.ID == "files")
	}

	if filesFocused {
		switch k.Text {
		case "i":
			a.imagesOnly = !a.imagesOnly
			_ = a.open(a.dir)
			return false
		case "m":
			a.preview.CycleMode()
			return false
		case "p", "P":
			a.preview.TogglePlay()
			return false
		}
	}

	quit := a.frame.HandleKey(k)
	if quit && k.Key == "backspace" {
		parent := filepath.Dir(a.dir)
		if parent != a.dir {
			prev := filepath.Base(a.dir)
			if err := a.open(parent); err == nil {
				a.selectByName(prev)
			}
			a.updatePreview()
			return false
		}
	}

	a.updatePreview()
	return quit
}

func (a *app) HandleMouse(m loom.MouseEvent) bool {
	quit := a.frame.HandleMouse(m)
	a.updatePreview()
	return quit
}
