package cmd

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"golang.org/x/term"
)

type menuButton struct {
	label  string
	col    int // 1-indexed column
	width  int // character width
	action string // e.g. "prev", "next", "settings", "about", "back", "quit", "inc", "dec", "save", "cancel"
}

type browserItem struct {
	path  string
	isDir bool
	name  string
}

// ── Custom folder pixel icon ──────────────────────────────────────────────────

func createFolderIcon() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 16, 12))
	folderColor := color.RGBA{R: 245, G: 158, B: 11, A: 255} // Amber/yellow
	tabColor := color.RGBA{R: 217, G: 119, B: 6, A: 255}    // Darker amber for tab
	borderColor := color.RGBA{R: 146, G: 64, B: 14, A: 255} // Dark brown outline

	for y := 0; y < 12; y++ {
		for x := 0; x < 16; x++ {
			// Tab border & fill
			if y >= 1 && y <= 2 && x >= 1 && x <= 5 {
				if y == 1 || x == 1 || x == 5 {
					img.Set(x, y, borderColor)
				} else {
					img.Set(x, y, tabColor)
				}
			}
			// Body border & fill
			if y >= 3 && y <= 10 && x >= 1 && x <= 14 {
				if y == 3 || y == 10 || x == 1 || x == 14 {
					img.Set(x, y, borderColor)
				} else {
					img.Set(x, y, folderColor)
				}
			}
		}
	}
	return img
}

// ── Customizable Labels ──────────────────────────────────────────────────────

func loadLabels() map[string]string {
	labels := map[string]string{
		"prev":     "[◀ Prev]",
		"next":     "[Next ▶]",
		"settings": "[⚙ Settings]",
		"about":    "[ℹ About]",
		"quit":     "[✖ Quit]",
		"back":     "[◀ Back]",
		"inc":      "[▲ Increase]",
		"dec":      "[▼ Decrease]",
		"save":     "[✔ Save]",
		"cancel":   "[✖ Cancel]",
	}

	data, err := os.ReadFile("spec/labels.yaml")
	if err != nil {
		return labels // Fallback to default labels
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			if val != "" {
				labels[key] = val
			}
		}
	}
	return labels
}

// ── YAML view parser ─────────────────────────────────────────────────────────

type YamlView struct {
	Type     string
	Name     string
	Title    string
	Content  string
	Controls []string
}

func parseYamlView(path string) (*YamlView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	view := &YamlView{}
	lines := strings.Split(string(data), "\n")

	inContent := false
	inControls := false

	for _, line := range lines {
		if inContent {
			if line == "" {
				view.Content += "\n"
				continue
			}
			if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
				view.Content += strings.TrimPrefix(strings.TrimPrefix(line, "  "), "\t") + "\n"
				continue
			} else {
				inContent = false
			}
		}

		if inControls {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				view.Controls = append(view.Controls, strings.TrimPrefix(trimmed, "- "))
				continue
			} else if line != "" && !strings.HasPrefix(line, " ") {
				inControls = false
			}
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "type:") {
			view.Type = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
		} else if strings.HasPrefix(line, "name:") {
			view.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		} else if strings.HasPrefix(line, "title:") {
			view.Title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
		} else if strings.HasPrefix(line, "content: |") {
			inContent = true
		} else if strings.HasPrefix(line, "controls:") {
			inControls = true
		}
	}

	return view, nil
}

func getAboutView() *YamlView {
	view, err := parseYamlView("spec/about.yaml")
	if err == nil && view != nil {
		return view
	}
	// Fallback
	return &YamlView{
		Type:  "view",
		Name:  "about",
		Title: "Cati — cat for images & video in terminal",
		Content: `Version: 1.0.0
License: AGPL-3.0-or-later
Authors: Uwe Jugel (codeberg.org/ubunatic/cati)

Controls (Grid Preview):
  • Left/Right/Up/Down Arrow: Move selection
  • PageUp/PageDown, [, ]: Navigate pages
  • Mouse wheel: Scroll pages
  • Click thumbnail / Enter / Space: View full screen
  • a / A: Toggle About page
  • s / S: Settings dialog
  • q / Esc: Quit application

Controls (Interactive Single View):
  • + / -: Zoom in / zoom out (centred on screen)
  • Mouse wheel: Zoom in / zoom out at cursor position
  • Left-click drag: Pan (grab-and-pull the image)
  • Up/Down/Left/Right Arrows: Pan the image
  • c / C: Copy current viewport to clipboard (PNG)
  • q / Esc: Go back to Grid view`,
		Controls: []string{"back", "quit", "website"},
	}
}

