package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
)

// This is assets/cati_0001.png, kept inline so the command works from an
// installed binary without depending on the repository's asset directory.
const embeddedCatiLogo = "iVBORw0KGgoAAAANSUhEUgAAABgAAAAOCAYAAAA1+Nx+AAAAAXNSR0IArs4c6QAAARdJREFUOI1jVBL3/f/WQ42BFkB4xy0GlrceagzCO24x/BWUpKrhzO+fM/wVlGRgYWBggBhuZ0xVC97+vMUgcPIzAxOlBvG5yGBlwwCKBeLF4hgKQmaKMITMFEFRA8PoBn+5/RG/Bd8vsqJoDJkpwnD4FjPD4VvMcMO/X2Rl+H6RFcUQmME8qvwYFrAgcz7tecLAwCDDIF4szvCy9yUDAwMDnEZVw8DAqY9wyL+HnzEMxuoDiMbfGC6kBKD4ABYEMFcevsUMD/816W8wghAG+Fxk4HrwWoBsOAMDJHgOIxkIC0JkgByx2CIZSxygAlxxAAPI4Y8tLijOB4QACwMDJFu//XmLqgbDih9GJXHf/9Quh2CA+f1zBgAYdXGLiM0UnAAAAABJRU5ErkJggg=="

func modesCommand() *cobra.Command {
	var width int
	var smart bool
	cmd := &cobra.Command{
		Use:   "modes",
		Short: "list render modes with a cati/emojig logo demo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if width < 1 {
				return fmt.Errorf("--width must be greater than zero")
			}
			return runModesDemo(cmd.OutOrStdout(), width, smart)
		},
	}
	cmd.Flags().IntVarP(&width, "width", "w", 12, "target width of each logo demo")
	cmd.Flags().BoolVar(&smart, "smart", false, "choose the best nearby width by PSNR")
	return cmd
}

func runModesDemo(out io.Writer, width int, smart bool) error {
	cati, err := decodeEmbeddedLogo()
	if err != nil {
		return err
	}
	emojigPath := filepath.Join("testdata", "emojig-icon.svg")
	emojig, err := halfblock.LoadImage(emojigPath)
	if err != nil {
		return fmt.Errorf("load emojig logo %q: %w", emojigPath, err)
	}

	fmt.Fprintln(out, "Available render modes (cati logo | emojig logo):")
	for _, entry := range renderModes {
		normal, err := renderModePair(cati, emojig, width, entry.cfg, false, entry.name)
		if err != nil {
			return err
		}
		if !smart {
			fmt.Fprintf(out, "\n%s\n", entry.name)
			for _, line := range normal {
				fmt.Fprintln(out, line)
			}
			continue
		}
		smartLines, err := renderModePair(cati, emojig, width, entry.cfg, true, entry.name)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "\n%-*s+smart\n", ansiLinesWidth(normal)+4, entry.name)
		for i := 0; i < max(len(normal), len(smartLines)); i++ {
			var left, right string
			if i < len(normal) {
				left = normal[i]
			}
			if i < len(smartLines) {
				right = smartLines[i]
			}
			fmt.Fprintf(out, "%s    %s\n", left, right)
		}
	}
	return nil
}

func renderModePair(cati, emojig image.Image, width int, cfg renderCfg, smart bool, name string) ([]string, error) {
	cfg.smart = smart
	left, err := smartPrepare(cati, width, 7, cfg)
	if err != nil {
		return nil, fmt.Errorf("fit %s cati logo: %w", name, err)
	}
	right, err := smartPrepare(emojig, width, 7, cfg)
	if err != nil {
		return nil, fmt.Errorf("fit %s emojig logo: %w", name, err)
	}
	leftLines, err := renderDemoLines(left, cfg)
	if err != nil {
		return nil, fmt.Errorf("render %s cati logo: %w", name, err)
	}
	rightLines, err := renderDemoLines(right, cfg)
	if err != nil {
		return nil, fmt.Errorf("render %s emojig logo: %w", name, err)
	}
	leftWidth := ansiLinesWidth(leftLines)
	lines := make([]string, 0, max(len(leftLines), len(rightLines)))
	for i := 0; i < max(len(leftLines), len(rightLines)); i++ {
		var leftLine, rightLine string
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}
		lines = append(lines, fmt.Sprintf("  %s  |  %s", padANSILine(leftLine, leftWidth), padANSILine(rightLine, leftWidth)))
	}
	return lines, nil
}

func ansiLinesWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, ansiLineWidth(line))
	}
	return width
}

func ansiLineWidth(line string) int {
	widths := visibleLineWidths(line)
	if len(widths) == 0 {
		return 0
	}
	return widths[0]
}

func padANSILine(line string, width int) string {
	return line + strings.Repeat(" ", max(0, width-ansiLineWidth(line)))
}

func decodeEmbeddedLogo() (image.Image, error) {
	b, err := base64.StdEncoding.DecodeString(embeddedCatiLogo)
	if err != nil {
		return nil, fmt.Errorf("decode embedded cati logo: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("decode embedded cati logo PNG: %w", err)
	}
	return img, nil
}

func renderDemoLines(img image.Image, rc renderCfg) ([]string, error) {
	var buf bytes.Buffer
	var err error
	switch {
	case rc.mode.useSextant():
		err = sextant.Render(&buf, img, 0, sextant.Options{Mode: rc.sextantMode, NoLinePrefix: true})
	case rc.mode.useQuad():
		opts := rc.quadOpts
		opts.NoLinePrefix = true
		err = quadblock.Render(&buf, img, 0, opts)
	case rc.mode.useSpark():
		spec := rc.mode.viewSpec()
		err = sparkline.Render(&buf, img, max(1, img.Bounds().Dx()/spec.CellW), sparkline.Options{Mode: rc.sparkMode, Rows: max(1, img.Bounds().Dy()/spec.CellH), CellW: spec.CellW, CellH: spec.CellH, AspectX: spec.AspectX, NoLinePrefix: true})
	default:
		err = halfblock.Render(&buf, img, 0, halfblock.Options{NoLinePrefix: true})
	}
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(buf.String(), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}
