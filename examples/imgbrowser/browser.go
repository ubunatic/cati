package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"codeberg.org/ubunatic/loom"
)

// imageExts is the set of file extensions recognised as images.
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".webp": true, ".tiff": true, ".tif": true,
	".avif": true, ".svg": true, ".ico": true,
}

func isImageFile(name string) bool {
	return imageExts[strings.ToLower(filepath.Ext(name))]
}

// openFile launches the system's default viewer for the target file.
func openFile(path string) error {
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

// imageDirDesc returns a space-joined list of image filenames found directly
// inside dir (one level). Used as Item.Desc so loom's text filter keeps a
// directory visible when the user searches for a filename that lives inside it.
func imageDirDesc(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "<dir>"
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && isImageFile(e.Name()) {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "<dir>"
	}
	return "<dir> " + strings.Join(names, " ")
}

type browser struct {
	frame      *loom.Frame
	list       *loom.Choice
	preview    *imagePreview
	dir        string
	paths      map[string]string
	imagesOnly bool
	filterLen  int // shadow of list.query length; 0 = filter is empty
	boxHeight  int // height of the split boxes in terminal rows
}

func newBrowser(path string) (*browser, error) {
	dir, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	b := &browser{
		preview:    newImagePreview(),
		imagesOnly: true,
		boxHeight:  20,
	}
	border := loom.BoxBorder{
		TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
		Horizontal: "─", Vertical: "│", TitlePrefix: " ", TitleSuffix: " ",
	}
	b.frame = &loom.Frame{
		Gap: 1, Breakpoint: 65,
		Status: "Tab pane  •  ↑↓ select  •  ⇧/⌃ ↑↓ rows  •  Enter open  •  m/⇧m mode  •  # info  •  +/- zoom  •  i img-only  •  F10 quit",
		Boxes: []loom.Box{
			{ID: "files", Dynamic: true, MinWidth: 24, Height: b.boxHeight, Border: border},
			{ID: "preview", Dynamic: true, MinWidth: 30, Height: b.boxHeight, Border: border, Child: b.preview},
		},
		Actions: []loom.FrameAction{
			{ID: "quit", Action: "quit", Key: "ctrl-q"},
			{ID: "quit-f10", Action: "quit", Key: "f10"},
		},
	}
	if err := b.open(dir); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *browser) ContentHeight() int {
	return b.frame.ContentHeight()
}

func (b *browser) HeightForWidth(cols int) int {
	return b.frame.HeightForWidth(cols)
}

func (b *browser) open(dir string) error {
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
			// Desc holds the directory marker plus image children so the text
			// filter keeps this entry visible when the user types a child name.
			desc := imageDirDesc(fullPath)
			items = append(items, loom.Item{Name: name, Desc: desc})
			paths[name] = fullPath
		} else {
			if b.imagesOnly && !isImageFile(name) {
				continue // skip non-images in images-only mode
			}
			items = append(items, loom.Item{Name: name})
			paths[name] = fullPath
		}
	}
	list := loom.NewChoice(items)
	list.SelectOnlyOnClick = true
	list.Prompt = "filter> "
	list.Placeholder = "type to filter"
	list.OnSelect = func(item loom.Item) {
		p := paths[item.Name]
		if info, err := os.Stat(p); err == nil {
			if info.IsDir() {
				// Capture the current dir name before open() changes b.dir.
				prevBase := filepath.Base(b.dir)
				if err := b.open(p); err != nil {
					b.preview.SetMessage("Error: " + err.Error())
				} else if item.Name == ".." {
					// Navigated up — re-select the folder we came from.
					b.selectByName(prevBase)
				}
			} else {
				// File: open using OS handler (xdg-open / open / start)
				if err := openFile(p); err != nil {
					b.preview.SetMessage("Open error: " + err.Error())
				}
			}
		}
	}
	b.dir, b.paths, b.list = dir, paths, list
	b.filterLen = 0 // filter is always empty after navigating to a new dir
	b.frame.Boxes[0].Child = list
	b.updatePreview()
	return nil
}

// updatePreview points the preview widget at the currently highlighted
// item. It's called after every key/mouse event, not just Enter, so the
// image preview tracks the cursor as it moves through the list.
func (b *browser) updatePreview() {
	item, ok := b.list.Selected()
	if !ok {
		b.preview.SetMessage("No matching file")
		return
	}
	path := b.paths[item.Name]
	info, err := os.Stat(path)
	if err != nil {
		b.preview.SetMessage("Error: " + err.Error())
		return
	}
	if info.IsDir() {
		b.preview.SetMessage("<dir> " + item.Name)
		return
	}
	b.preview.SetPath(path)
}

