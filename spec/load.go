package spec

import (
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── Style Spec ───────────────────────────────────────────────────────────────

type StyleApp struct {
	Bg          string `yaml:"bg"`
	BorderStyle string `yaml:"border_style"`
	BorderColor string `yaml:"border_color"`
}

type StyleButtons struct {
	Fg          string `yaml:"fg"`
	Bg          string `yaml:"bg"`
	BorderColor string `yaml:"border_color"`
	LeftCap     string `yaml:"left_cap"`
	RightCap    string `yaml:"right_cap"`
	ActiveFg    string `yaml:"active_fg"`
	ActiveBg    string `yaml:"active_bg"`
}

type StylePreview struct {
	Bg string `yaml:"bg"`
}

type StyleControlBar struct {
	Bg string `yaml:"bg"`
	Fg string `yaml:"fg"`
}

type StyleHeaderBar struct {
	Fg   string `yaml:"fg"`
	Bg   string `yaml:"bg"`
	Bold bool   `yaml:"bold"`
}

type StyleGrid struct {
	ItemFg         string `yaml:"item_fg"`
	ItemBg         string `yaml:"item_bg"`
	SelectedFg     string `yaml:"selected_fg"`
	SelectedBg     string `yaml:"selected_bg"`
	SelectedBold   bool   `yaml:"selected_bold"`
	SelectedMarker string `yaml:"selected_marker"`
	ImageBorder    string `yaml:"image_border"`
}

type StyleScrollBar struct {
	ThumbChar string `yaml:"thumb_char"`
	RailChar  string `yaml:"rail_char"`
	Width     int    `yaml:"width"`
	ThumbFg   string `yaml:"thumb_fg"`
	RailFg    string `yaml:"rail_fg"`
	RailBg    string `yaml:"rail_bg"`
}

type StylePageTitle struct {
	Fg   string `yaml:"fg"`
	Bold bool   `yaml:"bold"`
}

type StyleSpec struct {
	App        StyleApp        `yaml:"app"`
	Buttons    StyleButtons    `yaml:"buttons"`
	Preview    StylePreview    `yaml:"preview"`
	ControlBar StyleControlBar `yaml:"control_bar"`
	HeaderBar  StyleHeaderBar  `yaml:"header_bar"`
	Grid       StyleGrid       `yaml:"grid"`
	ScrollBar  StyleScrollBar  `yaml:"scroll_bar"`
	PageTitle  StylePageTitle  `yaml:"page_title"`
}

func LoadStyle() (StyleSpec, error) {
	var spec StyleSpec
	data, err := fs.ReadFile(FS, "style.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Theme Spec ───────────────────────────────────────────────────────────────

type ThemeToken struct {
	Fg   string `yaml:"fg"`
	Bg   string `yaml:"bg"`
	Bold bool   `yaml:"bold"`
}

type ThemeSpec map[string]ThemeToken

func LoadTheme() (ThemeSpec, error) {
	var spec ThemeSpec
	data, err := fs.ReadFile(FS, "theme.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Labels Spec ──────────────────────────────────────────────────────────────

func LoadLabels() (map[string]string, error) {
	var spec map[string]string
	data, err := fs.ReadFile(FS, "labels.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Buttons Spec ─────────────────────────────────────────────────────────────

type ButtonDef struct {
	Text      string   `yaml:"text"`
	Style     string   `yaml:"style"`
	Prio      int      `yaml:"prio"`
	Action    string   `yaml:"action"`
	Keys      []string `yaml:"keys"`
	AltAction string   `yaml:"alt_action"`
	AltKeys   []string `yaml:"alt_keys"`
}

type ButtonsSpec struct {
	Buttons map[string]ButtonDef `yaml:"buttons"`
}

func LoadButtons() (ButtonsSpec, error) {
	var spec ButtonsSpec
	data, err := fs.ReadFile(FS, "buttons.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Views Spec ───────────────────────────────────────────────────────────────

type ViewRow map[string]string

type ViewsSpec struct {
	Views map[string][]ViewRow `yaml:"views"`
}

func LoadViews() (ViewsSpec, error) {
	var spec ViewsSpec
	data, err := fs.ReadFile(FS, "views.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Zoom Levels Spec ─────────────────────────────────────────────────────────

type ZoomLevelsDef struct {
	Levels             string `yaml:"levels"`
	Extend             string `yaml:"extend"`
	UpscaleSmallImages bool   `yaml:"upscale_small_images"`
}

type ZoomLevelsSpec struct {
	Levels             []float64
	Extend             string
	UpscaleSmallImages bool
}

func LoadZoomLevels() (ZoomLevelsSpec, error) {
	def := ZoomLevelsDef{
		UpscaleSmallImages: true,
	}
	var spec ZoomLevelsSpec
	data, err := fs.ReadFile(FS, "zoom_levels.yaml")
	if err != nil {
		return spec, err
	}
	if err = yaml.Unmarshal(data, &def); err != nil {
		return spec, err
	}
	spec.Extend = def.Extend
	spec.UpscaleSmallImages = def.UpscaleSmallImages

	parts := strings.Split(def.Levels, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		val, err := strconv.ParseFloat(p, 64)
		if err == nil {
			spec.Levels = append(spec.Levels, val)
		}
	}
	return spec, nil
}

// ── Render Modes Spec ────────────────────────────────────────────────────────

type RenderModeGeometry struct {
	W int `yaml:"w"`
	H int `yaml:"h"`
}

type RenderModeDef struct {
	Name        string              `yaml:"name"`
	Aliases     []string            `yaml:"aliases"`
	Description string              `yaml:"description"`
	Renderer    string              `yaml:"renderer"`
	Cell        RenderModeGeometry  `yaml:"cell"`
	Analysis    *RenderModeGeometry `yaml:"analysis"`
	GlyphSets   []string            `yaml:"glyph_sets"`
	Colorer     string              `yaml:"colorer"`
	SmartStep   string              `yaml:"smart_step"`
	NativeStep  int                 `yaml:"native_step"`
}

type SmartRenderPolicy struct {
	Metric       string  `yaml:"metric"`
	MaxReduction float64 `yaml:"max_reduction"`
	Step         string  `yaml:"step"`
	TieBreak     string  `yaml:"tie_break"`
}

type RenderModesSpec struct {
	Cycle            []string            `yaml:"cycle"`
	Modes            []RenderModeDef     `yaml:"modes"`
	GlyphSets        map[string][]string `yaml:"glyph_sets"`
	Colorers         map[string]string   `yaml:"colorers"`
	Renderers        map[string]string   `yaml:"renderers"`
	Smart            SmartRenderPolicy   `yaml:"smart"`
	SetRegistry      []GlyphSetDef       `yaml:"set_registry"`
	Compositions     map[string][]int    `yaml:"compositions"`
	CompositionOrder []string            `yaml:"composition_order"`
}

func LoadRenderModes() (RenderModesSpec, error) {
	var spec RenderModesSpec
	data, err := fs.ReadFile(FS, "render_modes.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

type GlyphSetDef struct {
	ID          int                `yaml:"id"`
	Name        string             `yaml:"name"`
	Geometry    RenderModeGeometry `yaml:"geometry"`
	Glyphs      []string           `yaml:"glyphs"`
	Generated   string             `yaml:"generated"`
	Approximate bool               `yaml:"approximate"`
}

type GlyphSetResolution struct {
	IDs         []int
	Glyphs      []rune
	Geometry    RenderModeGeometry
	Approximate bool
}

// ResolveGlyphSetExpression resolves a named union or the d<ids> debug grammar.
// Names and aliases are case-sensitive; IDs are sorted and set 0 is implicit.
func ResolveGlyphSetExpression(expression string) (GlyphSetResolution, error) {
	rm, err := LoadRenderModes()
	if err != nil {
		return GlyphSetResolution{}, fmt.Errorf("load render mode registry: %w", err)
	}
	defs := make(map[int]GlyphSetDef, len(rm.SetRegistry))
	byName := make(map[string]int, len(rm.SetRegistry))
	for _, def := range rm.SetRegistry {
		if _, ok := defs[def.ID]; ok || def.Name == "" {
			return GlyphSetResolution{}, fmt.Errorf("invalid glyph set %q/%d", def.Name, def.ID)
		}
		defs[def.ID] = def
		byName[def.Name] = def.ID
	}
	ids, err := resolveExpression(expression, rm.Compositions, byName, defs)
	if err != nil {
		return GlyphSetResolution{}, err
	}
	seen := map[int]bool{0: true}
	normalized := []int{0}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			normalized = append(normalized, id)
		}
	}
	sort.Ints(normalized)
	res := GlyphSetResolution{IDs: normalized, Geometry: RenderModeGeometry{W: 1, H: 1}}
	for _, id := range normalized {
		def := defs[id]
		res.Geometry.W = lcm(res.Geometry.W, def.Geometry.W)
		res.Geometry.H = lcm(res.Geometry.H, def.Geometry.H)
		res.Approximate = res.Approximate || def.Approximate
		glyphs := def.Glyphs
		if def.Generated == "sextant_2x3_with_columns" {
			glyphs = append([]string{" "}, generatedSextants()...)
		}
		for _, glyph := range glyphs {
			for _, r := range glyph {
				if !containsRune(res.Glyphs, r) {
					res.Glyphs = append(res.Glyphs, r)
				}
			}
		}
	}
	return res, nil
}

func resolveExpression(expr string, compositions map[string][]int, names map[string]int, defs map[int]GlyphSetDef) ([]int, error) {
	if ids, ok := compositions[expr]; ok {
		return append([]int(nil), ids...), nil
	}
	if expr == "" {
		return nil, fmt.Errorf("empty glyph-set expression")
	}
	if strings.HasPrefix(expr, "d") {
		if expr == "d" {
			return []int{0}, nil
		}
		body := strings.TrimPrefix(expr, "d")
		if body == "" {
			return nil, fmt.Errorf("empty debug expression")
		}
		parts := strings.Split(body, ",")
		ids := make([]int, 0, len(parts))
		for _, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("empty token in debug expression %q", expr)
			}
			id, err := strconv.Atoi(part)
			if err != nil || id < 0 || strconv.Itoa(id) != part {
				return nil, fmt.Errorf("invalid set ID %q in %q", part, expr)
			}
			if _, ok := defs[id]; !ok {
				return nil, fmt.Errorf("unknown glyph set ID %d", id)
			}
			ids = append(ids, id)
		}
		return ids, nil
	}
	parts := strings.Split(expr, "+")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("empty union operand in %q", expr)
		}
		if id, ok := names[part]; ok {
			ids = append(ids, id)
			continue
		}
		if id, ok := compositions[part]; ok {
			ids = append(ids, id...)
			continue
		}
		return nil, fmt.Errorf("unknown union operand %q", part)
	}
	return ids, nil
}

func generatedSextants() []string {
	out := make([]string, 0, 62)
	for r := rune(0x1fb00); r <= 0x1fb3b; r++ {
		out = append(out, string(r))
	}
	out = append(out, "▌", "▐")
	return out
}
func containsRune(rs []rune, r rune) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
func lcm(a, b int) int {
	if a <= 0 || b <= 0 {
		return 0
	}
	return a / gcd(a, b) * b
}

// ── Controls Spec ────────────────────────────────────────────────────────────

type ControlDef struct {
	Type    string      `yaml:"type"`
	Min     int         `yaml:"min"`
	Max     int         `yaml:"max"`
	Values  []string    `yaml:"values"`
	Default interface{} `yaml:"default"`
	Set     string      `yaml:"set"`
	Get     string      `yaml:"get"`
}

type ControlsSpec struct {
	Controls map[string]ControlDef `yaml:"controls"`
}

func LoadControls() (ControlsSpec, error) {
	var spec ControlsSpec
	data, err := fs.ReadFile(FS, "controls.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Yaml View Spec (about.yaml) ──────────────────────────────────────────────

type YamlView struct {
	Type     string   `yaml:"type"`
	Name     string   `yaml:"name"`
	Title    string   `yaml:"title"`
	Content  string   `yaml:"content"`
	Controls []string `yaml:"controls"`
}

func LoadYamlView(name string) (*YamlView, error) {
	var spec YamlView
	data, err := fs.ReadFile(FS, name)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// ── Config Spec ──────────────────────────────────────────────────────────────

type ConfigDef struct {
	PreviewHeight     int    `yaml:"preview_height"`
	ViewMode          string `yaml:"view_mode"`
	MaxJobs           int    `yaml:"max_jobs"`
	VideoFrames       int    `yaml:"video_frames"`
	PreviewVideos     bool   `yaml:"preview_videos"`
	VideoPreviewDelay int    `yaml:"video_preview_delay"`
}

type ConfigSpec struct {
	Config ConfigDef `yaml:"config"`
}

// LoadConfigDefaults reads default settings values.
func LoadConfigDefaults() (ConfigSpec, error) {
	var spec ConfigSpec
	data, err := fs.ReadFile(FS, "config.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}
