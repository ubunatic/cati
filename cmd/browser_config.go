package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"ubunatic.com/cati/internal/input"
	spec "ubunatic.com/cati/spec"
)

// ControlSpec describes a single tunable setting loaded from spec/controls.yaml.
type ControlSpec struct {
	Key    string
	Type   string // "int", "bool", "enum"
	Min    int
	Max    int
	Values []string // for enum type
	Set    string
	Get    string
}

type controlHandler struct {
	typeName string
	get      func(Settings) string
	set      func(ControlSpec, int, *Settings)
}

var controlHandlers = func() map[string]controlHandler {
	byControl := map[string]controlHandler{
		"preview_height": {typeName: "int", get: func(s Settings) string { return fmt.Sprintf("%d rows", s.MaxPreviewHeight) }, set: func(c ControlSpec, d int, s *Settings) {
			s.MaxPreviewHeight = max(c.Min, min(c.Max, s.MaxPreviewHeight+d))
		}},
		"view_mode": {typeName: "enum", get: func(s Settings) string { return s.ViewMode }, set: func(c ControlSpec, d int, s *Settings) {
			if len(c.Values) == 0 {
				return
			}
			i := 0
			for n, v := range c.Values {
				if v == s.ViewMode {
					i = n
					break
				}
			}
			i = ((i+d)%len(c.Values) + len(c.Values)) % len(c.Values)
			s.ViewMode = c.Values[i]
		}},
		"preview_videos": {typeName: "bool", get: func(s Settings) string { return strconv.FormatBool(s.PreviewVideos) }, set: func(_ ControlSpec, d int, s *Settings) { s.PreviewVideos = d > 0 }},
		"max_jobs":       {typeName: "int", get: func(s Settings) string { return strconv.Itoa(s.MaxJobs) }, set: func(c ControlSpec, d int, s *Settings) { s.MaxJobs = max(c.Min, min(c.Max, s.MaxJobs+d)) }},
		"video_frames":   {typeName: "int", get: func(s Settings) string { return strconv.Itoa(s.VideoFrames) }, set: func(c ControlSpec, d int, s *Settings) { s.VideoFrames = max(c.Min, min(c.Max, s.VideoFrames+d)) }},
		"video_preview_delay": {typeName: "int", get: func(s Settings) string { return strconv.Itoa(s.VideoPreviewDelay) }, set: func(c ControlSpec, d int, s *Settings) {
			// Settings navigation uses 100 ms input steps; allowed bounds come from controls.yaml.
			s.VideoPreviewDelay = max(c.Min, min(c.Max, s.VideoPreviewDelay+d*100))
		}},
	}
	handlers := make(map[string]controlHandler, len(byControl)*2)
	for key, handler := range byControl {
		handlers["set_"+key] = handler
		handlers["get_"+key] = handler
	}
	return handlers
}()

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

func notifySignals(ch chan<- os.Signal, signals []os.Signal) bool {
	if len(signals) == 0 {
		return false
	}
	signal.Notify(ch, signals...)
	return true
}

// applySettingsDelta increments or decrements the field matched by c.Key inside s.
func applySettingsDelta(c ControlSpec, delta int, s *Settings) {
	if h, ok := controlHandlers[c.Set]; ok && h.set != nil {
		h.set(c, delta, s)
	}
}

func validateControlHandlers(controls []ControlSpec, handlers map[string]controlHandler) error {
	for _, c := range controls {
		h, ok := handlers[c.Set]
		if !ok || h.set == nil {
			return fmt.Errorf("control %s has no Go setter handler %q", c.Key, c.Set)
		}
		if h.typeName != c.Type {
			return fmt.Errorf("control %s type %q does not match setter handler type %q", c.Key, c.Type, h.typeName)
		}
		h, ok = handlers[c.Get]
		if !ok || h.get == nil {
			return fmt.Errorf("control %s has no Go getter handler %q", c.Key, c.Get)
		}
	}
	return nil
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

// loadStyle loads colors/structure from spec/style.yaml.
// FIXME(#004-item-C): spec.LoadTheme() tokens exist but are never applied.
// spec/buttons.yaml's style: fields (danger, primary, etc.) are ignored.
func loadStyle() (*StyleConfig, error) {
	return loadStyleFrom(spec.FS)
}

func loadStyleFrom(fsys fs.FS) (*StyleConfig, error) {
	s, err := spec.LoadStyleFrom(fsys)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &StyleConfig{}, nil
		}
		return nil, fmt.Errorf("load style spec: %w", err)
	}
	return styleConfigFromSpec(s), nil
}

