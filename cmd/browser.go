package cmd

import (
	"fmt"
	"image"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"golang.org/x/term"
)

type menuButton struct {
	label  string
	col    int // 1-indexed column
	width  int // character width
	action string // e.g. "prev", "next", "about", "back", "quit"
}

// browser runs an interactive grid preview of multiple images and videos.
func browser(paths []string, initWidth, initHeight int) error {
	// Cache for thumbnails to keep resizing and page navigation super responsive.
	type thumbKey struct {
		path string
		w, h int
	}
	thumbCache := make(map[thumbKey]image.Image)

	getThumbnail := func(path string, cellW, cellH int) (image.Image, error) {
		key := thumbKey{path: path, w: cellW, h: cellH}
		if img, ok := thumbCache[key]; ok {
			return img, nil
		}
		img, err := halfblock.LoadImage(path)
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
		oldState = nil // Not a terminal or mock tests
	}
	defer func() {
		if oldState != nil {
			_ = term.Restore(fd, oldState)
		}
	}()

	// Signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	// Setup input channel
	inputs := make(chan string, 32)
	go func() {
		buf := make([]byte, 32)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			inputs <- string(buf[:n])
		}
	}()

	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	halfblock.EnableMouse(os.Stdout)
	defer func() {
		halfblock.DisableMouse(os.Stdout)
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	selectedIdx := 0
	viewMode := "grid" // "grid" or "about"
	var buttons []menuButton

	termCols, termRows := resolveTermSize(initWidth, initHeight)

	redraw := func() {
		termCols, termRows = resolveTermSize(initWidth, initHeight)

		if viewMode == "about" {
			drawAboutPage(os.Stdout, termCols, termRows)
			buttons = drawBottomMenu(os.Stdout, termCols, termRows, 0, 0, true)
			return
		}

		// Grid view
		marginTop := 1
		marginBottom := 3
		gridRowsLimit := termRows - marginTop - marginBottom

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
		if termRows < 14 {
			gridRows = 1
		}

		itemsPerPage := gridCols * gridRows
		numPages := (len(paths) + itemsPerPage - 1) / itemsPerPage

		if selectedIdx < 0 {
			selectedIdx = 0
		}
		if selectedIdx >= len(paths) {
			selectedIdx = len(paths) - 1
		}
		pageIdx := selectedIdx / itemsPerPage
		startIdx := pageIdx * itemsPerPage
		endIdx := startIdx + itemsPerPage
		if endIdx > len(paths) {
			endIdx = len(paths)
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

		// Render thumbnails onto the composite canvas
		for idx := startIdx; idx < endIdx; idx++ {
			cellItemIdx := idx - startIdx
			colIdx := cellItemIdx % gridCols
			rowIdx := cellItemIdx / gridCols
			left := colIdx * (cellW + gapX)
			top := rowIdx * (cellH + gapY)

			thumb, err := getThumbnail(paths[idx], cellW, cellH-1)
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

		// Draw the page composite image
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

			name := filepath.Base(paths[idx])
			if len(name) > cellW {
				name = name[:cellW-3] + "..."
			}
			colAbs := left + 1 + (cellW-len(name))/2

			if idx == selectedIdx {
				// Reverse video (bold black text on green chip background)
				fmt.Fprintf(os.Stdout, "\x1b[%d;%dH\x1b[1;30;42m %s \x1b[m", rowAbs, colAbs, name)
			} else {
				fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%s", rowAbs, colAbs, name)
			}
		}

		// Print title / pager header
		fmt.Fprintf(os.Stdout, "\x1b[1;1H\x1b[K\x1b[1;36m cati Image Browser \x1b[m — Page %d/%d (%d-%d of %d)",
			pageIdx+1, numPages, startIdx+1, endIdx, len(paths))

		// Print bottom menu buttons
		buttons = drawBottomMenu(os.Stdout, termCols, termRows, pageIdx, numPages, false)

		// Print status line
		fmt.Fprintf(os.Stdout, "\x1b[%d;1H\x1b[K\x1b[7m [Enter/Click] View  [◀/▶/Scroll] Page  [a] About  [q] Quit \x1b[m", termRows)
	}

	redraw()

	for {
		select {
		case <-sigs:
			return nil

		case k := <-inputs:
			termCols, termRows = resolveTermSize(initWidth, initHeight)

			gridCols := 3
			gridRows := 2
			if termCols < 60 {
				gridCols = 2
			}
			if termCols < 40 {
				gridCols = 1
			}
			if termRows < 14 {
				gridRows = 1
			}
			itemsPerPage := gridCols * gridRows

			if btn, col, row, release, ok := parseSGRMouse(k); ok {
				if viewMode == "about" {
					if row == termRows-1 && release {
						for _, b := range buttons {
							if col >= b.col && col < b.col+b.width {
								if b.action == "back" {
									viewMode = "grid"
									halfblock.ClearScreen(os.Stdout)
									redraw()
								} else if b.action == "quit" {
									return nil
								}
							}
						}
					}
					continue
				}

				// Grid view mouse interactions
				if sgrIsScroll(btn) && !release {
					if sgrScrollDir(btn) < 0 {
						selectedIdx = max(0, selectedIdx-itemsPerPage)
					} else {
						selectedIdx = min(len(paths)-1, selectedIdx+itemsPerPage)
					}
					redraw()
				} else if row == termRows-1 && release {
					// Button click
					for _, b := range buttons {
						if col >= b.col && col < b.col+b.width {
							switch b.action {
							case "prev":
								selectedIdx = max(0, selectedIdx-itemsPerPage)
								redraw()
							case "next":
								selectedIdx = min(len(paths)-1, selectedIdx+itemsPerPage)
								redraw()
							case "about":
								viewMode = "about"
								redraw()
							case "quit":
								return nil
							}
							break
						}
					}
				} else if !sgrIsScroll(btn) && !sgrIsDrag(btn) && sgrButton(btn) == 0 && !release {
					// Left-click on thumbnail
					marginTop := 1
					c := col - 1
					r := row - 1 - marginTop

					gapX := 4
					gapY := 2
					gridRowsLimit := termRows - marginTop - 3
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
					if endIdx > len(paths) {
						endIdx = len(paths)
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
							selectedIdx = clickedIdx
							redraw()

							// Open single interactive view
							if oldState != nil {
								_ = term.Restore(fd, oldState)
							}
							halfblock.DisableMouse(os.Stdout)
							halfblock.ShowCursor(os.Stdout)

							_ = interactive(paths[selectedIdx], initWidth, initHeight)

							oldState, err = term.MakeRaw(fd)
							halfblock.HideCursor(os.Stdout)
							halfblock.ClearScreen(os.Stdout)
							halfblock.EnableMouse(os.Stdout)

							redraw()
							break
						}
					}
				}
			} else {
				// Keyboard events
				if viewMode == "about" {
					switch k {
					case "q", "Q", "\x1b", "a", "A":
						viewMode = "grid"
						halfblock.ClearScreen(os.Stdout)
						redraw()
					case "\x03": // Ctrl+C
						return nil
					}
					continue
				}

				// Grid view keyboard inputs
				switch k {
				case "q", "Q", "\x1b", "\x03":
					return nil

				case "a", "A":
					viewMode = "about"
					redraw()

				case "\x1b[D": // Left arrow
					if selectedIdx > 0 {
						selectedIdx--
						redraw()
					}

				case "\x1b[C": // Right arrow
					if selectedIdx < len(paths)-1 {
						selectedIdx++
						redraw()
					}

				case "\x1b[A": // Up arrow
					if selectedIdx >= gridCols {
						selectedIdx -= gridCols
						redraw()
					}

				case "\x1b[B": // Down arrow
					if selectedIdx+gridCols < len(paths) {
						selectedIdx += gridCols
						redraw()
					}

				case "\x0d", "\x0a", " ": // Enter or Space
					if oldState != nil {
						_ = term.Restore(fd, oldState)
					}
					halfblock.DisableMouse(os.Stdout)
					halfblock.ShowCursor(os.Stdout)

					_ = interactive(paths[selectedIdx], initWidth, initHeight)

					oldState, err = term.MakeRaw(fd)
					halfblock.HideCursor(os.Stdout)
					halfblock.ClearScreen(os.Stdout)
					halfblock.EnableMouse(os.Stdout)

					redraw()

				case "\x1b[5~", "[": // Page Up
					selectedIdx = max(0, selectedIdx-itemsPerPage)
					redraw()

				case "\x1b[6~", "]": // Page Down
					selectedIdx = min(len(paths)-1, selectedIdx+itemsPerPage)
					redraw()
				}
			}
		}
	}
}