func (b *browser) Draw(c *loom.Canvas, r loom.Rect) {
	b.frame.Title = "Browse " + b.dir
	title := "* Files"
	if b.imagesOnly {
		title = "▣ Images"
	}
	b.frame.Boxes[0].Title = title

	prevTitle := "Preview"
	if b.preview.showInfo {
		prevTitle = "Info"
	}
	b.frame.Boxes[1].Title = prevTitle

	if focused := b.frame.FocusedBox(); focused != nil {
		if focused.ID == "files" {
			b.frame.Boxes[0].Title = "▶ " + title
		} else {
			b.frame.Boxes[1].Title = "▶ " + prevTitle
		}
	}
	b.frame.Draw(c, r)
}

func (b *browser) HandleKey(k loom.KeyEvent) bool {
	// Explicit quit keys — pane.DisableDefaultQuit suppresses pane-level handling.
	if k.Key == "ctrl-c" || k.Key == "ctrl-d" || k.Key == "f10" || k.Key == "F10" {
		return true
	}

	// Shift-Up / Shift-Down and Ctrl-Up / Ctrl-Down dynamically resize the pane rows.
	switch k.Key {
	case "shift-up", "shift_up", "S-up", "ctrl-up", "ctrl_up", "C-up":
		if b.boxHeight > 4 {
			b.boxHeight--
			b.frame.Boxes[0].Height = b.boxHeight
			b.frame.Boxes[1].Height = b.boxHeight
		}
		return false
	case "shift-down", "shift_down", "S-down", "ctrl-down", "ctrl_down", "C-down":
		b.boxHeight++
		b.frame.Boxes[0].Height = b.boxHeight
		b.frame.Boxes[1].Height = b.boxHeight
		return false
	}

	// Keys that apply when the files pane is focused.
	if focused := b.frame.FocusedBox(); focused != nil && focused.ID == "files" {
		// Toggle image-only mode with 'i' when filter is empty.
		if k.Text == "i" && b.filterLen == 0 {
			b.imagesOnly = !b.imagesOnly
			_ = b.open(b.dir) // resets filterLen
			return false
		}
		// Toggle info mode with '#' when filter is empty.
		if k.Text == "#" && b.filterLen == 0 {
			b.preview.ToggleInfo()
			return false
		}
		// Cycle mode with 'm' (forward) or 'M' / Shift-M (backward) when filter is empty.
		if b.filterLen == 0 {
			switch {
			case k.Text == "m" && k.Key != "shift-m" && k.Key != "shift-M":
				b.preview.CycleMode()
				return false
			case k.Text == "M" || k.Key == "shift-m" || k.Key == "shift-M":
				b.preview.CycleModePrev()
				return false
			}
		}
		// +/-/=/0 when filter is empty: forward zoom to the preview pane
		// rather than starting a filter search.
		if b.filterLen == 0 {
			switch k.Text {
			case "+", "=", "-", "0":
				b.preview.applyZoomKey(k.Text)
				return false
			}
		}
	}

	quit := b.frame.HandleKey(k)

	// Keep filterLen in sync with Choice.query (which is unexported).
	// Choice only returns quit=true on backspace when query is already empty,
	// so that case is handled separately below.
	switch {
	case k.Key == "backspace" && !quit:
		// Choice consumed a backspace from a non-empty filter.
		if b.filterLen > 0 {
			b.filterLen--
		}
	case k.Text != "":
		// A printable character was appended to the filter.
		b.filterLen++
	}

	// Backspace on an empty filter causes Choice to signal quit — navigate
	// up to the parent directory instead of exiting the app.
	if quit && k.Key == "backspace" {
		parent := filepath.Dir(b.dir)
		if parent != b.dir {
			prev := filepath.Base(b.dir)
			if err := b.open(parent); err != nil { // resets filterLen
				b.preview.SetMessage("Error: " + err.Error())
			} else {
				b.selectByName(prev)
			}
			b.updatePreview()
			return false
		}
		// Already at filesystem root — let the quit propagate.
	}

	b.updatePreview()
	return quit
}

// selectByName moves the list cursor to the item with the given name.
// The list is freshly built (sel=0, empty filter) when this is called,
// so replaying N down-arrow events lands on the Nth item.
func (b *browser) selectByName(name string) {
	for i, item := range b.list.Items {
		if item.Name == name {
			for j := 0; j < i; j++ {
				b.list.HandleKey(loom.KeyEvent{Key: "down"})
			}
			return
		}
	}
}

func (b *browser) HandleMouse(k loom.MouseEvent) bool {
	quit := b.frame.HandleMouse(k)
	b.updatePreview()
	return quit
}
