package cmd

import (
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// supportedImageExtensions returns list of file extensions recognised by cati
// (both lowercase and uppercase) for shell completion filtering.
func supportedImageExtensions() []string {
	var exts []string
	for ext := range imageExts {
		clean := strings.TrimPrefix(ext, ".")
		if clean != "" {
			exts = append(exts, strings.ToLower(clean), strings.ToUpper(clean))
		}
	}
	sort.Strings(exts)
	return slices.Compact(exts)
}

func completePlayModes() []string {
	return []string{
		"once\tPlay once and exit (default)",
		"repeat\tLoop playback continuously",
		"preview\tRender frame previews",
	}
}

// aspectModes is the canonical list of --aspect values (name, description).
// Validation, help text, and shell completion all derive from it.
var aspectModes = [][2]string{
	{"default", "Square-pixel font correction (stretch to -W/-H box when both given)"},
	{"aligned", "Continuous aspect snap onto the mode's subcell lattice"},
	{"pixel", "Pixel-art aspect snap with bounded distortion and padding, nearest-neighbor sampling"},
	{"raw", "Alias for pixel"},
	{"1:1", "Alias for pixel"},
	{"contain", "Square-pixel fit inside -W/-H box with letterbox padding"},
	{"fit", "Alias for contain"},
}

func aspectModeNames() []string {
	names := make([]string, len(aspectModes))
	for i, m := range aspectModes {
		names[i] = m[0]
	}
	return names
}

func completeAspectModes() []string {
	out := make([]string, len(aspectModes))
	for i, m := range aspectModes {
		out[i] = m[0] + "\t" + m[1]
	}
	return out
}

func completePrescalerModes() []string {
	return []string{
		"nearest-neighbor\tNearest-neighbor pixel scaling (alias: nn)",
		"nn\tNearest-neighbor pixel scaling",
		"pyramid\tPyramid downsampling for high-ratio scaling",
	}
}

func completeCropSpecs() []string {
	return []string{
		"auto\tAuto-crop to active bounding box (aliases: a, 1, true)",
		"center\tCenter crop to terminal size (aliases: c, m)",
		"l,t\tAnchor top-left (left, top)",
		"c,m\tAnchor center (center, middle)",
		"r,b\tAnchor bottom-right (right, bottom)",
		"W:H\tCrop to width W and height H in cells",
		"W:H:X:Y\tCrop to WxH starting at cell (X,Y)",
	}
}

func completeZoomSpecs() []string {
	return []string{
		"0\tFit image to viewport (default)",
		"1\t1:1 pixel scale (100%)",
		"1:1\t1:1 pixel scale",
		"100%\t1:1 pixel scale",
		"w\tScale image to fit terminal width",
		"h\tScale image to fit terminal height",
	}
}

func completeTimeRanges() []string {
	return []string{
		"5s\tPlayback first 5 seconds",
		"5s:7s\tPlayback window from 5s to 7s",
	}
}

func completeRenderModes() []string {
	var completions []string
	seen := make(map[string]bool)

	add := func(name, desc string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		desc = strings.TrimSpace(strings.ReplaceAll(desc, "\n", " "))
		if desc != "" {
			completions = append(completions, name+"\t"+desc)
		} else {
			completions = append(completions, name)
		}
	}

	for _, m := range renderModes {
		desc := m.definition.Description
		if desc == "" {
			desc = m.name + " render mode"
		}
		add(m.name, desc)
		for _, a := range m.aliases {
			add(a, "alias for "+m.name)
		}
	}

	for _, m := range legacyRenderModes {
		desc := m.definition.Description
		if desc == "" {
			desc = m.name + " (legacy)"
		}
		add(m.name, desc)
		for _, a := range m.aliases {
			add(a, "legacy alias for "+m.name)
		}
	}

	sort.Strings(completions)
	return completions
}

// registerRootCompletion configures Cobra autocompletion on the root cati command.
func registerRootCompletion(root *cobra.Command) {
	root.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if cmd.Flags().Changed("play") {
			hasMode := false
			for _, arg := range args {
				lower := strings.ToLower(arg)
				if lower == "once" || lower == "repeat" || lower == "preview" {
					hasMode = true
					break
				}
			}
			if !hasMode {
				return completePlayModes(), cobra.ShellCompDirectiveNoFileComp
			}
		}
		return supportedImageExtensions(), cobra.ShellCompDirectiveFilterFileExt
	}

	_ = root.RegisterFlagCompletionFunc("play", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePlayModes(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = root.RegisterFlagCompletionFunc("aspect", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeAspectModes(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = root.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeRenderModes(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = root.RegisterFlagCompletionFunc("prescaler", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrescalerModes(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = root.RegisterFlagCompletionFunc("crop", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCropSpecs(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = root.RegisterFlagCompletionFunc("zoom", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeZoomSpecs(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = root.RegisterFlagCompletionFunc("range", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeTimeRanges(), cobra.ShellCompDirectiveNoFileComp
	})
}