func styleConfigFromSpec(s spec.StyleSpec) *StyleConfig {
	return &StyleConfig{
		AppBg: s.App.Bg, AppBorderStyle: s.App.BorderStyle, AppBorderColor: s.App.BorderColor,
		BtnFg: s.Buttons.Fg, BtnBg: s.Buttons.Bg, BtnBorderColor: s.Buttons.BorderColor,
		BtnLeftCap: s.Buttons.LeftCap, BtnRightCap: s.Buttons.RightCap,
		BtnActiveFg: s.Buttons.ActiveFg, BtnActiveBg: s.Buttons.ActiveBg,
		PreviewBg: s.Preview.Bg, ControlBarBg: s.ControlBar.Bg, ControlBarFg: s.ControlBar.Fg,
		HeaderFg: s.HeaderBar.Fg, HeaderBg: s.HeaderBar.Bg, HeaderBold: s.HeaderBar.Bold,
		GridItemFg: s.Grid.ItemFg, GridItemBg: s.Grid.ItemBg,
		GridSelectedFg: s.Grid.SelectedFg, GridSelectedBg: s.Grid.SelectedBg,
		GridSelectedBold: s.Grid.SelectedBold, GridSelectedMarker: s.Grid.SelectedMarker,
		ImageBorder:     s.Grid.ImageBorder,
		ScrollThumbChar: s.ScrollBar.ThumbChar, ScrollRailChar: s.ScrollBar.RailChar,
		ScrollWidth: s.ScrollBar.Width, ScrollThumbFg: s.ScrollBar.ThumbFg,
		ScrollRailFg: s.ScrollBar.RailFg, ScrollRailBg: s.ScrollBar.RailBg,
		PageTitleFg: s.PageTitle.Fg, PageTitleBold: s.PageTitle.Bold,
	}
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

func loadButtonPriorities() map[string]int {
	prios := map[string]int{}
	btnSpec, err := spec.LoadButtons()
	if err != nil {
		return prios
	}
	for name, def := range btnSpec.Buttons {
		prios[name] = def.Prio
	}
	return prios
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
		open := strings.IndexByte(tpl[i:], '{')
		if open == -1 {
			break
		}
		i += open + 1
		close := strings.IndexByte(tpl[i:], '}')
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

func getAboutView() (*YamlView, error) {
	return aboutViewFrom(spec.FS)
}

func aboutViewFrom(fsys fs.FS) (*YamlView, error) {
	v, err := spec.LoadYamlViewFrom(fsys, "about.yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return &YamlView{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load about spec: %w", err)
	}
	return &YamlView{Type: v.Type, Name: v.Name, Title: v.Title, Content: v.Content, Controls: v.Controls}, nil
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

func settingsFromConfigDefaults(c spec.ConfigDef) Settings {
	return Settings{
		MaxPreviewHeight:  c.PreviewHeight,
		ViewMode:          c.ViewMode,
		PreviewVideos:     c.PreviewVideos,
		MaxJobs:           c.MaxJobs,
		VideoFrames:       c.VideoFrames,
		VideoPreviewDelay: c.VideoPreviewDelay,
	}
}

func loadSpecConfigDefaults() (Settings, error) {
	s, err := spec.LoadConfigDefaults()
	if err != nil {
		return Settings{}, err
	}
	return settingsFromConfigDefaults(s.Config), nil
}

func loadConfig() (Settings, error) {
	cfg, err := loadSpecConfigDefaults()
	if err != nil {
		return Settings{}, fmt.Errorf("load config defaults: %w", err)
	}
	dir := getConfigDir()
	if dir == "" {
		return cfg, nil
	}
	path := filepath.Join(dir, "config")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil
	}
	return applyConfigOverrides(cfg, data), nil
}

func applyConfigOverrides(cfg Settings, data []byte) Settings {
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

func loadControls() ([]ControlSpec, error) {
	cSpec, err := spec.LoadControls()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("load controls spec: %w", err)
	}
	controls := controlsFromSpec(cSpec)
	if err := validateControlHandlers(controls, controlHandlers); err != nil {
		return nil, err
	}
	return controls, nil
}

func controlsFromSpec(cSpec spec.ControlsSpec) []ControlSpec {
	controls := make([]ControlSpec, 0, len(cSpec.Order))
	for _, key := range cSpec.Order {
		def := cSpec.Controls[key]
		controls = append(controls, ControlSpec{Key: key, Type: def.Type, Min: def.Min, Max: def.Max, Values: def.Values, Set: def.Set, Get: def.Get})
	}
	return controls
}

// ── Dynamic Directory Loading ───────────────────────────────────────────────