func drawAboutPage(w io.Writer, termCols, termRows int) {
	halfblock.ClearScreen(w)

	lines := []string{
		"                   _   _ ",
		"  ___  __ _  _ _  (_) | |",
		" / __|/ _` || ' \\ | | |_|",
		"| (__| (_| || | | || |  _ ",
		" \\___|\\__,||_|_|_||_| |_|",
		"  cat for images & video in terminal",
		"",
		"Version: 1.0.0",
		"License: AGPL-3.0-or-later",
		"Authors: Uwe Jugel (codeberg.org/ubunatic/cati)",
		"",
		"Controls (Grid Preview):",
		"  • Left/Right/Up/Down Arrow: Move selection",
		"  • PageUp/PageDown, [, ]: Navigate pages",
		"  • Mouse wheel: Scroll pages",
		"  • Click thumbnail / Enter / Space: View full screen",
		"  • a / A: Toggle About page",
		"  • q / Esc: Quit application",
		"",
		"Controls (Interactive Single View):",
		"  • + / -: Zoom in / zoom out (centred on screen)",
		"  • Mouse wheel: Zoom in / zoom out at cursor position",
		"  • Left-click drag: Pan (grab-and-pull the image)",
		"  • Up/Down/Left/Right Arrows: Pan the image",
		"  • c / C: Copy current viewport to clipboard (PNG)",
		"  • q / Esc: Go back to Grid view",
	}

	for i, line := range lines {
		row := 2 + i
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

func drawBottomMenu(w io.Writer, termCols, termRows int, pageIdx, numPages int, isAboutPage bool) []menuButton {
	var buttons []menuButton

	// Clear the button line first
	fmt.Fprintf(w, "\x1b[%d;1H\x1b[K", termRows-1)

	if isAboutPage {
		btnBack := menuButton{
			label:  "[◀ Back]",
			action: "back",
		}
		btnQuit := menuButton{
			label:  "[✖ Quit]",
			action: "quit",
		}

		btnBack.col = 2
		btnBack.width = len(btnBack.label)

		btnQuit.col = btnBack.col + btnBack.width + 3
		btnQuit.width = len(btnQuit.label)

		buttons = append(buttons, btnBack, btnQuit)
	} else {
		btnPrev := menuButton{
			label:  "[◀ Prev]",
			action: "prev",
		}
		btnNext := menuButton{
			label:  "[Next ▶]",
			action: "next",
		}
		btnAbout := menuButton{
			label:  "[ℹ About]",
			action: "about",
		}
		btnQuit := menuButton{
			label:  "[✖ Quit]",
			action: "quit",
		}

		currentCol := 4
		btnPrev.col = currentCol
		btnPrev.width = len(btnPrev.label)

		currentCol += btnPrev.width + 3
		btnNext.col = currentCol
		btnNext.width = len(btnNext.label)

		currentCol += btnNext.width + 3
		btnAbout.col = currentCol
		btnAbout.width = len(btnAbout.label)

		currentCol += btnAbout.width + 3
		btnQuit.col = currentCol
		btnQuit.width = len(btnQuit.label)

		buttons = append(buttons, btnPrev, btnNext, btnAbout, btnQuit)
	}

	for _, btn := range buttons {
		fmt.Fprintf(w, "\x1b[%d;%dH\x1b[7m%s\x1b[m", termRows-1, btn.col, btn.label)
	}
	return buttons
}
