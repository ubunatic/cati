package spec

import (
	"io/fs"
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
	Name      string              `yaml:"name"`
	Aliases   []string            `yaml:"aliases"`
	Renderer  string              `yaml:"renderer"`
	Cell      RenderModeGeometry  `yaml:"cell"`
	Analysis  *RenderModeGeometry `yaml:"analysis"`
	GlyphSets []string            `yaml:"glyph_sets"`
	Colorer   string              `yaml:"colorer"`
}

type RenderModesSpec struct {
	Cycle     []string            `yaml:"cycle"`
	Modes     []RenderModeDef     `yaml:"modes"`
	GlyphSets map[string][]string `yaml:"glyph_sets"`
	Colorers  map[string]string   `yaml:"colorers"`
	Renderers map[string]string   `yaml:"renderers"`
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
