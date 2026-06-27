package cmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/input"
	spec "codeberg.org/ubunatic/cati/spec"
	"golang.org/x/term"
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

// ControlSpec describes a single tunable setting loaded from spec/controls.yaml.
type ControlSpec struct {
	Key    string
	Type   string // "int", "bool", "enum"
	Min    int
	Max    int
	Values []string // for enum type
}

// settingsFieldLabel converts a snake_case control key to a Title Case display label.
func settingsFieldLabel(key string) string {
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// applySettingsDelta increments or decrements the field matched by c.Key inside s.
func applySettingsDelta(c ControlSpec, delta int, s *Settings) {
	switch c.Key {
	case "preview_height":
		s.MaxPreviewHeight = max(c.Min, min(c.Max, s.MaxPreviewHeight+delta))
	case "view_mode":
		if len(c.Values) > 0 {
			idx := 0
			for i, v := range c.Values {
				if v == s.ViewMode {
					idx = i
					break
				}
			}
			idx = ((idx+delta)%len(c.Values) + len(c.Values)) % len(c.Values)
			s.ViewMode = c.Values[idx]
		}
	case "preview_videos":
		s.PreviewVideos = delta > 0
	case "max_jobs":
		s.MaxJobs = max(c.Min, min(c.Max, s.MaxJobs+delta))
	case "video_frames":
		s.VideoFrames = max(c.Min, min(c.Max, s.VideoFrames+delta))
	case "video_preview_delay":
		s.VideoPreviewDelay = max(c.Min, min(c.Max, s.VideoPreviewDelay+delta*100))
	}
}

type StyleConfig struct {
	AppBg          string
	AppBorderStyle string
	AppBorderColor string
	BtnFg          string
	BtnBg          string
	BtnBorderColor string
	BtnLeftCap     string
	BtnRightCap    string
	BtnActiveFg    string
	BtnActiveBg    string
	PreviewBg      string
	ControlBarBg   string
	ControlBarFg   string
	// Header bar (top status line)
	HeaderFg   string
	HeaderBg   string
	HeaderBold bool
	// Grid / list item display
	GridItemFg         string
	GridItemBg         string
	GridSelectedFg     string
	GridSelectedBg     string
	GridSelectedBold   bool
	GridSelectedMarker string
	ImageBorder        string
	// Scroll bar
	ScrollThumbChar string
	ScrollRailChar  string
	ScrollWidth     int
	ScrollThumbFg   string
	ScrollRailFg    string
	ScrollRailBg    string
	// Page title (about/settings headers)
	PageTitleFg   string
	PageTitleBold bool
}

var namedColors = map[string]string{
	"black": "#000000", "blk": "#000000",
	"white": "#ffffff", "wht": "#ffffff",
	"red":   "#ff0000",
	"green": "#008000", "grn": "#008000",
	"blue": "#0000ff", "blu": "#0000ff",
	"yellow": "#ffff00", "yel": "#ffff00",
	"orange": "#ffa500", "org": "#ffa500",
	"purple": "#800080", "pur": "#800080",
	"pink": "#ffc0cb", "pnk": "#ffc0cb",
	"cyan": "#00ffff", "cyn": "#00ffff",
	"magenta": "#ff00ff", "mag": "#ff00ff",
	"brown": "#a52a2a", "brn": "#a52a2a",
	"gray": "#808080", "grey": "#808080", "gry": "#808080",
	"navy": "#000080", "nav": "#000080",
	"lime":   "#00ff00",
	"aqua":   "#00ffff",
	"teal":   "#008080",
	"maroon": "#800000",
	"olive":  "#808000",
	"silver": "#c0c0c0", "slv": "#c0c0c0",
}

func parseColor(s string) (color.RGBA, bool) {
	if resolved, ok := namedColors[strings.ToLower(s)]; ok {
		s = resolved
	}
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return color.RGBA{}, false
	}
	r, err := strconv.ParseUint(h[0:2], 16, 8)
	if err != nil {
		return color.RGBA{}, false
	}
	g, err := strconv.ParseUint(h[2:4], 16, 8)
	if err != nil {
		return color.RGBA{}, false
	}
	b, err := strconv.ParseUint(h[4:6], 16, 8)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, true
}