// ── Config loader & saver ───────────────────────────────────────────────────

func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "cati")
}

func loadSettings() int {
	dir := getConfigDir()
	if dir == "" {
		return 20
	}
	path := filepath.Join(dir, "config")
	data, err := os.ReadFile(path)
	if err != nil {
		return 20
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "height" {
			h, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err == nil && h > 0 {
				return h
			}
		}
	}
	return 20
}

func saveSettings(height int) error {
	dir := getConfigDir()
	if dir == "" {
		return fmt.Errorf("could not determine home dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "config")
	content := fmt.Sprintf("height=%d\n", height)
	return os.WriteFile(path, []byte(content), 0o644)
}

// ── Dynamic Directory Loading ───────────────────────────────────────────────

func loadBrowserItems(currentDir string, initialItems []browserItem) []browserItem {
	if currentDir == "" {
		return initialItems
	}

	var out []browserItem
	// Parent folder entry
	out = append(out, browserItem{
		path:  filepath.Dir(currentDir),
		isDir: true,
		name:  "..",
	})

	files, err := os.ReadDir(currentDir)
	if err != nil {
		return out
	}

	var dirs []browserItem
	var imgs []browserItem

	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, ".") {
			continue // Skip hidden files
		}
		path := filepath.Join(currentDir, name)
		if f.IsDir() {
			dirs = append(dirs, browserItem{
				path:  path,
				isDir: true,
				name:  name,
			})
		} else if isImageFile(path) {
			imgs = append(imgs, browserItem{
				path:  path,
				isDir: false,
				name:  name,
			})
		}
	}

	out = append(out, dirs...)
	out = append(out, imgs...)
	return out
}

// ── Browser ──────────────────────────────────────────────────────────────────

