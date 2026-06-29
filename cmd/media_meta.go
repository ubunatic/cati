package cmd

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/ubunatic/cati/internal/halfblock"
)

// MediaMeta holds display-ready metadata strings for a single media file.
// All fields are strings; empty string means unknown or not applicable.
// Display fields (DispW, DispH, DispMode) are set by the caller from layout
// context — they depend on the current terminal/cell size, not the file itself.
type MediaMeta struct {
	// File
	Name      string // base filename
	NameShort string // optional shortened filename for narrow hint bars
	Ext       string // lowercase extension without dot, e.g. "jpg"
	Size      string // human-readable file size, e.g. "3.2 MB"
	Modified  string // modification date "2006-01-02"

	// Source pixel dimensions
	SrcW string // e.g. "1920"
	SrcH string // e.g. "1080"

	// Display context — caller sets these from the current layout
	DispW    string // terminal chars wide
	DispH    string // terminal chars tall
	DispMode string // "half", "quad", or "spark"

	// Video/audio
	Duration  string // "1:23:45", "1:23", or "45s"
	FPS       string // "24" or "29.97"
	VCodec    string // e.g. "h264"
	ACodec    string // e.g. "aac", or "" if no audio
	Bitrate   string // e.g. "5.2 Mbps"
	Container string // e.g. "mp4"

	// Tags (from EXIF / media container)
	Title    string
	Author   string
	Date     string // capture/creation date from tags
	Location string // GPS or place string
	Camera   string // device model
	Comment  string
}

// Vars returns a flat map[string]string with every field under a "meta.*" key.
// Every key is always present (empty string for unknown), so spec templates
// referencing { meta.X } never fall back to showing the raw key name.
func (m MediaMeta) Vars() map[string]string {
	srcRes := ""
	if m.SrcW != "" && m.SrcH != "" {
		srcRes = m.SrcW + "×" + m.SrcH
	}
	dispRes := ""
	if m.DispW != "" && m.DispH != "" && m.DispMode != "" {
		dispRes = m.DispW + "×" + m.DispH + " " + m.DispMode
	}
	return map[string]string{
		"meta.name": m.Name,
		"meta.name_short": func() string {
			if m.NameShort != "" {
				return m.NameShort
			}
			return m.Name
		}(),
		"meta.ext":       m.Ext,
		"meta.size":      m.Size,
		"meta.modified":  m.Modified,
		"meta.src_w":     m.SrcW,
		"meta.src_h":     m.SrcH,
		"meta.src_res":   srcRes,
		"meta.disp_w":    m.DispW,
		"meta.disp_h":    m.DispH,
		"meta.disp_mode": m.DispMode,
		"meta.disp_res":  dispRes,
		"meta.duration":  m.Duration,
		"meta.fps":       m.FPS,
		"meta.vcodec":    m.VCodec,
		"meta.acodec":    m.ACodec,
		"meta.bitrate":   m.Bitrate,
		"meta.container": m.Container,
		"meta.title":     m.Title,
		"meta.author":    m.Author,
		"meta.date":      m.Date,
		"meta.location":  m.Location,
		"meta.camera":    m.Camera,
		"meta.comment":   m.Comment,
	}
}

// loadMediaMeta loads file-level metadata for path synchronously.
// For images it reads source dimensions from the file header.
// For videos (and for rich tag data on any file) it calls ffprobe.
// Display fields (DispW, DispH, DispMode) are left empty — the caller sets
// them from the current terminal/cell layout before passing Vars() downstream.
func loadMediaMeta(path string, isVideo bool) MediaMeta {
	var m MediaMeta

	info, err := os.Stat(path)
	if err != nil {
		return m
	}
	m.Name = filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))
	if len(ext) > 1 {
		m.Ext = ext[1:] // strip leading "."
	}
	m.Size = humanSize(info.Size())
	m.Modified = info.ModTime().Format("2006-01-02")

	if !isVideo {
		// Fast header-only decode to get source dimensions.
		if f, err := os.Open(path); err == nil {
			if cfg, _, err := image.DecodeConfig(f); err == nil {
				m.SrcW = fmt.Sprintf("%d", cfg.Width)
				m.SrcH = fmt.Sprintf("%d", cfg.Height)
			}
			_ = f.Close()
		}
	}

	// ffprobe gives rich data for both video and image files.
	pr := halfblock.ProbeMediaMeta(path)

	if pr.Width > 0 && pr.Height > 0 {
		m.SrcW = fmt.Sprintf("%d", pr.Width)
		m.SrcH = fmt.Sprintf("%d", pr.Height)
	}
	if pr.Duration > 0 {
		m.Duration = formatDuration(pr.Duration)
	}
	if pr.FPS > 0 {
		m.FPS = formatFPS(pr.FPS)
	}
	m.VCodec = pr.VCodec
	m.ACodec = pr.ACodec
	if pr.Bitrate > 0 {
		m.Bitrate = formatBitrate(pr.Bitrate)
	}
	m.Container = pr.Container
	m.Title = pr.Title
	m.Author = pr.Artist
	m.Date = pr.Date
	m.Location = pr.Location
	m.Camera = pr.Camera
	m.Comment = pr.Comment

	return m
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func formatDuration(secs float64) string {
	total := int(math.Round(secs))
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%d:%02d", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatFPS(fps float64) string {
	rounded := math.Round(fps*100) / 100
	s := fmt.Sprintf("%.2f", rounded)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func formatBitrate(bps int64) string {
	switch {
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.0f kbps", float64(bps)/1_000)
	default:
		return fmt.Sprintf("%d bps", bps)
	}
}
