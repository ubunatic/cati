package cmd

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/internal/metrics"
	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/sparkline"
)

type probePattern struct{ name, mode, description string }

var probePatterns = []probePattern{
	{"probe", "oct", "asymmetric: 2x4 red pixel at (0,1) on blue"},
	{"diag", "vec6", "vector: 4x6 green upper-left diagonal on purple"},
	{"grid", "oct", "multi: 12x8 yellow pattern on dark background"},
}

func modesProbeCommand() *cobra.Command {
	var mode, imagePath string
	var list bool
	cmd := &cobra.Command{
		Use:   "test [pattern|image]",
		Short: "compare source pixels, reconstructed pixels, and emitted glyphs",
		Long:  "Compare at 1:1 pixel resolution. Without input, run all three probe patterns. --mode overrides each pattern's default (oct, vec, oct); images default to oct. Patterns: probe/asymmetric, diag/vector, grid/multi. Images are padded to whole cells without resizing; metrics exclude padding.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("image") && (imagePath == "" || len(args) > 0) {
				return fmt.Errorf("--image requires a nonempty path and cannot accompany positional input")
			}
			if list {
				if len(args) > 0 || cmd.Flags().Changed("image") || cmd.Flags().Changed("mode") {
					return fmt.Errorf("--list-patterns cannot accompany input or --mode")
				}
				for _, p := range probePatterns {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", p.name, p.mode, p.description); err != nil {
						return err
					}
				}
				return nil
			}
			if cmd.Flags().Changed("mode") {
				if _, err := spec.ResolveGlyphSetExpression(mode); err != nil {
					return err
				}
			}
			if imagePath != "" {
				return runProbeImage(cmd.OutOrStdout(), imagePath, mode)
			}
			if len(args) == 0 {
				for _, p := range probePatterns {
					if err := runProbePattern(cmd.OutOrStdout(), p.name, mode); err != nil {
						return err
					}
				}
				return nil
			}
			if _, ok := canonicalProbePattern(args[0]); ok {
				return runProbePattern(cmd.OutOrStdout(), args[0], mode)
			}
			return runProbeImage(cmd.OutOrStdout(), args[0], mode)
		},
	}
	cmd.Flags().StringVarP(&mode, "mode", "m", "", "glyph mode or expression (pattern defaults; oct for images)")
	cmd.Flags().StringVarP(&imagePath, "image", "i", "", "custom image path (preserves source pixels)")
	cmd.Flags().BoolVar(&list, "list-patterns", false, "list built-in patterns and aliases")
	return cmd
}

func canonicalProbePattern(name string) (string, bool) {
	switch name {
	case "probe", "asymmetric":
		return "probe", true
	case "diag", "vector":
		return "diag", true
	case "grid", "multi":
		return "grid", true
	}
	return "", false
}

