package cmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
	browserui "ubunatic.com/cati/internal/browser"
	"ubunatic.com/cati/internal/input"
	spec "ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
)

type menuButton struct {
	label     string
	col       int    // 1-indexed column
	width     int    // character width
	action    string // primary action (left-click / primary key)
	altAction string // optional secondary action (right-click / alt_keys)
}

type browserItem struct {
	path  string
	isDir bool
	name  string
}

// ── Embedded folder icon ──────────────────────────────────────────────────────

//go:embed folder.png
var folderIconData []byte

var folderIcon image.Image

func initFolderIcon() {
	img, err := png.Decode(bytes.NewReader(folderIconData))
	if err != nil {
		panic(err)
	}
	folderIcon = img
}

func getFolderIcon() image.Image {
	if folderIcon == nil {
		initFolderIcon()
	}
	return folderIcon
}

// ── Style Configuration ──────────────────────────────────────────────────────

func loadBrowserItems(currentDir string, initialItems []browserItem) []browserItem {
	if currentDir == "" {
		return initialItems
	}

	var out []browserItem
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
			continue
		}
		path := filepath.Join(currentDir, name)
		if f.IsDir() {
			dirs = append(dirs, browserItem{
				path:  path,
				isDir: true,
				name:  name,
			})
		} else if isImageFile(path) {
			dirs = append(dirs, browserItem{
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

type scrollDragState struct {
	active bool
	startY int
}

func browser(args []string, initWidth, initHeight int, rc renderCfg, fullComp bool, initialZoom string, jobs int) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.MaxJobs = resolveWorkerCount(jobs, cfg.MaxJobs)
	inputSpec, err := input.Load(fs.FS(spec.FS))
	if err != nil {
		return fmt.Errorf("load input spec: %w", err)
	}
	style, err := loadStyle()
	if err != nil {
		return err
	}
	aboutView, err := getAboutView()
	if err != nil {
		return err
	}
	labels := loadLabels()
	for k, v := range loadButtons(style.BtnLeftCap, style.BtnRightCap) {
		labels[k] = v
	}
	viewBtnRows := loadViewButtonRows()
	viewKeyRows := loadViewKeyRows()
	controls, err := loadControls()
	if err != nil {
		return err
	}
	btnActions := loadButtonActions()
	// altBtnActions := loadAltButtonActions()
	viewKeyMaps := buildViewKeyMaps(viewKeyRows, loadButtonKeyDefs(inputSpec))

	cfgHeight := cfg.MaxPreviewHeight
	viewMode := cfg.ViewMode // "grid" or "preview", toggled dynamically
	previewVideos := cfg.PreviewVideos
	videoPreviewDelay := time.Duration(cfg.VideoPreviewDelay) * time.Millisecond

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

	thumbCache := make(map[thumbKey][]image.Image)
	submitted := make(map[thumbKey]bool)

	// Async metadata loading — one goroutine per path, results sent back here.
	type metaResult struct {
		path string
		meta MediaMeta
	}
	metaCache := make(map[string]*MediaMeta)
	metaLoading := make(map[string]bool)
	metaCh := make(chan metaResult, 8)

	// Per-video one-shot animation: plays frames once when video scrolls into view
	// or when the user moves the selection cursor onto a video.
	type animState struct {
		frameIdx int
		playing  bool
	}
	animMap := make(map[string]*animState) // keyed by item path

	// currentVisibleKeys is populated by redraw so the animTicker can find
	// cached frames for the selected item.
	currentVisibleKeys := make(map[thumbKey]bool)
	prevSelectedIdx := -1

	// thumbnail job queue and async workers
	tq := newThumbQueue()
	thumbResults := make(chan thumbResult, 64)
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	defer tq.stop()

	nWorkers := cfg.MaxJobs
	nVideoFrames := cfg.VideoFrames
	if nVideoFrames <= 0 {
		nVideoFrames = 10
	}
	startThumbWorkers(workerCtx, nWorkers, tq, previewVideos, nVideoFrames, rc, thumbResults)

	getThumbnail := func(item browserItem, cellW, cellH int) image.Image {
		if item.isDir {
			key := thumbKey{path: item.path, w: cellW, h: cellH}
			if frames, ok := thumbCache[key]; ok && len(frames) > 0 {
				return frames[0]
			}
			scaled := rc.scaleToFit(getFolderIcon(), cellW, cellH)
			thumbCache[key] = []image.Image{scaled}
			return scaled
		}
		key := thumbKey{path: item.path, w: cellW, h: cellH}
		if frames, ok := thumbCache[key]; ok {
			if len(frames) == 0 {
				return nil
			}
			if anim := animMap[item.path]; anim != nil {
				return frames[min(anim.frameIdx, len(frames)-1)]
			}
			return frames[0]
		}
		if !submitted[key] {
			submitted[key] = true
			isVideo := halfblock.IsVideo(item.path)
			if !isVideo || previewVideos {
				tq.submit(thumbJob{key: key, isVideo: isVideo})
			}
		}
		return nil
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
	notifySignals(sigs, inputSpec.SignalsFor(input.EventQuit))
	defer signal.Stop(sigs)

	inputs := make(chan string, 256)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			tokens := inputSpec.Tokenize(string(buf[:n]))
			for _, tok := range tokens {
				// Mouse SGR events (\x1b[<...) are dropped when the buffer is
				// nearly full so they never prevent keyboard tokens from being
				// queued. Keyboard tokens always block-send (they arrive rarely
				// and must not be lost).
				if strings.HasPrefix(tok, "\x1b[<") {
					select {
					case inputs <- tok:
					default:
					}
				} else {
					inputs <- tok
				}
			}
		}
	}()

	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	fmt.Fprint(os.Stdout, "\x1b[?1003h\x1b[?1006h")
	defer func() {
		fmt.Fprint(os.Stdout, "\x1b[?1003l\x1b[?1006l")
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	selectedIdx := 0
	selectionChangedAt := time.Now().Add(-videoPreviewDelay) // ready immediately on start

	// startSelectedVideoAnim starts the one-shot animation for the currently
	// selected video.  Called only after the hover delay has elapsed.
	startSelectedVideoAnim := func() {
		if selectedIdx < 0 || selectedIdx >= len(items) {
			return
		}
		item := items[selectedIdx]
		if item.isDir || !halfblock.IsVideo(item.path) {
			return
		}
		for key := range currentVisibleKeys {
			if key.path == item.path {
				if frames := thumbCache[key]; len(frames) > 1 {
					animMap[item.path] = &animState{frameIdx: 0, playing: true}
					return
				}
			}
		}
	}

	activeSettingsField := 0
	var tempCfg Settings
	var buttons []menuButton
	hoveredButtonAction := ""
	var scrollDrag scrollDragState
	var lastKey string

	termCols, termRows := resolveTermSize(initWidth, initHeight)

	redraw := func() {
		termCols, termRows = resolveTermSize(initWidth, initHeight)

		effHeight := termRows
		if cfgHeight > 0 && cfgHeight < termRows {
			effHeight = cfgHeight
		}

		if viewMode == "about" {
			drawAboutPage(os.Stdout, termCols, effHeight, style, aboutView)
			buttons = drawBottomMenu(os.Stdout, effHeight, termCols, "about", hoveredButtonAction, style, labels, viewBtnRows, nil, btnActions, nil)
			drawHintBar(os.Stdout, effHeight, termCols, labels["hint_about"], map[string]string{"last_key": lastKey}, style)
			return
		}

		if viewMode == "settings" {
			drawSettingsPage(os.Stdout, termCols, effHeight, controls, tempCfg, activeSettingsField, labels)
			buttons = drawBottomMenu(os.Stdout, effHeight, termCols, "settings", hoveredButtonAction, style, labels, viewBtnRows, nil, btnActions, nil)
			activeSetting := ""
			if activeSettingsField >= 0 && activeSettingsField < len(controls) {
				activeSetting = settingsFieldLabel(controls[activeSettingsField].Key)
			}
			drawHintBar(os.Stdout, effHeight, termCols, labels["hint_settings"], map[string]string{"active_setting": activeSetting, "last_key": lastKey}, style)
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

		// Detect if we should use dense list mode
		isDense := true
		for _, item := range items {
			if !item.isDir {
				isDense = false
				break
			}
		}

		gridCols := 3
		gridRows := 2
		gapX := 4
		gapY := 2
		cellH := (gridRowsLimit - (gridRows-1)*gapY) / gridRows

		if isDense {
			gridCols = max(1, (termCols-4)/22)
			gridRows = gridRowsLimit
			cellH = 1
			gapY = 0
			gapX = 2
		} else {
			if termCols < 60 {
				gridCols = 2
			}
			if termCols < 40 {
				gridCols = 1
			}
			if effHeight < 14 {
				gridRows = 1
				cellH = gridRowsLimit
			}
		}

		// Adjust sizes if preview mode is active
		leftW := termCols * 40 / 100
		if leftW < 25 {
			leftW = 25
		}
		if leftW > 50 {
			leftW = 50
		}

		if viewMode == "preview" {
			gridCols = 1
			gridRows = gridRowsLimit
			cellH = 1
			gapY = 0
			gapX = 0
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

		cellW := (termCols - (gridCols-1)*gapX) / gridCols
		if viewMode == "preview" {
			cellW = leftW
		}
		if cellW < 10 {
			cellW = 10
		}
		if cellH < 1 {
			cellH = 1
		}

		cellPixW, cellPixH := rc.renderCellSize()
		compW := termCols * cellPixW
		compH := gridRowsLimit * cellPixH
		compImg := image.NewRGBA(image.Rect(0, 0, compW, compH))

		// Apply background color from style
		var bgCol color.RGBA
		if c, ok := parseColor(style.PreviewBg); ok {
			bgCol = c
		}
		if bgCol.A > 0 {
			for y := 0; y < compH; y++ {
				for x := 0; x < compW; x++ {
					compImg.Set(x, y, bgCol)
				}
			}
		}

		// Render thumbnails
		if viewMode == "preview" {
			// Draw divider line
			for y := 0; y < gridRowsLimit; y++ {
				rowAbs := marginTop + 1 + y
				fmt.Fprintf(os.Stdout, "\x1b[%d;%dH│", rowAbs, leftW+1)
			}

			// Render right pane preview
			prevW := (termCols - style.ScrollWidth - 1) - (leftW + 2) + 1
			if prevW > 0 {
				targetItem := items[selectedIdx]
				previewImg := getThumbnail(targetItem, prevW, gridRowsLimit)
				if previewImg != nil {
					scaledW := previewImg.Bounds().Dx()
					scaledH := previewImg.Bounds().Dy()
					offsetX := (prevW*cellPixW - scaledW) / 2
					offsetY := (gridRowsLimit*cellPixH - scaledH) / 2
					destX := (leftW+2)*cellPixW + offsetX
					destY := offsetY

					for ty := 0; ty < scaledH; ty++ {
						for tx := 0; tx < scaledW; tx++ {
							dx := destX + tx
							dy := destY + ty
							if dx >= 0 && dx < compW && dy >= 0 && dy < compH {
								compImg.Set(dx, dy, previewImg.At(tx, ty))
							}
						}
					}
				}
			}
		} else if !isDense {
			// Grid thumbnails
			for idx := startIdx; idx < endIdx; idx++ {
				cellItemIdx := idx - startIdx
				colIdx := cellItemIdx % gridCols
				rowIdx := cellItemIdx / gridCols
				left := colIdx * (cellW + gapX) * cellPixW
				top := rowIdx * (cellH + gapY)

				thumb := getThumbnail(items[idx], cellW, cellH-1)
				if thumb != nil {
					thumbW := thumb.Bounds().Dx()
					thumbH := thumb.Bounds().Dy()
					offsetX := (cellW*cellPixW - thumbW) / 2
					offsetY := ((cellH-1)*cellPixH - thumbH) / 2
					destX := left + offsetX
					destY := top*cellPixH + offsetY
					if items[idx].isDir {
						destX-- // shift folder icon 1px left for better quad alignment
					}

					for ty := 0; ty < thumbH; ty++ {
						for tx := 0; tx < thumbW; tx++ {
							dx := destX + tx
							dy := destY + ty
							if dx >= 0 && dx < compW && dy >= 0 && dy < compH {
								compImg.Set(dx, dy, thumb.At(tx, ty))
							}
						}
					}
				}
			}
		}

		// Prioritize visible items and expose current visible set for animation triggers.
		visibleKeys := make(map[thumbKey]bool)
		for idx := startIdx; idx < endIdx; idx++ {
			if !items[idx].isDir {
				key := thumbKey{path: items[idx].path, w: cellW, h: cellH - 1}
				visibleKeys[key] = true
			}
		}
		currentVisibleKeys = visibleKeys
		tq.prioritize(visibleKeys)

		// Draw page composite image
		halfblock.CursorHome(os.Stdout)
		fmt.Fprintf(os.Stdout, "\x1b[%d;1H", marginTop+1)
		_ = renderChecked(os.Stdout, compImg, rc)
		halfblock.EraseDown(os.Stdout)

		// Print filenames centered below thumbnails (Grid) or in dynamic vertical lists
		folderIcon := labels["folder_icon"]
		fileIcon := labels["file_icon"]
		selAnsi := styleSelectedAnsi(style)
		itemAnsi := styleItemAnsi(style)
		marker := style.GridSelectedMarker

		if viewMode == "preview" {
			for idx := startIdx; idx < endIdx; idx++ {
				cellItemIdx := idx - startIdx
				rowAbs := marginTop + 1 + cellItemIdx

				name := items[idx].name
				if items[idx].isDir {
					name = folderIcon + " " + name
					if items[idx].name == ".." {
						name = folderIcon + " .."
					}
				} else {
					name = fileIcon + " " + name
				}

				if len(name) > leftW-3 {
					name = name[:leftW-6] + "..."
				}

				if idx == selectedIdx {
					spaces := strings.Repeat(" ", max(0, leftW-len(marker)-len(name)))
					fmt.Fprintf(os.Stdout, "\x1b[%d;1H%s%s%s%s\x1b[m", rowAbs, selAnsi, marker, name, spaces)
				} else {
					spaces := strings.Repeat(" ", max(0, leftW-1-len(name)))
					fmt.Fprintf(os.Stdout, "\x1b[%d;1H%s %s%s\x1b[m", rowAbs, itemAnsi, name, spaces)
				}
			}
		} else if isDense {
			for idx := startIdx; idx < endIdx; idx++ {
				cellItemIdx := idx - startIdx
				colIdx := cellItemIdx % gridCols
				rowIdx := cellItemIdx / gridCols
				left := colIdx * (cellW + gapX)
				rowAbs := marginTop + 1 + rowIdx

				name := folderIcon + " " + items[idx].name
				if items[idx].name == ".." {
					name = folderIcon + " .."
				}
				if len(name) > cellW {
					name = name[:cellW-3] + "..."
				}
				colAbs := left + 2

				if idx == selectedIdx {
					fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%s%s%s \x1b[m", rowAbs, colAbs, selAnsi, marker, name)
				} else {
					fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%s%s\x1b[m", rowAbs, colAbs, itemAnsi, name)
				}
			}
		} else {
			for idx := startIdx; idx < endIdx; idx++ {
				cellItemIdx := idx - startIdx
				colIdx := cellItemIdx % gridCols
				rowIdx := cellItemIdx / gridCols
				left := colIdx * (cellW + gapX)
				top := rowIdx * (cellH + gapY)
				rowAbs := marginTop + 1 + top + cellH - 1

				name := items[idx].name
				if items[idx].isDir {
					name = folderIcon + " " + name
					if items[idx].name == ".." {
						name = folderIcon + " .."
					}
				}
				if len(name) > cellW {
					name = name[:cellW-3] + "..."
				}
				colAbs := left + 1 + (cellW-len(name))/2

				if idx == selectedIdx {
					fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%s%s%s \x1b[m", rowAbs, colAbs, selAnsi, marker, name)
				} else {
					fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%s%s\x1b[m", rowAbs, colAbs, itemAnsi, name)
				}
			}
		}

		// Draw Paging / Scrollbar
		totalRows := (len(items) + gridCols - 1) / gridCols
		visibleRows := gridRows
		var thumbHeight, thumbTop int
		if totalRows <= visibleRows {
			thumbHeight = gridRowsLimit
			thumbTop = 0
		} else {
			thumbHeight = max(1, gridRowsLimit*visibleRows/totalRows)
			currentRow := selectedIdx / gridCols
			maxStartRow := totalRows - visibleRows
			if maxStartRow > 0 {
				thumbTop = (currentRow * (gridRowsLimit - thumbHeight)) / maxStartRow
			}
		}

		scrollbarCol := termCols - style.ScrollWidth + 1
		for y := 0; y < gridRowsLimit; y++ {
			rowAbs := marginTop + 1 + y
			var ch string
			var clr string
			if y >= thumbTop && y < thumbTop+thumbHeight {
				ch = style.ScrollThumbChar
				clr = styleFG(style.ScrollThumbFg, "")
			} else {
				ch = style.ScrollRailChar
				clr = styleFG(style.ScrollRailFg, "")
			}
			if style.ScrollWidth == 2 {
				ch = ch + ch
			}
			fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%s%s\x1b[m", rowAbs, scrollbarCol, clr, ch)
		}

		// Print header
		dirInfo := "Root"
		if currentDir != "" {
			dirInfo = currentDir
		}
		hdrBold := ""
		if style.HeaderBold {
			hdrBold = "\x1b[1m"
		}
		baseAnsi := hdrBold + styleFG(style.HeaderFg, "") + styleBG(style.HeaderBg, "")
		hdrText := fmtHeader(labels["header"], map[string]string{
			"app_name": labels["app_name"],
			"dir":      dirInfo,
			"page":     strconv.Itoa(pageIdx + 1),
			"pages":    strconv.Itoa(numPages),
			"start":    strconv.Itoa(startIdx + 1),
			"end":      strconv.Itoa(endIdx),
			"total":    strconv.Itoa(len(items)),
		}, baseAnsi)
		fmt.Fprintf(os.Stdout, "\x1b[1;1H\x1b[K%s%s\x1b[m", baseAnsi, hdrText)

		// Print bottom menu buttons
		buttons = drawBottomMenu(os.Stdout, effHeight, termCols, "grid", hoveredButtonAction, style, labels, viewBtnRows, nil, btnActions, nil)

		// Print hint bar
		activeFileName := ""
		previewState := ""
		queueSize := ""
		var currentMeta MediaMeta
		if selectedIdx >= 0 && selectedIdx < len(items) {
			item := items[selectedIdx]
			activeFileName = item.name
			if !item.isDir {
				key := thumbKey{path: item.path, w: cellW, h: cellH - 1}
				if frames, ok := thumbCache[key]; ok {
					if len(frames) > 1 {
						previewState = fmt.Sprintf("%df", len(frames))
					} else if halfblock.IsVideo(item.path) {
						previewState = "vid"
					} else {
						previewState = "img"
					}
				} else if submitted[key] {
					previewState = "…"
				}
				// meta: use cached value or kick off async load
				if m, ok := metaCache[item.path]; ok && m != nil {
					currentMeta = *m
				} else if !metaLoading[item.path] {
					metaLoading[item.path] = true
					go func(p string, isVid bool) {
						m := loadMediaMeta(p, isVid)
						metaCh <- metaResult{path: p, meta: m}
					}(item.path, halfblock.IsVideo(item.path))
				}
				currentMeta.DispW = fmt.Sprintf("%d", cellW)
				currentMeta.DispH = fmt.Sprintf("%d", cellH-1)
				currentMeta.DispMode = rcDispMode(rc)
			}
		}
		if n := tq.Len(); n > 0 {
			queueSize = fmt.Sprintf("↻%d", n)
		}
		hintVars := map[string]string{
			"active_file":   activeFileName,
			"preview_state": previewState,
			"queue_size":    queueSize,
			"last_key":      lastKey,
		}
		for k, v := range currentMeta.Vars() {
			hintVars[k] = v
		}
		drawHintBar(os.Stdout, effHeight, termCols, labels["hint_browser"], hintVars, style)
	}

	redraw()

	animTicker := time.NewTicker(300 * time.Millisecond)
	defer animTicker.Stop()

	for {
		select {
		case <-sigs:
			return nil

		case result := <-thumbResults:
			thumbCache[result.key] = result.frames
			redraw()

		case mr := <-metaCh:
			metaCache[mr.path] = &mr.meta
			redraw()

		case <-animTicker.C:
			// Start preview for the selected video after the hover delay.
			if time.Since(selectionChangedAt) >= videoPreviewDelay {
				startSelectedVideoAnim()
			}
			anyPlaying := false
			for path, anim := range animMap {
				if !anim.playing {
					continue
				}
				anim.frameIdx++
				for key := range currentVisibleKeys {
					if key.path == path {
						if frames := thumbCache[key]; anim.frameIdx >= len(frames) {
							anim.playing = false
						}
						break
					}
				}
				anyPlaying = true
			}
			if anyPlaying {
				redraw()
			}

		case k := <-inputs:
			termCols, termRows = resolveTermSize(initWidth, initHeight)
			effHeight := termRows
			if cfgHeight > 0 && cfgHeight < termRows {
				effHeight = cfgHeight
			}

			marginTop := 1
			marginBottom := 3
			gridRowsLimit := effHeight - marginTop - marginBottom
			_ = marginBottom

			isDense := true
			for _, item := range items {
				if !item.isDir {
					isDense = false
					break
				}
			}

			gridCols := 3
			gridRows := 2
			if isDense {
				gridCols = max(1, (termCols-4)/22)
				gridRows = gridRowsLimit
			} else {
				if termCols < 60 {
					gridCols = 2
				}
				if termCols < 40 {
					gridCols = 1
				}
				if effHeight < 14 {
					gridRows = 1
				}
			}

			leftW := termCols * 40 / 100
			if leftW < 25 {
				leftW = 25
			}
			if leftW > 50 {
				leftW = 50
			}

			if viewMode == "preview" {
				gridCols = 1
				gridRows = gridRowsLimit
			}
			itemsPerPage := gridCols * gridRows

			changed := false
			shouldQuit := false

			// viewKeyAction returns the action bound to tok in the current view's key map, if any.
			viewKeyAction := func(tok string) (string, bool) {
				name := viewMode
				if viewMode == "grid" || viewMode == "preview" {
					name = "browser"
				}
				action, ok := viewKeyMaps[name][tok]
				return action, ok
			}

			// navigateToParent goes up one directory level, or quits if already at root.
			navigateToParent := func() {
				if currentDir == "" {
					shouldQuit = true
					return
				}
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
				items = loadBrowserItems(currentDir, initialItems)
				selectedIdx = 0
				halfblock.ClearScreen(os.Stdout)
				changed = true
			}

			processInput := func(tok string) {
				lastKey = inputSpec.EventName(inputSpec.Classify(tok))
				if m, ok := inputSpec.ParseMouse(tok); ok {
					// Button hover / click
					if m.Row == effHeight-1 {
						found := false
						for _, b := range buttons {
							if m.Col >= b.col && m.Col < b.col+b.width {
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

					// View mode handlers
					if viewMode == "about" {
						if m.Row == effHeight-1 && m.Release {
							for _, b := range buttons {
								if m.Col >= b.col && m.Col < b.col+b.width {
									switch b.action {
									case "go_back":
										viewMode = "grid"
										halfblock.ClearScreen(os.Stdout)
										changed = true
									case "open_website":
										openWebsite(labels["website_url"])
									case "quit":
										shouldQuit = true
									}
								}
							}
						}
						return
					}

					if viewMode == "settings" {
						if m.Row == effHeight-1 && m.Release {
							for _, b := range buttons {
								if m.Col >= b.col && m.Col < b.col+b.width {
									switch b.action {
									case "save_settings":
										cfgHeight = tempCfg.MaxPreviewHeight
										previewVideos = tempCfg.PreviewVideos
										nVideoFrames = tempCfg.VideoFrames
										videoPreviewDelay = time.Duration(tempCfg.VideoPreviewDelay) * time.Millisecond
										viewMode = tempCfg.ViewMode
										if viewMode == "settings" {
											viewMode = "grid"
										}
										_ = saveConfig(Settings{
											MaxPreviewHeight:  cfgHeight,
											ViewMode:          viewMode,
											PreviewVideos:     previewVideos,
											MaxJobs:           tempCfg.MaxJobs,
											VideoFrames:       nVideoFrames,
											VideoPreviewDelay: tempCfg.VideoPreviewDelay,
										})
										halfblock.ClearScreen(os.Stdout)
										changed = true
									case "cancel_settings":
										viewMode = "grid"
										halfblock.ClearScreen(os.Stdout)
										changed = true
									case "quit":
										shouldQuit = true
									}
								}
							}
						}
						return
					}

					// Scrollbar dragging logic
					scrollbarCol := termCols - style.ScrollWidth + 1
					if !m.IsScroll() && !m.IsDrag() && m.Button == 0 && !m.Release {
						if m.Col >= scrollbarCol && m.Col <= termCols && m.Row >= marginTop+1 && m.Row <= marginTop+gridRowsLimit {
							scrollDrag.active = true
							scrollDrag.startY = m.Row
						}
					}
					if m.IsDrag() && m.Button == 0 && scrollDrag.active {
						totalRows := (len(items) + gridCols - 1) / gridCols
						if totalRows > gridRows {
							relativeRow := m.Row - (marginTop + 1)
							targetRow := relativeRow * totalRows / gridRowsLimit
							if targetRow < 0 {
								targetRow = 0
							}
							if targetRow >= totalRows {
								targetRow = totalRows - 1
							}
							selectedIdx = targetRow*gridCols + (selectedIdx % gridCols)
							if selectedIdx >= len(items) {
								selectedIdx = len(items) - 1
							}
							changed = true
						}
					}
					if !m.IsScroll() && !m.IsDrag() && m.Button == 0 && m.Release {
						scrollDrag.active = false
					}

					// Grid scroll wheel - scrolls by one row
					if m.IsScroll() && !m.Release {
						if m.ScrollDir() < 0 {
							selectedIdx = max(0, selectedIdx-gridCols)
						} else {
							selectedIdx = min(len(items)-1, selectedIdx+gridCols)
						}
						changed = true
					} else if m.Row == effHeight-1 && m.Release {
						// Controls button click
						for _, b := range buttons {
							if m.Col >= b.col && m.Col < b.col+b.width {
								switch b.action {
								case "go_back":
									navigateToParent()
								case "nav_prev":
									selectedIdx = max(0, selectedIdx-itemsPerPage)
									changed = true
								case "nav_next":
									selectedIdx = min(len(items)-1, selectedIdx+itemsPerPage)
									changed = true
								case "open_settings":
									tempCfg = Settings{
										MaxPreviewHeight: cfgHeight,
										ViewMode:         viewMode,
										PreviewVideos:    previewVideos,
										MaxJobs:          nWorkers,
										VideoFrames:      nVideoFrames,
									}
									viewMode = "settings"
									activeSettingsField = 0
									changed = true
								case "open_about":
									viewMode = "about"
									changed = true
								case "toggle_mode":
									if viewMode == "grid" {
										viewMode = "preview"
									} else {
										viewMode = "grid"
									}
									halfblock.ClearScreen(os.Stdout)
									changed = true
								case "quit":
									shouldQuit = true
								case "inc_zoom":
									cfgHeight = min(termRows, cfgHeight+1)
									changed = true
								case "dec_zoom":
									cfgHeight = max(10, cfgHeight-1)
									changed = true
								}
								break
							}
						}
					} else if !scrollDrag.active {
						// Hover & click cells coordinates
						marginTop := 1
						c := m.Col - 1
						r := m.Row - 1 - marginTop

						gapX := 4
						gapY := 2
						gridRowsLimit := effHeight - marginTop - 3
						cellW := (termCols - (gridCols-1)*gapX) / gridCols
						cellH := (gridRowsLimit - (gridRows-1)*gapY) / gridRows
						if cellW < 10 {
							cellW = 10
						}
						if cellH < 1 {
							cellH = 1
						}

						if viewMode == "preview" {
							cellW = leftW
							cellH = 1
							gapY = 0
							gapX = 0
						} else if isDense {
							cellH = 1
							gapY = 0
							gapX = 2
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

								// Execute navigation or view single image
								if !m.IsScroll() && !m.IsDrag() && m.Button == 0 && !m.Release {
									targetItem := items[selectedIdx]
									if targetItem.isDir {
										if targetItem.name == ".." {
											navigateToParent()
										} else {
											currentDir = targetItem.path
											items = loadBrowserItems(currentDir, initialItems)
											selectedIdx = 0
											halfblock.ClearScreen(os.Stdout)
										}
									} else {
										if oldState != nil {
											_ = term.Restore(fd, oldState)
										}
										fmt.Fprint(os.Stdout, "\x1b[?1003l\x1b[?1006l")
										halfblock.ShowCursor(os.Stdout)

										_ = forwardToPlayer(targetItem.path, initWidth, initHeight, rc, fullComp, initialZoom, jobs)

										// Propagate any quit signal that arrived while the viewer ran.
										select {
										case <-sigs:
											shouldQuit = true
											return
										default:
										}

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
						case "\x03":
							shouldQuit = true
						default:
							if action, ok := viewKeyAction(tok); ok {
								switch action {
								case "go_back", "open_about":
									viewMode = "grid"
									halfblock.ClearScreen(os.Stdout)
									changed = true
								case "quit":
									shouldQuit = true
								}
							}
						}
						return
					}

					if viewMode == "settings" {
						switch tok {
						case "\t": // Tab — cycle field (structural, not in spec)
							activeSettingsField = (activeSettingsField + 1) % len(controls)
							changed = true
						case "\x1b[A": // ↑ increase
							if activeSettingsField < len(controls) {
								applySettingsDelta(controls[activeSettingsField], +1, &tempCfg)
							}
							changed = true
						case "\x1b[B": // ↓ decrease
							if activeSettingsField < len(controls) {
								applySettingsDelta(controls[activeSettingsField], -1, &tempCfg)
							}
							changed = true
						case "\x03":
							shouldQuit = true
						default:
							if action, ok := viewKeyAction(tok); ok {
								switch action {
								case "save_settings":
									cfgHeight = tempCfg.MaxPreviewHeight
									previewVideos = tempCfg.PreviewVideos
									nVideoFrames = tempCfg.VideoFrames
									viewMode = tempCfg.ViewMode
									if viewMode == "settings" {
										viewMode = "grid"
									}
									_ = saveConfig(Settings{
										MaxPreviewHeight: cfgHeight,
										ViewMode:         viewMode,
										PreviewVideos:    previewVideos,
										MaxJobs:          tempCfg.MaxJobs,
										VideoFrames:      nVideoFrames,
									})
									halfblock.ClearScreen(os.Stdout)
									changed = true
								case "cancel_settings", "go_back":
									viewMode = "grid"
									halfblock.ClearScreen(os.Stdout)
									changed = true
								case "quit":
									shouldQuit = true
								}
							}
						}
						return
					}

					// Grid view keyboard inputs
					switch tok {
					case "\x0d", "\x0a", " ": // Enter or Space — open selected item (structural)
						targetItem := items[selectedIdx]
						if targetItem.isDir {
							if targetItem.name == ".." {
								navigateToParent()
							} else {
								currentDir = targetItem.path
								items = loadBrowserItems(currentDir, initialItems)
								selectedIdx = 0
								halfblock.ClearScreen(os.Stdout)
							}
						} else {
							if oldState != nil {
								_ = term.Restore(fd, oldState)
							}
							fmt.Fprint(os.Stdout, "\x1b[?1003l\x1b[?1006l")
							halfblock.ShowCursor(os.Stdout)

							_ = forwardToPlayer(targetItem.path, initWidth, initHeight, rc, fullComp, initialZoom, jobs)

							// Propagate any quit signal that arrived while the viewer ran.
							select {
							case <-sigs:
								shouldQuit = true
								return
							default:
							}

							oldState, err = term.MakeRaw(fd)
							halfblock.HideCursor(os.Stdout)
							halfblock.ClearScreen(os.Stdout)
							fmt.Fprint(os.Stdout, "\x1b[?1003h\x1b[?1006h")
						}
						changed = true

					default:
						// Spec-driven key dispatch — action names from spec/buttons.yaml keys: field.
						action, ok := viewKeyAction(tok)
						if !ok {
							break
						}
						switch action {
						case "quit":
							shouldQuit = true
						case "go_back":
							navigateToParent()
						case "open_about":
							viewMode = "about"
							changed = true
						case "open_settings":
							tempCfg = Settings{
								MaxPreviewHeight: cfgHeight,
								ViewMode:         viewMode,
								PreviewVideos:    previewVideos,
								MaxJobs:          nWorkers,
								VideoFrames:      nVideoFrames,
							}
							viewMode = "settings"
							activeSettingsField = 0
							changed = true
						case "toggle_mode":
							if viewMode == "grid" {
								viewMode = "preview"
							} else {
								viewMode = "grid"
							}
							halfblock.ClearScreen(os.Stdout)
							changed = true
						case "inc_zoom":
							cfgHeight = min(termRows, cfgHeight+1)
							changed = true
						case "dec_zoom":
							cfgHeight = max(10, cfgHeight-1)
							changed = true
						case "nav_prev":
							if selectedIdx > 0 {
								selectedIdx--
								changed = true
							}
						case "nav_next":
							if selectedIdx < len(items)-1 {
								selectedIdx++
								changed = true
							}
						case "nav_page_prev":
							selectedIdx = max(0, selectedIdx-itemsPerPage)
							changed = true
						case "nav_page_next":
							selectedIdx = min(len(items)-1, selectedIdx+itemsPerPage)
							changed = true
						case "nav_up":
							if selectedIdx >= gridCols {
								selectedIdx -= gridCols
								changed = true
							}
						case "nav_down":
							if selectedIdx+gridCols < len(items) {
								selectedIdx += gridCols
								changed = true
							}
						case "open_website":
							openWebsite(labels["website_url"])
						}
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
				if selectedIdx != prevSelectedIdx {
					prevSelectedIdx = selectedIdx
					selectionChangedAt = time.Now()
				}
			}
		}
	}
}

func parseColor(s string) (color.RGBA, bool) { return browserui.ParseColor(s) }
func styleFG(s, def string) string           { return browserui.StyleFG(s, def) }
func styleBG(s, def string) string           { return browserui.StyleBG(s, def) }
func renderTpl(tpl string, vars map[string]string, baseANSI string) string {
	return browserui.RenderTemplate(tpl, vars, baseANSI)
}
func tplWidth(tpl string, vars map[string]string) int { return browserui.TemplateWidth(tpl, vars) }
func truncateANSI(s string, width int) string         { return browserui.TruncateANSI(s, width) }