// paletteFG/paletteBG return ANSI sequences for 16-color palette names.
// These adapt to the user's terminal theme (Solarized, Gruvbox, etc.)
// unlike 24-bit hex which is always a fixed color.
var paletteFGCodes = map[string]string{
	"dark":  "\x1b[90m", // bright black / dark gray
	"light": "\x1b[97m", // bright white
}
var paletteBGCodes = map[string]string{
	"dark":  "\x1b[100m",
	"light": "\x1b[107m",
}

func styleFG(s string, def string) string {
	if s == "" {
		return def
	}
	if ansi, ok := paletteFGCodes[strings.ToLower(s)]; ok {
		return ansi
	}
	c, ok := parseColor(s)
	if !ok {
		return def
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func styleBG(s string, def string) string {
	if s == "" {
		return def
	}
	if ansi, ok := paletteBGCodes[strings.ToLower(s)]; ok {
		return ansi
	}
	c, ok := parseColor(s)
	if !ok {
		return def
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

// renderTpl renders a template string with variable substitution and inline styling.
//
// Syntax: literal text mixed with { expr } blocks where expr is:
//
//	key            — value from vars map (falls back to key name if missing)
//	'literal'      — quoted literal string passed through as-is
//	key | mod …    — value with style modifiers applied
//
// Modifiers: bold, dim, italic, underline, or any named/hex color (applied as fg).
// After each styled segment the baseAnsi style is restored automatically.
func renderTpl(tpl string, vars map[string]string, baseAnsi string) string {
	var sb strings.Builder
	i := 0
	for i < len(tpl) {
		open := strings.Index(tpl[i:], "{")
		if open == -1 {
			sb.WriteString(tpl[i:])
			break
		}
		sb.WriteString(tpl[i : i+open])
		i += open + 1
		close := strings.Index(tpl[i:], "}")
		if close == -1 {
			sb.WriteByte('{')
			continue
		}
		expr := tpl[i : i+close]
		i += close + 1

		parts := strings.Split(expr, "|")
		key := strings.TrimSpace(parts[0])
		val := tplResolve(key, vars)

		if len(parts) == 1 {
			sb.WriteString(val)
			continue
		}
		for _, mod := range parts[1:] {
			switch strings.TrimSpace(mod) {
			case "bold":
				sb.WriteString("\x1b[1m")
			case "dim":
				sb.WriteString("\x1b[2m")
			case "italic":
				sb.WriteString("\x1b[3m")
			case "underline":
				sb.WriteString("\x1b[4m")
			default:
				if fg := styleFG(strings.TrimSpace(mod), ""); fg != "" {
					sb.WriteString(fg)
				}
			}
		}
		sb.WriteString(val)
		sb.WriteString("\x1b[m")
		sb.WriteString(baseAnsi)
	}
	return sb.String()
}

// tplResolve returns the value for a template key: unquotes 'literal' / "literal",
// looks up vars, or falls back to the key itself.
func tplResolve(key string, vars map[string]string) string {
	if len(key) >= 2 &&
		((key[0] == '\'' && key[len(key)-1] == '\'') ||
			(key[0] == '"' && key[len(key)-1] == '"')) {
		return key[1 : len(key)-1]
	}
	if vars != nil {
		if v, ok := vars[key]; ok {
			return v
		}
	}
	return key
}

// tplWidth returns the visual (rune) width of a rendered template string,
// counting only the resolved text — not the { } syntax or ANSI escapes.
func tplWidth(tpl string, vars map[string]string) int {
	w := 0
	i := 0
	for i < len(tpl) {
		open := strings.Index(tpl[i:], "{")
		if open == -1 {
			w += utf8.RuneCountInString(tpl[i:])
			break
		}
		w += utf8.RuneCountInString(tpl[i : i+open])
		i += open + 1
		close := strings.Index(tpl[i:], "}")
		if close == -1 {
			break
		}
		expr := tpl[i : i+close]
		i += close + 1
		parts := strings.Split(expr, "|")
		w += utf8.RuneCountInString(tplResolve(strings.TrimSpace(parts[0]), vars))
	}
	return w
}

// fmtHeader is renderTpl specialised for the header bar (kept for call-site clarity).
func fmtHeader(tpl string, vars map[string]string, baseAnsi string) string {
	return renderTpl(tpl, vars, baseAnsi)
}

func styleSelectedAnsi(style *StyleConfig) string {
	s := styleFG(style.GridSelectedFg, "")
	s += styleBG(style.GridSelectedBg, "")
	if style.GridSelectedBold {
		s = "\x1b[1m" + s
	}
	return s
}

func styleItemAnsi(style *StyleConfig) string {
	return styleFG(style.GridItemFg, "") + styleBG(style.GridItemBg, "")
}

func loadStyle() *StyleConfig {
	cfg := &StyleConfig{
		AppBorderStyle:     "box",
		BtnLeftCap:         "[",
		BtnRightCap:        "]",
		GridSelectedBold:   true,
		GridSelectedMarker: " ",
		ImageBorder:        "none",
		ScrollThumbChar:    "█",
		ScrollRailChar:     "▒",
		ScrollWidth:        1,
		ScrollRailBg:       "",
	}

	s, err := spec.LoadStyle()
	if err != nil {
		return cfg
	}

	cfg.AppBg = s.App.Bg
	if s.App.BorderStyle != "" {
		cfg.AppBorderStyle = s.App.BorderStyle
	}
	cfg.AppBorderColor = s.App.BorderColor

	cfg.BtnFg = s.Buttons.Fg
	cfg.BtnBg = s.Buttons.Bg
	cfg.BtnBorderColor = s.Buttons.BorderColor
	if s.Buttons.LeftCap != "" {
		cfg.BtnLeftCap = s.Buttons.LeftCap
	}
	if s.Buttons.RightCap != "" {
		cfg.BtnRightCap = s.Buttons.RightCap
	}
	cfg.BtnActiveFg = s.Buttons.ActiveFg
	cfg.BtnActiveBg = s.Buttons.ActiveBg

	cfg.PreviewBg = s.Preview.Bg

	cfg.ControlBarBg = s.ControlBar.Bg
	cfg.ControlBarFg = s.ControlBar.Fg

	cfg.HeaderFg = s.HeaderBar.Fg
	cfg.HeaderBg = s.HeaderBar.Bg
	cfg.HeaderBold = s.HeaderBar.Bold

	cfg.GridItemFg = s.Grid.ItemFg
	cfg.GridItemBg = s.Grid.ItemBg
	cfg.GridSelectedFg = s.Grid.SelectedFg
	cfg.GridSelectedBg = s.Grid.SelectedBg
	cfg.GridSelectedBold = s.Grid.SelectedBold
	if s.Grid.SelectedMarker != "" {
		cfg.GridSelectedMarker = s.Grid.SelectedMarker
	}
	if s.Grid.ImageBorder == "box" || s.Grid.ImageBorder == "double" || s.Grid.ImageBorder == "none" {
		cfg.ImageBorder = s.Grid.ImageBorder
	}

	if s.ScrollBar.ThumbChar != "" {
		cfg.ScrollThumbChar = s.ScrollBar.ThumbChar
	}
	if s.ScrollBar.RailChar != "" {
		cfg.ScrollRailChar = s.ScrollBar.RailChar
	}
	if s.ScrollBar.Width == 1 || s.ScrollBar.Width == 2 {
		cfg.ScrollWidth = s.ScrollBar.Width
	}
	cfg.ScrollThumbFg = s.ScrollBar.ThumbFg
	cfg.ScrollRailFg = s.ScrollBar.RailFg
	cfg.ScrollRailBg = s.ScrollBar.RailBg

	cfg.PageTitleFg = s.PageTitle.Fg
	cfg.PageTitleBold = s.PageTitle.Bold

	return cfg
}

// ── Customizable Labels ──────────────────────────────────────────────────────

func loadLabels() map[string]string {
	labels := map[string]string{
		"app_name":      "Cati Browser",
		"header":        " {app_name}  [{dir}] — Page {page}/{pages} ({start}-{end} of {total})",
		"folder_icon":   "📁",
		"file_icon":     "📄",
		"hint_browser":  "[Enter/Click] View/Enter  [◀/▶/Scroll] Page  [s] Settings  [m] Toggle Mode  [q] Quit",
		"hint_settings": "[▲/▼] Adjust  [s/Enter] Save  [Esc/q] Cancel",
		"hint_about":    "[q/Esc] Back",
		"hint_viewer":   "[q/Esc] Back  [+/-] Zoom",
	}

	l, err := spec.LoadLabels()
	if err != nil {
		return labels
	}
	for k, v := range l {
		if v != "" {
			labels[k] = v
		}
	}
	return labels
}

// loadButtonActions reads spec/buttons.yaml and returns button_name → action name.
func loadButtonActions() map[string]string {
	actions := map[string]string{}
	btnSpec, err := spec.LoadButtons()
	if err != nil {
		return actions
	}
	for name, def := range btnSpec.Buttons {
		if def.Action != "" {
			actions[name] = def.Action
		}
	}
	return actions
}

// loadAltButtonActions reads spec/buttons.yaml and returns button_name → alt_action.
func loadAltButtonActions() map[string]string {
	actions := map[string]string{}
	btnSpec, err := spec.LoadButtons()
	if err != nil {
		return actions
	}
	for name, def := range btnSpec.Buttons {
		if def.AltAction != "" {
			actions[name] = def.AltAction
		}
	}
	return actions
}

// buttonKeyDef holds the action name and bound key sequences for one button entry.
type buttonKeyDef struct {
	action    string
	keys      []string
	altAction string
	altKeys   []string
}

// loadButtonKeyDefs reads spec/buttons.yaml and returns button_name → {action, keys}.
func loadButtonKeyDefs(inputSpec *input.Spec) map[string]buttonKeyDef {
	defs := map[string]buttonKeyDef{}
	btnSpec, err := spec.LoadButtons()
	if err != nil {
		return defs
	}
	for name, def := range btnSpec.Buttons {
		if def.Action != "" {
			var keys []string
			for _, k := range def.Keys {
				keys = append(keys, inputSpec.ResolveKeyAlias(k))
			}
			var altKeys []string
			for _, k := range def.AltKeys {
				altKeys = append(altKeys, inputSpec.ResolveKeyAlias(k))
			}
			defs[name] = buttonKeyDef{
				action:    def.Action,
				keys:      keys,
				altAction: def.AltAction,
				altKeys:   altKeys,
			}
		}
	}
	return defs
}

// extractViewButtonNames returns all button names referenced in a view row template.
// Handles { name }, { name | mod }, and { if(cond, name1, name2) } forms.
func extractViewButtonNames(tpl string) []string {
	var names []string
	i := 0
	for i < len(tpl) {
		open := strings.Index(tpl[i:], "{")
		if open == -1 {
			break
		}
		i += open + 1
		close := strings.Index(tpl[i:], "}")
		if close == -1 {
			break
		}
		expr := strings.TrimSpace(tpl[i : i+close])
		i += close + 1
		if strings.HasPrefix(expr, "if(") {
			inner := strings.TrimSuffix(strings.TrimPrefix(expr, "if("), ")")
			parts := strings.SplitN(inner, ",", 3)
			if len(parts) == 3 {
				names = append(names, strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]))
			}
		} else {
			name := strings.TrimSpace(strings.SplitN(expr, "|", 2)[0])
			if name != "" && !strings.HasPrefix(name, "'") && !strings.HasPrefix(name, "hint_") {
				names = append(names, name)
			}
		}
	}
	return names
}

// buildViewKeyMaps returns view → key → action, derived from which buttons each view shows.
// Context is correct: pressing <esc> in browser triggers go_back; in settings it triggers cancel_settings.
func buildViewKeyMaps(viewBtnRows map[string]string, defs map[string]buttonKeyDef) map[string]map[string]string {
	result := map[string]map[string]string{}
	for viewName, tpl := range viewBtnRows {
		km := map[string]string{}
		for _, btnName := range extractViewButtonNames(tpl) {
			if def, ok := defs[btnName]; ok {
				for _, k := range def.keys {
					km[k] = def.action
				}
				if def.altAction != "" {
					for _, k := range def.altKeys {
						km[k] = def.altAction
					}
				}
			}
		}
		result[viewName] = km
	}
	return result
}

// openWebsite opens url in the system default browser.
func openWebsite(url string) {
	if url == "" {
		return
	}
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"open", url}
	case "windows":
		args = []string{"cmd", "/c", "start", url}
	default:
		args = []string{"xdg-open", url}
	}
	// Fire-and-forget; ignore errors (no terminal to report them).
	_ = exec.Command(args[0], args[1:]...).Start() //nolint
}

