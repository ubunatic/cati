package cmd

import (
	"fmt"
	"image"
	"io"
	"math"

	"codeberg.org/ubunatic/cati/internal/input"
	"codeberg.org/ubunatic/cati/internal/metrics"
	"codeberg.org/ubunatic/cati/internal/viewgeom"
)

// viewerCore holds the shared state and logic for image and video viewers.
// Viewer-specific behaviour is provided via the rerender callback and handled
// by checking the unhandledAction return from handleKey / handleMouse.
type viewerCore struct {
	// config (set once at construction)
	viewName    string
	termCols    int
	termRows    int
	fullComp    bool
	inputSpec   *input.Spec
	style       *StyleConfig
	labels      map[string]string
	viewBtnRows map[string]string
	viewKeyMaps map[string]map[string]string
	btnActions  map[string]string

	// mutable viewer state
	rc          renderCfg
	state       viewState
	drag        dragState
	curQ        metrics.RenderQuality
	modeName    string
	lastNonHBID int
	lastSrcW    int
	lastSrcH    int
	src         image.Image // last known source image (orig for images, lastRawFrame for video)

	// UI state
	buttons      []menuButton
	activeAction string
	status       string
	infoVisible  bool
	lastKey      string
	lastVP       image.Image // last derived viewport pixel image

	// rerender derives the viewport from the current state (updates lastVP + curQ).
	// It does NOT render to screen. Set by the owning viewer after construction.
	rerender func()
}

const viewerChromeRows = 2

func newViewerCore(
	viewName string, initWidth, initHeight int, rc renderCfg, fullComp bool,
	inputSpec *input.Spec, style *StyleConfig, labels map[string]string,
	viewBtnRows map[string]string, viewKeyMaps map[string]map[string]string,
	btnActions map[string]string,
) *viewerCore {
	cols, rows := resolveViewerTermSize(initWidth, initHeight)
	modeName := rcModeName(rc)
	lastNonHBID := rc.id
	if !(rc.mode.useQuad() || rc.mode.useGeomShape()) {
		lastNonHBID = -1
	}
	return &viewerCore{
		viewName:    viewName,
		termCols:    cols,
		termRows:    rows,
		fullComp:    fullComp,
		inputSpec:   inputSpec,
		style:       style,
		labels:      labels,
		viewBtnRows: viewBtnRows,
		viewKeyMaps: viewKeyMaps,
		btnActions:  btnActions,
		rc:          rc,
		state:       viewState{zoom: 1.0},
		modeName:    modeName,
		lastNonHBID: lastNonHBID,
		rerender:    func() {},
	}
}

func (vc *viewerCore) viewRows() int {
	return max(1, vc.termRows-viewerChromeRows)
}

// switchMode adjusts pan and preserves the zoom k-value after a render-mode
// switch. It is a no-op when src is nil (e.g. before the first video frame).
func (vc *viewerCore) switchMode(oldRC renderCfg) {
	if vc.src == nil {
		return
	}
	preserveZoomForMode(&vc.state, vc.src, vc.termCols, vc.viewRows(), oldRC, vc.rc)
	recenterForMode(&vc.state, vc.src, vc.termCols, vc.viewRows(), oldRC, vc.rc)
}

