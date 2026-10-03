package cmd

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"ubunatic.com/cati/v1/halfblock"
)

// lineCapWriter wraps an io.Writer and records the maximum visible column
// reached across all output lines. It decodes ANSI CSI escape sequences
// (skipping them without counting) and counts visible UTF-8 runes.
// Used for asserting that no output line exceeds a terminal column budget.
type lineCapWriter struct {
	w      io.Writer
	col    int
	MaxCol int
}

func (lc *lineCapWriter) Write(p []byte) (int, error) {
	s := string(p)
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) {
					c := s[i]
					i++
					if c >= 0x40 && c <= 0x7e {
						break
					}
				}
				continue
			}
			continue
		}
		if s[i] == '\n' || s[i] == '\r' {
			if lc.col > lc.MaxCol {
				lc.MaxCol = lc.col
			}
			lc.col = 0
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		lc.col++
		i += size
	}
	if lc.col > lc.MaxCol {
		lc.MaxCol = lc.col
	}
	if lc.w != nil {
		return lc.w.Write(p)
	}
	return len(p), nil
}

func (lc *lineCapWriter) AssertFits(t *testing.T, budget int) {
	t.Helper()
	if lc.MaxCol > budget {
		t.Errorf("output exceeded %d columns: max col = %d", budget, lc.MaxCol)
	}
}

func testStyleForLinecap() *StyleConfig { return loadStyle() }
func testLabelsForLinecap() map[string]string {
	style := testStyleForLinecap()
	labels := loadLabels()
	for k, v := range loadButtons(style.BtnLeftCap, style.BtnRightCap) {
		labels[k] = v
	}
	return labels
}

func TestHintBarAndBottomBarUseStyleNotHardcodedReverseVideo(t *testing.T) {
	style := testStyleForLinecap()
	labels := testLabelsForLinecap()
	hint := labels["hint_viewer"]
	vars := map[string]string{
		"meta.name":       "test.png",
		"meta.name_short": "test.png",
		"meta.ext":        "png",
		"meta.src_res":    "1920×1080",
		"meta.disp_res":   "80×22",
		"render_mode":     "halfblock",
		"zoom_level":      "src px/cell=1",
		"ssim":            "0.950",
		"blockiness":      "0.010",
		"edge_cont":       "0.800",
		"last_key":        "r",
	}

	// hint bar
	var hintBuf bytes.Buffer
	drawHintBar(&hintBuf, 24, 80, hint, vars, style)
	if out := hintBuf.String(); strings.Contains(out, "\x1b[7m") {
		t.Error("hint bar output contains hardcoded reverse-video escape \\x1b[7m; use style.yaml control_bar colors instead")
	}

	// button bar for all views
	rows := loadViewButtonRows()
	btnActions := loadButtonActions()
	for _, view := range []string{"image_viewer", "video_player", "browser", "settings", "about"} {
		var btnBuf bytes.Buffer
		drawBottomMenu(&btnBuf, 24, 80, view, "", style, labels, rows, nil, btnActions, nil)
		if out := btnBuf.String(); strings.Contains(out, "\x1b[7m") {
			t.Errorf("button bar for view %q contains hardcoded reverse-video escape \\x1b[7m; use style.yaml colors instead", view)
		}
	}
}

func TestLineWidthHintBar80(t *testing.T) {
	style := testStyleForLinecap()
	labels := testLabelsForLinecap()
	hint := labels["hint_viewer"]
	vars := map[string]string{
		"meta.name":       "test.png",
		"meta.name_short": "test.png",
		"meta.ext":        "png",
		"meta.src_res":    "1920×1080",
		"meta.disp_res":   "80×22",
		"render_mode":     "halfblock",
		"zoom_level":      "src px/cell=1",
		"ssim":            "0.950",
		"blockiness":      "0.010",
		"edge_cont":       "0.800",
		"last_key":        "r",
	}
	lc := &lineCapWriter{}
	drawHintBar(lc, 24, 80, hint, vars, style)
	lc.AssertFits(t, 80)
}

func TestLineWidthButtonBar80(t *testing.T) {
	style := testStyleForLinecap()
	labels := testLabelsForLinecap()
	rows := loadViewButtonRows()
	btnActions := loadButtonActions()

	for _, view := range []string{"image_viewer", "video_player", "browser"} {
		t.Run(view, func(t *testing.T) {
			lc := &lineCapWriter{}
			drawBottomMenu(lc, 24, 80, view, "", style, labels, rows, nil, btnActions, nil)
			lc.AssertFits(t, 80)
		})
	}
}

func TestLineWidthRenderAllModes(t *testing.T) {
	src, err := halfblock.LoadImage("../testdata/gradient_32x32.png")
	if err != nil {
		t.Skipf("testdata/gradient_32x32.png not found: %v", err)
	}
	const termCols, termRows = 80, 24
	for _, m := range renderModes {
		t.Run(m.name, func(t *testing.T) {
			state := viewState{zoom: 1.0}
			vp := buildViewport(src, &state, termCols, termRows-2, m.cfg)
			lc := &lineCapWriter{}
			if err := m.cfg.render(lc, vp); err != nil {
				t.Fatalf("render(%s): %v", m.name, err)
			}
			lc.AssertFits(t, termCols)
		})
	}
}

func TestLineWidthRenderVideoSized(t *testing.T) {
	// Synthesize a 320×240 frame (common small video size).
	frame := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			frame.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	const termCols, termRows = 80, 24
	rc := renderCfg{id: 0}
	state := viewState{zoom: 1.0}
	vp := buildViewport(frame, &state, termCols, termRows-2, rc)
	lc := &lineCapWriter{}
	if err := rc.render(lc, vp); err != nil {
		t.Fatalf("render: %v", err)
	}
	lc.AssertFits(t, termCols)
}