// loadButtons reads spec/buttons.yaml and returns key → rendered label (caps applied).
func loadButtons(leftCap, rightCap string) map[string]string {
	wrap := func(text string) string { return leftCap + text + rightCap }
	buttons := map[string]string{}
	btnSpec, err := spec.LoadButtons()
	if err != nil {
		return buttons
	}
	for name, def := range btnSpec.Buttons {
		if def.Text != "" {
			buttons[name] = wrap(def.Text)
		}
	}
	return buttons
}

// ── YAML view parser ─────────────────────────────────────────────────────────

type YamlView struct {
	Type     string
	Name     string
	Title    string
	Content  string
	Controls []string
}

func parseYamlView(name string) (*YamlView, error) {
	v, err := spec.LoadYamlView(name)
	if err != nil {
		return nil, err
	}
	return &YamlView{
		Type:     v.Type,
		Name:     v.Name,
		Title:    v.Title,
		Content:  v.Content,
		Controls: v.Controls,
	}, nil
}

func getAboutView() *YamlView {
	view, err := parseYamlView("about.yaml")
	if err == nil && view != nil {
		return view
	}
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

type Settings struct {
	MaxPreviewHeight  int
	ViewMode          string
	PreviewVideos     bool
	MaxJobs           int
	VideoFrames       int
	VideoPreviewDelay int // milliseconds; 0 = immediate
}

func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "cati")
}