func probeSource(name string) (*image.RGBA, string, error) {
	canonical, ok := canonicalProbePattern(name)
	if !ok {
		return nil, "", fmt.Errorf("unknown probe pattern %q", name)
	}
	w, h, mode := 2, 4, "oct"
	fg, bg := color.RGBA{R: 255, A: 255}, color.RGBA{G: 50, B: 200, A: 255}
	if canonical == "diag" {
		w, h, mode = 4, 6, "vec6"
		fg, bg = color.RGBA{G: 220, B: 50, A: 255}, color.RGBA{R: 120, G: 20, B: 180, A: 255}
	}
	if canonical == "grid" {
		w, h = 12, 8
		fg, bg = color.RGBA{R: 255, G: 220, A: 255}, color.RGBA{R: 20, G: 20, B: 40, A: 255}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on := x == 0 && y == 1
			if canonical == "diag" {
				on = x+y < 3
			}
			if canonical == "grid" {
				on = (x+y)%3 == 0 || (x*y)%5 == 0
			}
			c := bg
			if on {
				c = fg
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img, mode, nil
}

func runProbePattern(w io.Writer, name, mode string) error {
	src, defaultMode, err := probeSource(name)
	if err != nil {
		return err
	}
	if mode == "" {
		mode = defaultMode
	}
	return writeProbe(w, name, src, mode)
}

func runProbeImage(w io.Writer, path, mode string) error {
	if mode == "" {
		mode = "oct"
	}
	if _, err := spec.ResolveGlyphSetExpression(mode); err != nil {
		return err
	}
	src, err := halfblock.LoadImage(path)
	if err != nil {
		return fmt.Errorf("load probe image %q: %w", path, err)
	}
	return writeProbe(w, path, src, mode)
}

type probeResult struct {
	source, reconstructed  *image.RGBA
	glyphs                 string
	cols, rows, padW, padH int
	mse, psnr              float64
	ssim                   string
}

func prepareProbe(src image.Image, mode string) (probeResult, error) {
	res, err := spec.ResolveGlyphSetExpression(mode)
	if err != nil {
		return probeResult{}, err
	}
	b := src.Bounds()
	if b.Empty() {
		return probeResult{}, fmt.Errorf("probe image must have pixels")
	}
	cw, ch := res.Geometry.W, res.Geometry.H
	cols, rows := ceilDiv(b.Dx(), cw), ceilDiv(b.Dy(), ch)
	// Normalize to premultiplied RGBA before metrics (also avoids NRGBA luma
	// fastpath alpha drift tracked in #055). Pad without resampling edge cells.
	padded := image.NewRGBA(image.Rect(0, 0, cols*cw, rows*ch))
	draw.Draw(padded, image.Rect(0, 0, b.Dx(), b.Dy()), src, b.Min, draw.Src)
	shapes := make([]sparkline.Shape, len(res.Shapes))
	for i, s := range res.Shapes {
		shapes[i] = sparkline.Shape{Ch: s.Glyph, Width: cw, Height: ch, Mask: s.Mask}
	}
	opts := sparkline.Options{CellW: cw, CellH: ch, AspectX: 1, Rows: rows, Shapes: shapes, NoLinePrefix: true}
	var glyphs bytes.Buffer
	if err := sparkline.Render(&glyphs, padded, cols, opts); err != nil {
		return probeResult{}, err
	}
	recon, err := sparkline.RenderToImageWithOptions(padded, cols, rows, opts)
	if err != nil {
		return probeResult{}, err
	}
	content := image.Rect(0, 0, b.Dx(), b.Dy())
	// Copy the original extent for display and metrics; retain padded glyph cells.
	source := image.NewRGBA(content)
	reconstructed := image.NewRGBA(content)
	draw.Draw(source, content, padded, image.Point{}, draw.Src)
	draw.Draw(reconstructed, content, recon, image.Point{}, draw.Src)
	ssim := "N/A (<8x8)"
	if b.Dx() >= 8 && b.Dy() >= 8 {
		ssim = fmt.Sprintf("%.6f", metrics.SSIMLuminance(source, reconstructed))
	}
	return probeResult{source: source, reconstructed: reconstructed, glyphs: glyphs.String(), cols: cols, rows: rows, padW: cols*cw - b.Dx(), padH: rows*ch - b.Dy(), mse: metrics.MSE(source, reconstructed), psnr: metrics.PSNR(source, reconstructed), ssim: ssim}, nil
}

func writeProbe(w io.Writer, name string, src image.Image, mode string) error {
	result, err := prepareProbe(src, mode)
	if err != nil {
		return err
	}
	var left, right bytes.Buffer
	opts := halfblock.Options{NoLinePrefix: true}
	if err := halfblock.Render(&left, result.source, 0, opts); err != nil {
		return err
	}
	if err := halfblock.Render(&right, result.reconstructed, 0, opts); err != nil {
		return err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s | Mode: %s | Source: %dx%d | Padding: right=%d bottom=%d ---\n", name, mode, src.Bounds().Dx(), src.Bounds().Dy(), result.padW, result.padH)
	fmt.Fprintf(&out, "MSE (RGBA8): %.6f | PSNR: %.6f dB | SSIM (luma): %s\n", result.mse, result.psnr, result.ssim)
	fmt.Fprintf(&out, "GLYPH OUTPUT (%d cols x %d rows):\n%s", result.cols, result.rows, result.glyphs)
	leftLines := strings.Split(strings.TrimSuffix(left.String(), "\n"), "\n")
	rightLines := strings.Split(strings.TrimSuffix(right.String(), "\n"), "\n")
	const leftTitle = "SOURCE PIXELS (1:1)"
	width := max(len(leftTitle), ansiLinesWidth(leftLines))
	fmt.Fprintf(&out, "%s  |  RECONSTRUCTED PIXELS (1:1)\n", padANSILine(leftTitle, width))
	for i := 0; i < max(len(leftLines), len(rightLines)); i++ {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		fmt.Fprintf(&out, "%s  |  %s\n", padANSILine(l, width), r)
	}
	out.WriteByte('\n')
	_, err = io.WriteString(w, out.String())
	return err
}