// handleAction executes a spec action and returns (quit, changed).
// Returns (false, false) for actions not owned by the core (caller handles them).
func (vc *viewerCore) handleAction(action, tok string) (quit, changed bool) {
	switch action {
	case "go_back", "quit":
		return true, false

	case "inc_zoom":
		if vc.lastSrcW > 0 {
			steps := zoomSteps(maxZoom(vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows(), vc.rc.mode), vc.lastSrcW)
			i := stepIdx(vc.state.zoom, steps)
			if i > 0 {
				vc.state.zoom = steps[i-1]
				vc.rerender()
				return false, true
			}
		}

	case "dec_zoom":
		if vc.lastSrcW > 0 {
			steps := zoomSteps(maxZoom(vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows(), vc.rc.mode), vc.lastSrcW)
			i := stepIdx(vc.state.zoom, steps)
			if i < len(steps)-1 {
				vc.state.zoom = steps[i+1]
				vc.rerender()
				return false, true
			}
		}

	case "zoom_k":
		if vc.lastSrcW > 0 {
			k := 1.0
			if len(tok) == 1 && tok[0] >= '0' && tok[0] <= '9' {
				val := int(tok[0] - '0')
				if val == 0 {
					vc.state.zoom = 1.0
					vc.rerender()
					return false, true
				}
				if val != 1 {
					k = float64(val)
				}
			}
			k = math.Max(k, 1.0/float64(vc.lastSrcW))
			k = math.Min(k, float64(vc.lastSrcW))
			mz := maxZoom(vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows(), vc.rc.mode)
			vc.state.zoom = vc.rc.mode.viewSpec().ZoomRatioForK(mz, k)
			vc.rerender()
			return false, true
		}

	case "cycle_render":
		oldRC := vc.rc
		vc.rc, vc.modeName = cycleRenderCfg(vc.rc)
		vc.switchMode(oldRC)
		vc.rerender()
		return false, true

	case "cycle_render_prev":
		oldRC := vc.rc
		vc.rc, vc.modeName = cycleRenderCfgPrev(vc.rc)
		vc.switchMode(oldRC)
		vc.rerender()
		return false, true

	case "toggle_gray":
		cycleGray(&vc.rc)
		vc.rerender()
		return false, true

	case "toggle_halfblock":
		oldRC := vc.rc
		graySaved, grayColorsSaved := vc.rc.gray, vc.rc.grayColors
		if vc.rc.mode.useQuad() || vc.rc.mode.useGeomShape() {
			vc.lastNonHBID = vc.rc.id
			if m, n, ok := findRenderModeByID(0); ok {
				vc.rc, vc.modeName = m, n
			}
		} else if vc.lastNonHBID >= 0 {
			if m, n, ok := findRenderModeByID(vc.lastNonHBID); ok {
				vc.rc, vc.modeName = m, n
			}
		} else {
			vc.rc, vc.modeName = cycleRenderCfg(vc.rc)
		}
		vc.rc.gray = graySaved
		vc.rc.grayColors = grayColorsSaved
		vc.switchMode(oldRC)
		vc.rerender()
		return false, true

	case "copy_viewport":
		if vc.lastVP != nil {
			if copyErr := copyImageToClipboard(vc.lastVP); copyErr != nil {
				vc.status = "⚠ copy failed: " + copyErr.Error()
			} else {
				vc.status = "✓ copied to clipboard"
			}
			return false, true
		}

	case "show_info":
		vc.infoVisible = !vc.infoVisible
		vc.status = ""
		return false, true
	}
	return false, false
}

// handleKey dispatches a keyboard token. Structural keys (Ctrl-C, arrows) are
// handled directly; spec-mapped keys are dispatched via handleAction.
// Returns unhandledAction when a key maps to a spec action that handleAction did
// not claim — the caller should handle it.
func (vc *viewerCore) handleKey(tok string) (quit, changed bool, unhandledAction string) {
	vc.lastKey = vc.inputSpec.EventName(vc.inputSpec.Classify(tok))
	geom := vc.rc.mode.viewSpec()
	mz := 1.0
	if vc.lastSrcW > 0 {
		mz = maxZoom(vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows(), vc.rc.mode)
	}
	k := max(1, int(math.Round(mz/vc.state.zoom)))
	hStep := max(1, min(vc.termCols/8, k))
	vStep := max(1, min(vc.viewRows()/8, k))

	switch tok {
	case "\x03": // ctrl-c always quits
		return true, false, ""
	case "\x1b[A": // ↑ — structural pan
		geom.PanByCells(&vc.state.panX, &vc.state.panY, 0, -vStep)
		vc.rerender()
		return false, true, ""
	case "\x1b[B": // ↓ — structural pan
		geom.PanByCells(&vc.state.panX, &vc.state.panY, 0, vStep)
		vc.rerender()
		return false, true, ""
	case "\x1b[C": // → — structural pan
		geom.PanByCells(&vc.state.panX, &vc.state.panY, hStep, 0)
		vc.rerender()
		return false, true, ""
	case "\x1b[D": // ← — structural pan
		geom.PanByCells(&vc.state.panX, &vc.state.panY, -hStep, 0)
		vc.rerender()
		return false, true, ""
	default:
		if action, ok := vc.viewKeyMaps[vc.viewName][tok]; ok {
			q, ch := vc.handleAction(action, tok)
			if q || ch {
				return q, ch, ""
			}
			return false, false, action
		}
	}
	return false, false, ""
}