func loadSpecConfigDefaults() Settings {
	cfg := Settings{MaxPreviewHeight: 20, ViewMode: "grid", PreviewVideos: true, MaxJobs: 0, VideoFrames: 10, VideoPreviewDelay: 1000}
	s, err := spec.LoadConfigDefaults()
	if err != nil {
		return cfg
	}
	if s.Config.PreviewHeight > 0 {
		cfg.MaxPreviewHeight = s.Config.PreviewHeight
	}
	if s.Config.ViewMode == "preview" || s.Config.ViewMode == "grid" {
		cfg.ViewMode = s.Config.ViewMode
	}
	cfg.PreviewVideos = s.Config.PreviewVideos
	if s.Config.MaxJobs >= 0 {
		cfg.MaxJobs = s.Config.MaxJobs
	}
	if s.Config.VideoFrames > 0 {
		cfg.VideoFrames = s.Config.VideoFrames
	}
	if s.Config.VideoPreviewDelay >= 0 {
		cfg.VideoPreviewDelay = s.Config.VideoPreviewDelay
	}
	return cfg
}

func loadConfig() Settings {
	cfg := loadSpecConfigDefaults()
	dir := getConfigDir()
	if dir == "" {
		return cfg
	}
	path := filepath.Join(dir, "config")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "max_preview_height", "height":
				if h, err := strconv.Atoi(val); err == nil && h > 0 {
					cfg.MaxPreviewHeight = h
				}
			case "view_mode":
				if val == "preview" || val == "grid" {
					cfg.ViewMode = val
				}
			case "preview_videos":
				cfg.PreviewVideos = val != "false"
			case "max_jobs":
				if n, err := strconv.Atoi(val); err == nil && n >= 0 {
					cfg.MaxJobs = n
				}
			case "video_frames":
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					cfg.VideoFrames = n
				}
			case "video_preview_delay":
				if n, err := strconv.Atoi(val); err == nil && n >= 0 {
					cfg.VideoPreviewDelay = n
				}
			}
		}
	}
	return cfg
}

