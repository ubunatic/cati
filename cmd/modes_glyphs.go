package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/spec"
)

func modesGlyphsCommand() *cobra.Command {
	var format string
	var grid int
	cmd := &cobra.Command{
		Use:   "glyphs [mode|expr]",
		Short: "inspect glyph inventories and subpixel masks",
		Long:  "Inspect spec-resolved glyph masks. Select a mode or alias (oct, vec, half), a set name (sextants), a named union (half+sextants), or set IDs (d10,50). Without a selector, list available compositions and sets.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "ascii" && format != "halfblock" {
				return fmt.Errorf("invalid --format %q: use ascii or halfblock", format)
			}
			if grid < 1 {
				return fmt.Errorf("invalid --grid %d: must be >= 1", grid)
			}
			if len(args) == 0 {
				return listGlyphSelectors(cmd.OutOrStdout())
			}
			return inspectGlyphs(cmd.OutOrStdout(), args[0], format, grid)
		},
	}
	cmd.Flags().StringVar(&format, "format", "halfblock", "mask preview format: halfblock or ascii (# foreground, . background)")
	cmd.Flags().IntVar(&grid, "grid", 8, "number of card columns in grid layout (default: 8)")
	return cmd
}

func listGlyphSelectors(w io.Writer) error {
	rm, err := spec.LoadRenderModes()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(rm.Compositions))
	for name := range rm.Compositions {
		names = append(names, name)
	}
	sort.Strings(names)
	if _, err := fmt.Fprintf(w, "Glyph compositions and aliases:\n%s\n\n", strings.Join(names, " ")); err != nil {
		return err
	}
	var sets strings.Builder
	if err := listGlyphSets(&sets); err != nil {
		return err
	}
	_, err = io.WriteString(w, sets.String())
	return err
}

func inspectGlyphs(w io.Writer, expression, format string, grid int) error {
	resolution, err := spec.ResolveGlyphSetExpression(expression)
	if err != nil {
		return err
	}
	geo := resolution.Geometry
	if _, err := fmt.Fprintf(w, "Glyph inventory: %s\nSets: %v | Glyphs: %d | Geometry: %dx%d | Approximate: %t\n", expression, resolution.IDs, len(resolution.Shapes), geo.W, geo.H, resolution.Approximate); err != nil {
		return err
	}
	if grid < 1 {
		grid = 8
	}
	var cards [][]string
	for _, shape := range resolution.Shapes {
		preview, err := glyphMaskPreview(shape.Mask, geo.W, geo.H, format)
		if err != nil {
			return err
		}
		var label string
		if shape.Glyph == ' ' {
			label = "␠ (space)"
		} else if shape.Glyph < 32 || shape.Glyph == 0x7f {
			label = fmt.Sprintf("%q", shape.Glyph)
		} else {
			label = fmt.Sprintf("'%c'", shape.Glyph)
		}
		cardHeader := fmt.Sprintf("%s U+%04X %dx%d", label, shape.Glyph, geo.W, geo.H)
		cardLines := append([]string{cardHeader}, strings.Split(preview, "\n")...)
		cards = append(cards, cardLines)
	}
	for _, line := range formatCardsGrid(cards, grid, 0) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// glyphMaskPreview reads the resolver's row-major mask without aspect scaling.
// Borders retain empty rows and columns when the preview is copied or piped.
func glyphMaskPreview(mask []bool, width, height int, format string) (string, error) {
	if width <= 0 || height <= 0 || len(mask)/width != height || len(mask)%width != 0 {
		return "", fmt.Errorf("invalid mask geometry %dx%d (length %d)", width, height, len(mask))
	}
	step := 1
	if format == "halfblock" {
		step = 2
	} else if format != "ascii" {
		return "", fmt.Errorf("invalid mask format %q", format)
	}
	var b strings.Builder
	topBorder := "┌" + strings.Repeat("─", width) + "┐"
	bottomBorder := "└" + strings.Repeat("─", width) + "┘"
	b.WriteString(topBorder)
	for y := 0; y < height; y += step {
		b.WriteString("\n│")
		for x := 0; x < width; x++ {
			top := mask[y*width+x]
			if step == 1 {
				if top {
					b.WriteByte('#')
				} else {
					b.WriteByte('.')
				}
				continue
			}
			bottom := y+1 < height && mask[(y+1)*width+x]
			switch {
			case top && bottom:
				b.WriteRune('█')
			case top:
				b.WriteRune('▀')
			case bottom:
				b.WriteRune('▄')
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteString("│")
	}
	b.WriteByte('\n')
	b.WriteString(bottomBorder)
	return b.String(), nil
}