// handleMouse dispatches a mouse event. Button-bar clicks dispatch to handleAction;
// canvas scroll and drag update zoom/pan. Returns unhandledAction when a button
// action was not claimed by handleAction — the caller should handle it.
func (vc *viewerCore) handleMouse(m input.MouseEvent) (quit, changed bool, unhandledAction string) {
	c, r := m.Col-1, m.Row-1

	if m.Row == vc.termRows-1 {
		// ── Button bar ────────────────────────────────────────────────────────
		newAction := ""
		for _, b := range vc.buttons {
			if m.Col >= b.col && m.Col < b.col+b.width {
				newAction = b.action
				break
			}
		}
		if newAction != vc.activeAction {
			vc.activeAction = newAction
			changed = true
		}
		if m.Release && newAction != "" {
			q, ch := vc.handleAction(newAction, "")
			if q {
				return true, true, ""
			}
			if ch {
				return false, true, ""
			}
			return false, changed, newAction
		}
		return false, changed, ""
	}

	if vc.activeAction != "" {
		vc.activeAction = ""
		changed = true
	}

	geomV := vc.rc.mode.viewSpec()
	switch {
	// ── Scroll wheel: zoom at cursor ──────────────────────────────────────────
	case m.IsScroll() && !m.Release && vc.lastSrcW > 0:
		steps := zoomSteps(maxZoom(vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows(), vc.rc.mode), vc.lastSrcW)
		i := stepIdx(vc.state.zoom, steps)
		if m.ScrollDir() < 0 && i > 0 {
			zoomAtCursor(&vc.state, steps[i-1], c, r, vc.rc.mode)
			vc.rerender()
			changed = true
		} else if m.ScrollDir() >= 0 && i < len(steps)-1 {
			zoomAtCursor(&vc.state, steps[i+1], c, r, vc.rc.mode)
			vc.rerender()
			changed = true
		}
	// ── Left press: start drag ────────────────────────────────────────────────
	case !m.IsScroll() && !m.IsDrag() && m.Button == 0 && !m.Release:
		vc.drag = viewgeom.NewPanAnchor(c, r, vc.state.panX, vc.state.panY)
	// ── Left drag: update pan ─────────────────────────────────────────────────
	case m.IsDrag() && m.Button == 0 && vc.drag.Active:
		vc.state.panX, vc.state.panY = geomV.PanFromAnchor(vc.drag, c, r)
		vc.rerender()
		changed = true
	// ── Left release: end drag ────────────────────────────────────────────────
	case !m.IsScroll() && !m.IsDrag() && m.Button == 0 && m.Release:
		vc.drag.Active = false
	}
	return false, changed, ""
}

// hintVars builds the shared hint variable map for drawHintBar.
func (vc *viewerCore) hintVars(fileMeta MediaMeta, hint string, extra map[string]string) map[string]string {
	zoomLabel := vc.rc.mode.viewSpec().ZoomLevel(vc.state.zoom, vc.lastSrcW, vc.lastSrcH, vc.termCols, vc.viewRows())
	if vc.src != nil && vc.lastSrcW > 0 {
		zoomLabel = zoomLevel(vc.state, vc.src, vc.termCols, vc.viewRows(), vc.rc)
	}
	graySuffix := ""
	if vc.rc.gray {
		graySuffix = fmt.Sprintf(" (gray%d)", grayColorsCount(vc.rc.grayColors))
	}
	vars := map[string]string{
		"last_key":    vc.lastKey,
		"ssim":        fmt.Sprintf("%.3f", vc.curQ.SSIM),
		"blockiness":  fmt.Sprintf("%.3f", vc.curQ.Blockiness),
		"edge_cont":   fmt.Sprintf("%.3f", vc.curQ.EdgeCont),
		"render_mode": vc.modeName + graySuffix,
		"zoom_level":  zoomLabel,
	}
	for k, v := range extra {
		vars[k] = v
	}
	return viewerHintVars(fileMeta, vc.termCols, hint, vars)
}

// drawMenu renders the button bar and records the button layout for mouse dispatch.
func (vc *viewerCore) drawMenu(w io.Writer, conditions map[string]bool) {
	vc.buttons = drawBottomMenu(w, vc.termRows, vc.termCols, vc.viewName, vc.activeAction, vc.style, vc.labels, vc.viewBtnRows, conditions, vc.btnActions, nil)
}

// drawHint renders the hint bar.
func (vc *viewerCore) drawHint(w io.Writer, hint string, hintVars map[string]string) {
	drawHintBar(w, vc.termRows, vc.termCols, hint, hintVars, vc.style)
}