// browser runs an interactive grid preview of files and subdirectories.
func browser(args []string, initWidth, initHeight int) error {
	cfgHeight := loadSettings()
	tempHeight := cfgHeight

	var initialItems []browserItem
	for _, p := range args {
		info, err := os.Stat(p)
		if err == nil {
			initialItems = append(initialItems, browserItem{
				path:  p,
				isDir: info.IsDir(),
				name:  filepath.Base(p),
			})
		}
	}

	currentDir := ""
	if len(args) == 1 {
		info, err := os.Stat(args[0])
		if err == nil && info.IsDir() {
			currentDir = args[0]
		}
	}

	items := loadBrowserItems(currentDir, initialItems)

	type thumbKey struct {
		path string
		w, h int
	}
	thumbCache := make(map[thumbKey]image.Image)

	getThumbnail := func(item browserItem, cellW, cellH int) (image.Image, error) {
		key := thumbKey{path: item.path, w: cellW, h: cellH}
		if img, ok := thumbCache[key]; ok {
			return img, nil
		}
		if item.isDir {
			img := createFolderIcon()
			scaled := halfblock.ScaleToFit(img, cellW, cellH)
			thumbCache[key] = scaled
			return scaled, nil
		}
		img, err := halfblock.LoadImage(item.path)
		if err != nil {
			return nil, err
		}
		scaled := halfblock.ScaleToFit(img, cellW, cellH)
		thumbCache[key] = scaled
		return scaled, nil
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		oldState = nil
	}
	defer func() {
		if oldState != nil {
			_ = term.Restore(fd, oldState)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	inputs := make(chan string, 32)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			tokens := tokenizeInput(string(buf[:n]))
			for _, tok := range tokens {
				inputs <- tok
			}
		}
	}()

	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	// Enable mouse reporting with any-event tracking (\x1b[?1003h) to capture hover/motion
	fmt.Fprint(os.Stdout, "\x1b[?1003h\x1b[?1006h")
	defer func() {
		fmt.Fprint(os.Stdout, "\x1b[?1003l\x1b[?1006l")
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	selectedIdx := 0
	viewMode := "grid" // "grid", "about", or "settings"
	var buttons []menuButton
	hoveredButtonAction := ""

	termCols, termRows := resolveTermSize(initWidth, initHeight)

	redraw := func() {
		termCols, termRows = resolveTermSize(initWidth, initHeight)

		effHeight := termRows
		if cfgHeight > 0 && cfgHeight < termRows {
			effHeight = cfgHeight
		}

		if viewMode == "about" {
			drawAboutPage(os.Stdout, termCols, effHeight)
			buttons = drawBottomMenu(os.Stdout, termCols, effHeight, 0, 0, "about", hoveredButtonAction)
			return
		}

		if viewMode == "settings" {
			drawSettingsPage(os.Stdout, termCols, effHeight, tempHeight)
			buttons = drawBottomMenu(os.Stdout, termCols, effHeight, 0, 0, "settings", hoveredButtonAction)
			return
		}

		// Grid view
		marginTop := 1
		marginBottom := 3
		gridRowsLimit := effHeight - marginTop - marginBottom

		if gridRowsLimit <= 0 {
			halfblock.ClearScreen(os.Stdout)
			fmt.Fprintf(os.Stdout, "\x1b[1;1HTerminal size too small.")
			return
		}

		gridCols := 3
		gridRows := 2
		if termCols < 60 {
			gridCols = 2
		}
		if termCols < 40 {
			gridCols = 1
		}
		if effHeight < 14 {
			gridRows = 1
		}

		itemsPerPage := gridCols * gridRows
		numPages := (len(items) + itemsPerPage - 1) / itemsPerPage

		if selectedIdx < 0 {
			selectedIdx = 0
		}
		if selectedIdx >= len(items) {
			selectedIdx = len(items) - 1
		}
		pageIdx := selectedIdx / itemsPerPage
		startIdx := pageIdx * itemsPerPage
		endIdx := startIdx + itemsPerPage
		if endIdx > len(items) {
			endIdx = len(items)
		}

		gapX := 4
		gapY := 2
		cellW := (termCols - (gridCols-1)*gapX) / gridCols
		cellH := (gridRowsLimit - (gridRows-1)*gapY) / gridRows
		if cellW < 10 {
			cellW = 10
		}
		if cellH < 4 {
			cellH = 4
		}

		compImg := image.NewRGBA(image.Rect(0, 0, termCols, gridRowsLimit*2))

		// Render thumbnails
		for idx := startIdx; idx < endIdx; idx++ {
			cellItemIdx := idx - startIdx
			colIdx := cellItemIdx % gridCols
			rowIdx := cellItemIdx / gridCols
			left := colIdx * (cellW + gapX)
			top := rowIdx * (cellH + gapY)

			thumb, err := getThumbnail(items[idx], cellW, cellH-1)
			if err == nil && thumb != nil {
				thumbW := thumb.Bounds().Dx()
				thumbH := thumb.Bounds().Dy()
				offsetX := (cellW - thumbW) / 2
				offsetY := ((cellH-1)*2 - thumbH) / 2
				destX := left + offsetX
				destY := top*2 + offsetY

				for ty := 0; ty < thumbH; ty++ {
					for tx := 0; tx < thumbW; tx++ {
						dx := destX + tx
						dy := destY + ty
						if dx >= 0 && dx < termCols && dy >= 0 && dy < gridRowsLimit*2 {
							compImg.Set(dx, dy, thumb.At(tx, ty))
						}
					}
				}
			}
		}

		// Draw page composite image
		halfblock.CursorHome(os.Stdout)
		fmt.Fprintf(os.Stdout, "\x1b[%d;1H", marginTop+1)
		_ = halfblock.Render(os.Stdout, compImg)
		halfblock.EraseDown(os.Stdout)

		// Print filenames centered below thumbnails
		for idx := startIdx; idx < endIdx; idx++ {
			cellItemIdx := idx - startIdx
			colIdx := cellItemIdx % gridCols
			rowIdx := cellItemIdx / gridCols
			left := colIdx * (cellW + gapX)
			top := rowIdx * (cellH + gapY)
			rowAbs := marginTop + 1 + top + cellH - 1

			name := items[idx].name
			if len(name) > cellW {
				name = name[:cellW-3] + "..."
			}
			colAbs := left + 1 + (cellW-len(name))/2

			if idx == selectedIdx {
				// Subtle highlighting (Nord slate: bold white text on light slate/gray background)
				fmt.Fprintf(os.Stdout, "\x1b[%d;%dH\x1b[1;38;2;255;255;255;48;2;71;85;105m %s \x1b[m", rowAbs, colAbs, name)
			} else {
				fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%s", rowAbs, colAbs, name)
			}
		}

		// Print header
		dirInfo := "Root"
		if currentDir != "" {
			dirInfo = currentDir
		}
		fmt.Fprintf(os.Stdout, "\x1b[1;1H\x1b[K\x1b[1;36m Cati Browser \x1b[m [%s] — Page %d/%d (%d-%d of %d)",
			dirInfo, pageIdx+1, numPages, startIdx+1, endIdx, len(items))

		// Print bottom menu buttons
		buttons = drawBottomMenu(os.Stdout, termCols, effHeight, pageIdx, numPages, "grid", hoveredButtonAction)

		// Print status line
		fmt.Fprintf(os.Stdout, "\x1b[%d;1H\x1b[K\x1b[7m [Enter/Click] View/Enter  [◀/▶/Scroll] Page  [s] Settings  [a] About  [q] Quit \x1b[m", effHeight)
	}

	redraw()

	for {
		select {
		case <-sigs:
			return nil

		case k := <-inputs:
			termCols, termRows = resolveTermSize(initWidth, initHeight)
			effHeight := termRows
			if cfgHeight > 0 && cfgHeight < termRows {
				effHeight = cfgHeight
			}

			gridCols := 3
			gridRows := 2
			if termCols < 60 {
				gridCols = 2
			}
			if termCols < 40 {
				gridCols = 1
			}
			if effHeight < 14 {
				gridRows = 1
			}
			itemsPerPage := gridCols * gridRows

			changed := false
			shouldQuit := false

			processInput := func(tok string) {
				if btn, col, row, release, ok := parseSGRMouse(tok); ok {
					// Detect button hovers/motion
					if row == effHeight-1 {
						found := false
						for _, b := range buttons {
							if col >= b.col && col < b.col+b.width {
								if hoveredButtonAction != b.action {
									hoveredButtonAction = b.action
									changed = true
								}
								found = true
								break
							}
						}
						if !found && hoveredButtonAction != "" {
							hoveredButtonAction = ""
							changed = true
						}
					} else if hoveredButtonAction != "" {
						hoveredButtonAction = ""
						changed = true
					}

					if viewMode == "about" {
						if row == effHeight-1 && release {
							for _, b := range buttons {
								if col >= b.col && col < b.col+b.width {
									if b.action == "back" {
										viewMode = "grid"
										halfblock.ClearScreen(os.Stdout)
										changed = true
									} else if b.action == "quit" {
										shouldQuit = true
									}
								}
							}
						}
						return
					}

					if viewMode == "settings" {
						if row == effHeight-1 && release {
							for _, b := range buttons {
								if col >= b.col && col < b.col+b.width {
									switch b.action {
									case "inc":
										tempHeight = min(60, tempHeight+1)
										changed = true
									case "dec":
										tempHeight = max(10, tempHeight-1)
										changed = true
									case "save":
										cfgHeight = tempHeight
										_ = saveSettings(cfgHeight)
										viewMode = "grid"
										halfblock.ClearScreen(os.Stdout)
										changed = true
									case "cancel":
										tempHeight = cfgHeight
										viewMode = "grid"
										halfblock.ClearScreen(os.Stdout)
										changed = true
									}
								}
							}
						}
						return
					}

					// Grid view mouse interactions
					if sgrIsScroll(btn) && !release {
						if sgrScrollDir(btn) < 0 {
							selectedIdx = max(0, selectedIdx-itemsPerPage)
						} else {
							selectedIdx = min(len(items)-1, selectedIdx+itemsPerPage)
						}
						changed = true
					} else if row == effHeight-1 && release {
						// Button click
						for _, b := range buttons {
							if col >= b.col && col < b.col+b.width {
								switch b.action {
								case "prev":
									selectedIdx = max(0, selectedIdx-itemsPerPage)
									changed = true
								case "next":
									selectedIdx = min(len(items)-1, selectedIdx+itemsPerPage)
									changed = true
								case "settings":
									viewMode = "settings"
									tempHeight = cfgHeight
									changed = true
								case "about":
									viewMode = "about"
									changed = true
								case "quit":
									shouldQuit = true
								}
								break
							}
						}
					} else {
						// Hover and Click on grid items
						marginTop := 1
						c := col - 1
						r := row - 1 - marginTop

						gapX := 4
						gapY := 2
						gridRowsLimit := effHeight - marginTop - 3
						cellW := (termCols - (gridCols-1)*gapX) / gridCols
						cellH := (gridRowsLimit - (gridRows-1)*gapY) / gridRows
						if cellW < 10 {
							cellW = 10
						}
						if cellH < 4 {
							cellH = 4
						}

						pageIdx := selectedIdx / itemsPerPage
						startIdx := pageIdx * itemsPerPage
						endIdx := startIdx + itemsPerPage
						if endIdx > len(items) {
							endIdx = len(items)
						}

						for cellItemIdx := 0; cellItemIdx < (endIdx - startIdx); cellItemIdx++ {
							colIdx := cellItemIdx % gridCols
							rowIdx := cellItemIdx / gridCols
							left := colIdx * (cellW + gapX)
							right := left + cellW
							top := rowIdx * (cellH + gapY)
							bottom := top + cellH

							if c >= left && c < right && r >= top && r < bottom {
								clickedIdx := startIdx + cellItemIdx
								if clickedIdx != selectedIdx {
									selectedIdx = clickedIdx
									changed = true
								}

								// Handle click
								if !sgrIsScroll(btn) && !sgrIsDrag(btn) && sgrButton(btn) == 0 && !release {
									targetItem := items[selectedIdx]
									if targetItem.isDir {
										if targetItem.name == ".." {
											// Navigate up
											isInitialDir := false
											for _, init := range initialItems {
												if init.isDir && filepath.Clean(init.path) == filepath.Clean(currentDir) {
													isInitialDir = true
													break
												}
											}
											if isInitialDir {
												currentDir = ""
											} else {
												currentDir = filepath.Dir(currentDir)
											}
										} else {
											currentDir = targetItem.path
										}
										items = loadBrowserItems(currentDir, initialItems)
										selectedIdx = 0
										halfblock.ClearScreen(os.Stdout)
									} else {
										// Show single file
										if oldState != nil {
											_ = term.Restore(fd, oldState)
										}
										fmt.Fprint(os.Stdout, "\x1b[?1003l\x1b[?1006l")
										halfblock.ShowCursor(os.Stdout)

										_ = interactiveWithChan(targetItem.path, initWidth, initHeight, inputs)

										oldState, err = term.MakeRaw(fd)
										halfblock.HideCursor(os.Stdout)
										halfblock.ClearScreen(os.Stdout)
										fmt.Fprint(os.Stdout, "\x1b[?1003h\x1b[?1006h")
									}
									changed = true
								}
								break
							}
						}
					}
				} else {
					// Keyboard events
					if viewMode == "about" {
						switch tok {
						case "q", "Q", "\x1b", "a", "A":
							viewMode = "grid"
							halfblock.ClearScreen(os.Stdout)
							changed = true
						case "\x03":
							shouldQuit = true
						}
						return
					}

					if viewMode == "settings" {
						switch tok {
						case "\x1b[A": // ↑
							tempHeight = min(60, tempHeight+1)
							changed = true
						case "\x1b[B": // ↓
							tempHeight = max(10, tempHeight-1)
							changed = true
						case "\x0d", "\x0a": // Enter
							cfgHeight = tempHeight
							_ = saveSettings(cfgHeight)
							viewMode = "grid"
							halfblock.ClearScreen(os.Stdout)
							changed = true
						case "q", "Q", "\x1b", "c", "C": // Cancel
							tempHeight = cfgHeight
							viewMode = "grid"
							halfblock.ClearScreen(os.Stdout)
							changed = true
						case "\x03":
							shouldQuit = true
						}
						return
					}

					// Grid view keyboard inputs
					switch tok {
					case "q", "Q", "\x1b", "\x03":
						shouldQuit = true

					case "a", "A":
						viewMode = "about"
						changed = true

					case "s", "S":
						viewMode = "settings"
						tempHeight = cfgHeight
						changed = true

					case "\x1b[D": // Left arrow
						if selectedIdx > 0 {
							selectedIdx--
							changed = true
						}

					case "\x1b[C": // Right arrow
						if selectedIdx < len(items)-1 {
							selectedIdx++
							changed = true
						}

					case "\x1b[A": // Up arrow
						if selectedIdx >= gridCols {
							selectedIdx -= gridCols
							changed = true
						}

					case "\x1b[B": // Down arrow
						if selectedIdx+gridCols < len(items) {
							selectedIdx += gridCols
							changed = true
						}

					case "\x0d", "\x0a", " ": // Enter or Space
						targetItem := items[selectedIdx]
						if targetItem.isDir {
							if targetItem.name == ".." {
								isInitialDir := false
								for _, init := range initialItems {
									if init.isDir && filepath.Clean(init.path) == filepath.Clean(currentDir) {
										isInitialDir = true
										break
									}
								}
								if isInitialDir {
									currentDir = ""
								} else {
									currentDir = filepath.Dir(currentDir)
								}
							} else {
								currentDir = targetItem.path
							}
							items = loadBrowserItems(currentDir, initialItems)
							selectedIdx = 0
							halfblock.ClearScreen(os.Stdout)
						} else {
							if oldState != nil {
								_ = term.Restore(fd, oldState)
							}
							fmt.Fprint(os.Stdout, "\x1b[?1003l\x1b[?1006l")
							halfblock.ShowCursor(os.Stdout)

							_ = interactiveWithChan(targetItem.path, initWidth, initHeight, inputs)

							oldState, err = term.MakeRaw(fd)
							halfblock.HideCursor(os.Stdout)
							halfblock.ClearScreen(os.Stdout)
							fmt.Fprint(os.Stdout, "\x1b[?1003h\x1b[?1006h")
						}
						changed = true

					case "\x1b[5~", "[": // Page Up
						selectedIdx = max(0, selectedIdx-itemsPerPage)
						changed = true

					case "\x1b[6~", "]": // Page Down
						selectedIdx = min(len(items)-1, selectedIdx+itemsPerPage)
						changed = true
					}
				}
			}

			// Process first event
			processInput(k)
			if shouldQuit {
				return nil
			}

			// Coalesce / drain consecutive events
			draining := true
			for draining {
				select {
				case tok := <-inputs:
					processInput(tok)
					if shouldQuit {
						return nil
					}
				default:
					draining = false
				}
			}

			if changed {
				redraw()
			}
		}
	}
}

func drawAboutPage(w io.Writer, termCols, termRows int) {
	halfblock.ClearScreen(w)

	about := getAboutView()
	lines := strings.Split(about.Content, "\n")

	// Print Title
	titleLine := "=== " + about.Title + " ==="
	fmt.Fprintf(w, "\x1b[2;%dH\x1b[1;36m%s\x1b[m", max(1, (termCols-len(titleLine))/2), titleLine)

	for i, line := range lines {
		row := 4 + i
		if row >= termRows-2 {
			break
		}
		col := (termCols - len(line)) / 2
		if col < 1 {
			col = 1
		}
		fmt.Fprintf(w, "\x1b[%d;%dH\x1b[K%s", row, col, line)
	}
}

func drawSettingsPage(w io.Writer, termCols, termRows int, tempHeight int) {
	halfblock.ClearScreen(w)

	lines := []string{
		"=================================",
		"          CATI SETTINGS          ",
		"=================================",
		"",
		fmt.Sprintf("  Browser Height:  [ %d ] rows", tempHeight),
		"",
		"  Use ↑ / ↓ to adjust height.",
		"  Press Enter to Save, Esc to Cancel.",
		"",
		"=================================",
	}

	for i, line := range lines {
		row := (termRows-len(lines))/2 + i
		col := (termCols - len(line)) / 2
		if col < 1 {
			col = 1
		}
		fmt.Fprintf(w, "\x1b[%d;%dH\x1b[K%s", row, col, line)
	}
}

func drawBottomMenu(w io.Writer, termCols, termRows int, pageIdx, numPages int, viewMode string, activeAction string) []menuButton {
	var buttons []menuButton
	labels := loadLabels()

	// Clear the button line first
	fmt.Fprintf(w, "\x1b[%d;1H\x1b[K", termRows-1)

	if viewMode == "about" {
		btnBack := menuButton{
			label:  labels["back"],
			action: "back",
		}
		btnQuit := menuButton{
			label:  labels["quit"],
			action: "quit",
		}

		btnBack.col = 2
		btnBack.width = len(btnBack.label)

		btnQuit.col = btnBack.col + btnBack.width + 3
		btnQuit.width = len(btnQuit.label)

		buttons = append(buttons, btnBack, btnQuit)
	} else if viewMode == "settings" {
		btnInc := menuButton{
			label:  labels["inc"],
			action: "inc",
		}
		btnDec := menuButton{
			label:  labels["dec"],
			action: "dec",
		}
		btnSave := menuButton{
			label:  labels["save"],
			action: "save",
		}
		btnCancel := menuButton{
			label:  labels["cancel"],
			action: "cancel",
		}

		currentCol := 4
		btnInc.col = currentCol
		btnInc.width = len(btnInc.label)

		currentCol += btnInc.width + 3
		btnDec.col = currentCol
		btnDec.width = len(btnDec.label)

		currentCol += btnDec.width + 3
		btnSave.col = currentCol
		btnSave.width = len(btnSave.label)

		currentCol += btnSave.width + 3
		btnCancel.col = currentCol
		btnCancel.width = len(btnCancel.label)

		buttons = append(buttons, btnInc, btnDec, btnSave, btnCancel)
	} else {
		btnPrev := menuButton{
			label:  labels["prev"],
			action: "prev",
		}
		btnNext := menuButton{
			label:  labels["next"],
			action: "next",
		}
		btnSettings := menuButton{
			label:  labels["settings"],
			action: "settings",
		}
		btnAbout := menuButton{
			label:  labels["about"],
			action: "about",
		}
		btnQuit := menuButton{
			label:  labels["quit"],
			action: "quit",
		}

		currentCol := 2
		btnPrev.col = currentCol
		btnPrev.width = len(btnPrev.label)

		currentCol += btnPrev.width + 2
		btnNext.col = currentCol
		btnNext.width = len(btnNext.label)

		currentCol += btnNext.width + 2
		btnSettings.col = currentCol
		btnSettings.width = len(btnSettings.label)

		currentCol += btnSettings.width + 2
		btnAbout.col = currentCol
		btnAbout.width = len(btnAbout.label)

		currentCol += btnAbout.width + 2
		btnQuit.col = currentCol
		btnQuit.width = len(btnQuit.label)

		buttons = append(buttons, btnPrev, btnNext, btnSettings, btnAbout, btnQuit)
	}

	for _, btn := range buttons {
		if btn.action == activeAction {
			// Hover/Active subtle styling (Slate-gray highlight: bold white text on gray-slate background)
			fmt.Fprintf(w, "\x1b[%d;%dH\x1b[1;38;2;248;250;252;48;2;71;85;105m%s\x1b[m", termRows-1, btn.col, btn.label)
		} else {
			// Subtle Nord-style slate gray buttons
			fmt.Fprintf(w, "\x1b[%d;%dH\x1b[38;2;148;163;184;48;2;30;41;59m%s\x1b[m", termRows-1, btn.col, btn.label)
		}
	}
	return buttons
}
