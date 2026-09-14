package main

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/ubunatic/loom"
)

type browser struct {
	frame   *loom.Frame
	list    *loom.Choice
	preview *imagePreview
	dir     string
	paths   map[string]string
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
	b := &browser{preview: &imagePreview{}}
	border := loom.BoxBorder{
		TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
		Horizontal: "─", Vertical: "│", TitlePrefix: " ", TitleSuffix: " ",
	}
	b.frame = &loom.Frame{
		Gap: 1, Breakpoint: 65,
		Status: "Tab pane  •  ↑↓ select  •  Enter open dir  •  ^Q quit",
		Boxes: []loom.Box{
			{ID: "files", Dynamic: true, MinWidth: 24, Height: 24, Border: border},
			{ID: "preview", Dynamic: true, MinWidth: 30, Height: 24, Border: border, Child: b.preview},
		},
		Actions: []loom.FrameAction{{ID: "quit", Action: "quit", Key: "ctrl-q"}},
	}
	if err := b.open(dir); err != nil {
		return nil, err
	}
	return b, nil
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
		desc := ""
		if entry.IsDir() {
			desc = "<dir>"
		}
		items = append(items, loom.Item{Name: entry.Name(), Desc: desc})
		paths[entry.Name()] = filepath.Join(dir, entry.Name())
	}
	list := loom.NewChoice(items)
	list.SelectOnlyOnClick = true
	list.Prompt = "filter> "
	list.Placeholder = "type to filter"
	list.OnSelect = func(item loom.Item) {
		p := paths[item.Name]
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			if err := b.open(p); err != nil {
				b.preview.SetMessage("Error: " + err.Error())
			}
		}
	}
	b.dir, b.paths, b.list = dir, paths, list
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
	b.frame.Boxes[0].Title = "Files"
	b.frame.Boxes[1].Title = "Preview"
	if focused := b.frame.FocusedBox(); focused != nil {
		if focused.ID == "files" {
			b.frame.Boxes[0].Title = "▶ Files"
		} else {
			b.frame.Boxes[1].Title = "▶ Preview"
		}
	}
	b.frame.Draw(c, r)
}

func (b *browser) HandleKey(k loom.KeyEvent) bool {
	quit := b.frame.HandleKey(k)
	b.updatePreview()
	return quit
}

func (b *browser) HandleMouse(k loom.MouseEvent) bool {
	quit := b.frame.HandleMouse(k)
	b.updatePreview()
	return quit
}
