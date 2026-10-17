// Package browser provides stateless presentation helpers for the browser and viewer.
package browser

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"unicode/utf8"
)

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

// ParseColor accepts named colours and three- or six-digit hex RGB values.
func ParseColor(s string) (color.RGBA, bool) {
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

// StyleFG resolves a foreground colour, returning def for invalid or empty values.
func StyleFG(s string, def string) string {
	if s == "" {
		return def
	}
	if ansi, ok := paletteFGCodes[strings.ToLower(s)]; ok {
		return ansi
	}
	c, ok := ParseColor(s)
	if !ok {
		return def
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

// StyleBG resolves a background colour, returning def for invalid or empty values.
func StyleBG(s string, def string) string {
	if s == "" {
		return def
	}
	if ansi, ok := paletteBGCodes[strings.ToLower(s)]; ok {
		return ansi
	}
	c, ok := ParseColor(s)
	if !ok {
		return def
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

// RenderTemplate renders a template string with variable substitution and inline styling.
//
// Syntax: literal text mixed with { expr } blocks where expr is:
//
//	key            — value from vars map (falls back to key name if missing)
//	'literal'      — quoted literal string passed through as-is
//	key | mod …    — value with style modifiers applied
//
// Modifiers: bold, dim, italic, underline, or any named/hex color (applied as fg).
// After each styled segment the baseAnsi style is restored automatically.
func RenderTemplate(tpl string, vars map[string]string, baseAnsi string) string {
	var sb strings.Builder
	i := 0
	for i < len(tpl) {
		open := strings.IndexByte(tpl[i:], '{')
		if open == -1 {
			sb.WriteString(tpl[i:])
			break
		}
		sb.WriteString(tpl[i : i+open])
		i += open + 1
		close := strings.IndexByte(tpl[i:], '}')
		if close == -1 {
			sb.WriteByte('{')
			continue
		}
		expr := tpl[i : i+close]
		i += close + 1

		parts := strings.Split(expr, "|")
		key := strings.TrimSpace(parts[0])
		val := resolve(key, vars)

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
				if fg := StyleFG(strings.TrimSpace(mod), ""); fg != "" {
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

// resolve returns the value for a template key: unquotes 'literal' / "literal",
// looks up vars, or falls back to the key itself.
func resolve(key string, vars map[string]string) string {
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

// TemplateWidth returns the visual (rune) width of a rendered template string,
// counting only the resolved text — not the { } syntax or ANSI escapes.
// TemplateWidth counts runes after resolving template variables without styling.
func TemplateWidth(tpl string, vars map[string]string) int {
	w := 0
	i := 0
	for i < len(tpl) {
		open := strings.IndexByte(tpl[i:], '{')
		if open == -1 {
			w += utf8.RuneCountInString(tpl[i:])
			break
		}
		w += utf8.RuneCountInString(tpl[i : i+open])
		i += open + 1
		close := strings.IndexByte(tpl[i:], '}')
		if close == -1 {
			break
		}
		expr := tpl[i : i+close]
		i += close + 1
		parts := strings.Split(expr, "|")
		w += utf8.RuneCountInString(resolve(strings.TrimSpace(parts[0]), vars))
	}
	return w
}

// TruncateANSI preserves escape sequences while keeping at most maxWidth runes.
func TruncateANSI(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	var out strings.Builder
	width := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			start := i
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
				out.WriteString(s[start:i])
				continue
			}
			out.WriteByte(s[start])
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		if width+1 > maxWidth {
			break
		}
		out.WriteString(s[i : i+size])
		width++
		i += size
	}
	return out.String()
}

// EllipsizeRunes shortens a name to a rune budget, using dots when space permits.
func EllipsizeRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}