func saveConfig(cfg Settings) error {
	dir := getConfigDir()
	if dir == "" {
		return fmt.Errorf("could not determine home dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "config")
	previewVideos := "true"
	if !cfg.PreviewVideos {
		previewVideos = "false"
	}
	content := fmt.Sprintf(
		"max_preview_height=%d\nview_mode=%s\npreview_videos=%s\nmax_jobs=%d\nvideo_frames=%d\nvideo_preview_delay=%d\n",
		cfg.MaxPreviewHeight, cfg.ViewMode, previewVideos, cfg.MaxJobs, cfg.VideoFrames, cfg.VideoPreviewDelay,
	)
	return os.WriteFile(path, []byte(content), 0o644)
}

func loadControls() []ControlSpec {
	specs := []ControlSpec{
		{Key: "preview_height", Type: "int", Min: 10, Max: 200},
		{Key: "view_mode", Type: "enum", Values: []string{"grid", "preview"}},
		{Key: "preview_videos", Type: "bool"},
		{Key: "max_jobs", Type: "int", Min: 1, Max: 32},
		{Key: "video_frames", Type: "int", Min: 1, Max: 60},
		{Key: "video_preview_delay", Type: "int", Min: 0, Max: 5000},
	}
	cSpec, err := spec.LoadControls()
	if err != nil {
		return specs
	}
	for i, s := range specs {
		if def, ok := cSpec.Controls[s.Key]; ok {
			if def.Min != 0 || def.Max != 0 {
				specs[i].Min = def.Min
				specs[i].Max = def.Max
			}
			if len(def.Values) > 0 {
				specs[i].Values = def.Values
			}
		}
	}
	return specs
}

// ── Dynamic Directory Loading ───────────────────────────────────────────────

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

func browser(args []string, initWidth, initHeight int, rc renderCfg, fullComp bool, initialZoom string) error {
	cfg := loadConfig()
	inputSpec, _ := input.Load(fs.FS(spec.FS))
	style := loadStyle()
	labels := loadLabels()
	for k, v := range loadButtons(style.BtnLeftCap, style.BtnRightCap) {
		labels[k] = v
	}
	viewBtnRows := loadViewButtonRows()
	viewKeyRows := loadViewKeyRows()
	controls := loadControls()
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
	if nWorkers <= 0 {
		nWorkers = max(1, runtime.NumCPU()/2)
	}
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
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
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
			drawAboutPage(os.Stdout, termCols, effHeight, style)
			buttons = drawBottomMenu(os.Stdout, effHeight, "about", hoveredButtonAction, style, labels, viewBtnRows, nil, btnActions, nil)
			drawHintBar(os.Stdout, effHeight, labels["hint_about"], map[string]string{"last_key": lastKey}, style)
			return
		}

		if viewMode == "settings" {
			drawSettingsPage(os.Stdout, termCols, effHeight, controls, tempCfg, activeSettingsField, labels)
			buttons = drawBottomMenu(os.Stdout, effHeight, "settings", hoveredButtonAction, style, labels, viewBtnRows, nil, btnActions, nil)
			activeSetting := ""
			if activeSettingsField >= 0 && activeSettingsField < len(controls) {
				activeSetting = settingsFieldLabel(controls[activeSettingsField].Key)
			}
			drawHintBar(os.Stdout, effHeight, labels["hint_settings"], map[string]string{"active_setting": activeSetting, "last_key": lastKey}, style)
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

		pixScale := 1
		if rc.mode.useQuad() {
			pixScale = 2
		}
		compW := termCols * pixScale
		compH := gridRowsLimit * 2
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
					offsetX := (prevW*pixScale - scaledW) / 2
					offsetY := (gridRowsLimit*2 - scaledH) / 2
					destX := (leftW+2)*pixScale + offsetX
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
				left := colIdx * (cellW + gapX) * pixScale
				top := rowIdx * (cellH + gapY)

				thumb := getThumbnail(items[idx], cellW, cellH-1)
				if thumb != nil {
					thumbW := thumb.Bounds().Dx()
					thumbH := thumb.Bounds().Dy()
					offsetX := (cellW*pixScale - thumbW) / 2
					offsetY := ((cellH-1)*2 - thumbH) / 2
					destX := left + offsetX
					destY := top*2 + offsetY
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
		_ = rc.render(os.Stdout, compImg)
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
		buttons = drawBottomMenu(os.Stdout, effHeight, "grid", hoveredButtonAction, style, labels, viewBtnRows, nil, btnActions, nil)

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
				currentMeta.DispMode = "half"
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
		drawHintBar(os.Stdout, effHeight, labels["hint_browser"], hintVars, style)
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

										if halfblock.IsVideo(targetItem.path) {
											_ = interactiveVideo(targetItem.path, initWidth, initHeight, rc, inputs, style, labels, viewBtnRows, viewKeyMaps, inputSpec, fullComp, initialZoom)
										} else {
											_ = interactiveWithChan(targetItem.path, initWidth, initHeight, rc, inputs, style, labels, viewBtnRows, viewKeyMaps, inputSpec, fullComp, initialZoom)
										}

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

							if halfblock.IsVideo(targetItem.path) {
								_ = interactiveVideo(targetItem.path, initWidth, initHeight, rc, inputs, style, labels, viewBtnRows, viewKeyMaps, inputSpec, fullComp, initialZoom)
							} else {
								_ = interactiveWithChan(targetItem.path, initWidth, initHeight, rc, inputs, style, labels, viewBtnRows, viewKeyMaps, inputSpec, fullComp, initialZoom)
							}

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

func drawAboutPage(w io.Writer, termCols, termRows int, style *StyleConfig) {
	halfblock.ClearScreen(w)

	about := getAboutView()
	lines := strings.Split(about.Content, "\n")

	titleAnsi := styleFG(style.PageTitleFg, "\x1b[36m")
	if style.PageTitleBold {
		titleAnsi = "\x1b[1m" + titleAnsi
	}
	titleLine := "=== " + about.Title + " ==="
	fmt.Fprintf(w, "\x1b[2;%dH%s%s\x1b[m", max(1, (termCols-len(titleLine))/2), titleAnsi, titleLine)

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

func drawSettingsPage(w io.Writer, termCols, termRows int, controls []ControlSpec, temp Settings, activeField int, labels map[string]string) {
	halfblock.ClearScreen(w)

	type field struct{ label, value string }
	fields := make([]field, len(controls))
	for i, c := range controls {
		var value string
		switch c.Key {
		case "preview_height":
			value = fmt.Sprintf("%d rows", temp.MaxPreviewHeight)
		case "view_mode":
			value = temp.ViewMode
		case "preview_videos":
			if temp.PreviewVideos {
				value = "true"
			} else {
				value = "false"
			}
		case "max_jobs":
			value = fmt.Sprintf("%d", temp.MaxJobs)
		case "video_frames":
			value = fmt.Sprintf("%d", temp.VideoFrames)
		default:
			value = "?"
		}
		fields[i] = field{settingsFieldLabel(c.Key), value}
	}

	lbl := func(key, fallback string) string {
		if v := labels[key]; v != "" {
			return v
		}
		return fallback
	}
	title := lbl("settings_title", "CATI SETTINGS")
	sep := strings.Repeat("=", max(len(title)+4, 33))
	centeredTitle := fmt.Sprintf("%*s%s", (len(sep)-len(title))/2, "", title)
	lines := []string{sep, centeredTitle, sep, ""}
	for i, f := range fields {
		if i == activeField {
			lines = append(lines, fmt.Sprintf("  > %-16s [ %s ]  (Selected)", f.label+":", f.value))
		} else {
			lines = append(lines, fmt.Sprintf("    %-16s [ %s ]", f.label+":", f.value))
		}
	}
	lines = append(lines,
		"",
		"  "+lbl("settings_hint_tab", "Press Tab to switch active setting."),
		"  "+lbl("settings_hint_adjust", "Use ↑ / ↓ to change value."),
		"  "+lbl("settings_hint_save", "Press Enter to Save, Esc to Cancel."),
		"",
		sep,
	)

	for i, line := range lines {
		row := (termRows-len(lines))/2 + i
		col := (termCols - len(line)) / 2
		if col < 1 {
			col = 1
		}
		fmt.Fprintf(w, "\x1b[%d;%dH\x1b[K%s", row, col, line)
	}
}

// loadViewButtonRows reads the visible button-bar row template for each view from spec/views.yaml.
// Only `row:` entries are included; `hidden_keys:` entries are excluded (use loadViewKeyRows for those).
func loadViewButtonRows() map[string]string {
	return loadViewRowYaml(false)
}

// loadViewKeyRows reads both visible and hidden button rows for key-map building.
// Includes both `row:` and `hidden_keys:` entries concatenated per view.
func loadViewKeyRows() map[string]string {
	return loadViewRowYaml(true)
}

func loadViewRowYaml(includeHidden bool) map[string]string {
	views, err := spec.LoadViews()
	if err != nil {
		return map[string]string{}
	}
	result := map[string]string{}
	for viewName, rows := range views.Views {
		var visible []string
		var hidden []string
		for _, row := range rows {
			if tpl := row["row"]; tpl != "" && !strings.Contains(tpl, "hint_") && len(visible) == 0 {
				visible = append(visible, tpl)
			}
			if includeHidden {
				if tpl := row["hidden_keys"]; tpl != "" {
					hidden = append(hidden, tpl)
				}
			}
		}
		if len(visible) == 0 && len(hidden) == 0 {
			continue
		}
		if includeHidden {
			result[viewName] = strings.TrimSpace(strings.Join(append(visible, hidden...), " "))
			continue
		}
		if len(visible) > 0 {
			result[viewName] = visible[0]
		}
	}
	return result
}

// drawHintBar renders the hint text line (last terminal row) for the current view.
func drawHintBar(w io.Writer, termRow int, label string, vars map[string]string, style *StyleConfig) {
	ctrlAnsi := styleBG(style.ControlBarBg, "") + styleFG(style.ControlBarFg, "")
	text := renderTpl(label, vars, ctrlAnsi)
	fmt.Fprintf(w, "\x1b[%d;1H\x1b[K%s %s \x1b[m", termRow, ctrlAnsi, text)
}

// drawBottomMenu renders the button bar for the given view using the row template from views.yaml.
// conditions is an optional map of runtime boolean flags (e.g. "playing") used by if() expressions.
// btnActions maps button key names to their registered action names (from spec/buttons.yaml); nil means use key name as action.
func drawBottomMenu(w io.Writer, termRows int, viewMode string, activeAction string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, conditions map[string]bool, btnActions map[string]string, altBtnActions map[string]string) []menuButton {
	if style == nil {
		style = loadStyle()
	}

	viewName := viewMode
	if viewMode == "grid" || viewMode == "preview" {
		viewName = "browser"
	}

	ctrlAnsi := styleBG(style.ControlBarBg, "") + styleFG(style.ControlBarFg, "")
	fmt.Fprintf(w, "\x1b[%d;1H\x1b[K%s", termRows-1, ctrlAnsi)

	tpl := viewBtnRows[viewName]
	if tpl == "" {
		return nil
	}

	var buttons []menuButton
	col := 1
	i := 0
	for i < len(tpl) {
		open := strings.Index(tpl[i:], "{")
		if open == -1 {
			break
		}
		// Render literal content between buttons (e.g. " | " separators)
		literal := tpl[i : i+open]
		if strings.TrimSpace(literal) != "" {
			fmt.Fprintf(w, "\x1b[%d;%dH%s%s\x1b[m", termRows-1, col, ctrlAnsi, literal)
		}
		col += utf8.RuneCountInString(literal)
		i += open + 1

		close := strings.Index(tpl[i:], "}")
		if close == -1 {
			break
		}
		expr := tpl[i : i+close]
		i += close + 1

		parts := strings.Split(expr, "|")
		key := strings.TrimSpace(parts[0])
		mods := parts[1:]

		// if(cond, trueKey, falseKey) — resolve at render time using conditions map
		if strings.HasPrefix(key, "if(") && strings.HasSuffix(key, ")") {
			inner := key[3 : len(key)-1]
			ifParts := strings.SplitN(inner, ",", 3)
			if len(ifParts) == 3 {
				cond := strings.TrimSpace(ifParts[0])
				if conditions[cond] {
					key = strings.TrimSpace(ifParts[1])
				} else {
					key = strings.TrimSpace(ifParts[2])
				}
			}
		}

		label := labels[key]
		if label == "" {
			label = key
		}

		action := key
		if a, ok := btnActions[key]; ok {
			action = a
		}
		altAction := altBtnActions[key]
		btn := menuButton{label: label, action: action, altAction: altAction, col: col, width: tplWidth(label, nil)}
		buttons = append(buttons, btn)

		fg := styleFG(style.BtnFg, "")
		bg := styleBG(style.BtnBg, "")
		boldEsc := ""
		for _, mod := range mods {
			switch strings.TrimSpace(mod) {
			case "bold":
				boldEsc = "\x1b[1m"
			default:
				if c := styleFG(strings.TrimSpace(mod), ""); c != "" {
					fg = c
				}
			}
		}
		if action == activeAction {
			boldEsc = "\x1b[1m"
			fg = styleFG(style.BtnActiveFg, fg)
			bg = styleBG(style.BtnActiveBg, bg)
		}
		baseAnsi := boldEsc + fg + bg
		rendered := renderTpl(label, nil, baseAnsi)
		fmt.Fprintf(w, "\x1b[%d;%dH%s%s\x1b[m", termRows-1, col, baseAnsi, rendered)
		col += btn.width
	}
	return buttons
}
