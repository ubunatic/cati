package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
	spec "ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
)

func drawAboutPage(w io.Writer, termCols, termRows int, style *StyleConfig, about *YamlView) {
	halfblock.ClearScreen(w)
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
		value := "?"
		if h, ok := controlHandlers[c.Get]; ok && h.get != nil {
			value = h.get(temp)
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
func drawHintBar(w io.Writer, termRow, termCols int, label string, vars map[string]string, style *StyleConfig) {
	ctrlAnsi := styleBG(style.ControlBarBg, "") + styleFG(style.ControlBarFg, "")
	text := renderTpl(label, vars, ctrlAnsi)
	cols := writerTermCols(w)
	if cols <= 0 {
		cols = termCols
	}
	if cols > 2 {
		text = truncateANSI(text, cols-2)
	}
	fmt.Fprintf(w, "\x1b[%d;1H\x1b[K%s %s \x1b[m", termRow, ctrlAnsi, text)
}

func writerTermCols(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	cols, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return cols
}

// drawBottomMenu renders the button bar for the given view using the row template from views.yaml.
// conditions is an optional map of runtime boolean flags (e.g. "playing") used by if() expressions.
// btnActions maps button key names to their registered action names (from spec/buttons.yaml); nil means use key name as action.
func drawBottomMenu(w io.Writer, termRows, termCols int, viewMode string, activeAction string, style *StyleConfig, labels map[string]string, viewBtnRows map[string]string, conditions map[string]bool, btnActions map[string]string, altBtnActions map[string]string) []menuButton {
	if style == nil {
		style = &StyleConfig{}
	}

	viewName := viewMode
	if viewMode == "grid" || viewMode == "preview" {
		viewName = "browser"
	}

	ctrlAnsi := styleBG(style.ControlBarBg, "") + styleFG(style.ControlBarFg, "")
	fmt.Fprintf(w, "\x1b[%d;1H\x1b[K%s", termRows-1, ctrlAnsi)
	maxCols := writerTermCols(w)
	if maxCols <= 0 {
		maxCols = termCols
	}

	tpl := viewBtnRows[viewName]
	if tpl == "" {
		return nil
	}

	items := parseMenuItems(tpl, labels, conditions, btnActions, altBtnActions, loadButtonPriorities())
	if maxCols > 0 {
		fitMenuItems(items, maxCols)
	}

	var buttons []menuButton
	col := 1
	for _, item := range items {
		if !item.visible {
			continue
		}
		literal := item.literal
		if strings.TrimSpace(literal) != "" {
			fmt.Fprintf(w, "\x1b[%d;%dH%s%s\x1b[m", termRows-1, col, ctrlAnsi, literal)
		}
		col += utf8.RuneCountInString(literal)

		width := tplWidth(item.label, nil)
		btn := menuButton{label: item.label, action: item.action, altAction: item.altAction, col: col, width: width}
		buttons = append(buttons, btn)

		fg := styleFG(style.BtnFg, "")
		bg := styleBG(style.BtnBg, "")
		boldEsc := ""
		for _, mod := range item.mods {
			switch strings.TrimSpace(mod) {
			case "bold":
				boldEsc = "\x1b[1m"
			default:
				if c := styleFG(strings.TrimSpace(mod), ""); c != "" {
					fg = c
				}
			}
		}
		if item.action == activeAction {
			boldEsc = "\x1b[1m"
			fg = styleFG(style.BtnActiveFg, fg)
			bg = styleBG(style.BtnActiveBg, bg)
		}
		baseAnsi := boldEsc + fg + bg
		rendered := renderTpl(item.label, nil, baseAnsi)
		fmt.Fprintf(w, "\x1b[%d;%dH%s%s\x1b[m", termRows-1, col, baseAnsi, rendered)
		col += btn.width
	}
	return buttons
}

type menuLayoutItem struct {
	literal      string
	key          string
	label        string
	fullLabel    string
	compactLabel string
	mods         []string
	action       string
	altAction    string
	prio         int
	visible      bool
	collapsed    bool
}

func parseMenuItems(tpl string, labels map[string]string, conditions map[string]bool, btnActions map[string]string, altBtnActions map[string]string, prios map[string]int) []menuLayoutItem {
	var items []menuLayoutItem
	i := 0
	for i < len(tpl) {
		open := strings.IndexByte(tpl[i:], '{')
		if open == -1 {
			break
		}
		literal := tpl[i : i+open]
		i += open + 1

		close := strings.IndexByte(tpl[i:], '}')
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
		items = append(items, menuLayoutItem{
			literal:      literal,
			key:          key,
			label:        label,
			fullLabel:    label,
			compactLabel: compactButtonLabel(label),
			mods:         mods,
			action:       action,
			altAction:    altBtnActions[key],
			prio:         prios[key],
			visible:      true,
		})
	}
	return items
}

func fitMenuItems(items []menuLayoutItem, maxCols int) {
	if menuItemsWidth(items) <= maxCols {
		return
	}
	order := make([]int, len(items))
	for i := range items {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := items[order[i]], items[order[j]]
		if a.prio != b.prio {
			return a.prio < b.prio
		}
		return order[i] > order[j]
	})
	for _, idx := range order {
		item := &items[idx]
		if !item.visible || item.collapsed || item.compactLabel == item.fullLabel {
			continue
		}
		item.label = item.compactLabel
		item.collapsed = true
		if menuItemsWidth(items) <= maxCols {
			return
		}
	}
	for _, idx := range order {
		items[idx].visible = false
		if menuItemsWidth(items) <= maxCols {
			return
		}
	}
}

func menuItemsWidth(items []menuLayoutItem) int {
	width := 0
	for _, item := range items {
		if !item.visible {
			continue
		}
		width += utf8.RuneCountInString(item.literal)
		width += tplWidth(item.label, nil)
	}
	return width
}

func compactButtonLabel(label string) string {
	left, body, right := splitButtonCaps(label)
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "{") {
		if end := strings.IndexByte(body, '}'); end >= 0 {
			return left + body[:end+1] + right
		}
	}
	r, size := utf8.DecodeRuneInString(body)
	if r == utf8.RuneError && size == 0 {
		return label
	}
	return left + body[:size] + right
}

func splitButtonCaps(label string) (string, string, string) {
	if label == "" {
		return "", "", ""
	}
	left := label[:1]
	right := label[len(label)-1:]
	if left == "[" && right == "]" {
		return left, label[1 : len(label)-1], right
	}
	return "", label, ""
}
