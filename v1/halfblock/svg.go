package halfblock

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SVGExts is the set of file extensions cati recognises as SVG images.
var SVGExts = map[string]bool{
	".svg": true,
}

// IsSVG reports whether path has a recognised SVG file extension.
func IsSVG(path string) bool {
	return SVGExts[strings.ToLower(filepath.Ext(path))]
}

// SVGMaxDim caps the long edge of the rasterized SVG in pixels. SVGs are
// vector and have no native resolution, so we rasterize at a fixed size
// large enough to survive downstream scaleToFit/zoom without blurring, while
// bounding memory use for pathological inputs.
const SVGMaxDim = 2048

// RasterizeSVG rasterizes path (an SVG file) to an image.Image using
// rsvg-convert. It requires rsvg-convert to be on $PATH.
func RasterizeSVG(path string) (image.Image, error) {
	return RasterizeSVGWithTarget(path, 0, 0)
}

// RasterizeSVGWithTarget rasterizes path to fit inside maxWidth×maxHeight
// pixels while preserving the SVG aspect ratio. A non-positive dimension means
// "unconstrained"; when both dimensions are unconstrained, SVGMaxDim is used as
// the long-edge cap for backwards compatibility.
func RasterizeSVGWithTarget(path string, maxWidth, maxHeight int) (image.Image, error) {
	w, h, err := svgRasterTarget(path, maxWidth, maxHeight)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("rsvg-convert",
		"--format=png",
		"-w", strconv.Itoa(w),
		"-h", strconv.Itoa(h),
		path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rsvg-convert %s: %w", path, err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decode rasterized %s: %w", path, err)
	}
	return img, nil
}

func svgRasterTarget(path string, maxWidth, maxHeight int) (width, height int, err error) {
	srcW, srcH, err := ProbeSVGDimensions(path)
	if err != nil {
		if maxWidth > 0 && maxHeight > 0 {
			return maxWidth, maxHeight, nil
		}
		if maxWidth > 0 {
			return maxWidth, maxWidth, nil
		}
		if maxHeight > 0 {
			return maxHeight, maxHeight, nil
		}
		return SVGMaxDim, SVGMaxDim, nil
	}
	width, height = fitSVGTarget(srcW, srcH, maxWidth, maxHeight)
	return width, height, nil
}

func fitSVGTarget(srcW, srcH, maxWidth, maxHeight int) (width, height int) {
	if srcW <= 0 || srcH <= 0 {
		return 1, 1
	}
	if maxWidth <= 0 && maxHeight <= 0 {
		if srcW >= srcH {
			return SVGMaxDim, max(1, srcH*SVGMaxDim/srcW)
		}
		return max(1, srcW*SVGMaxDim/srcH), SVGMaxDim
	}
	width, height = srcW, srcH
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
		height = max(1, srcH*width/srcW)
	}
	if maxHeight > 0 && height > maxHeight {
		height = maxHeight
		width = max(1, srcW*height/srcH)
	}
	return max(1, width), max(1, height)
}

// ProbeSVGDimensions returns the declared CSS pixel width and height of the
// SVG at path, read from the root <svg> element's width/height attributes.
// Absolute SVG/CSS units are converted with the browser rule 1in = 96px.
// When one dimension is missing but a viewBox exists, the missing dimension is
// derived from the viewBox aspect ratio. When width/height are absent or
// relative units such as percentages, it falls back to the viewBox size. It
// reads only the root start element, not the full document.
func ProbeSVGDimensions(path string) (width, height int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return 0, 0, fmt.Errorf("%s: no <svg> element found", path)
			}
			return 0, 0, fmt.Errorf("parse %s: %w", path, err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "svg" {
			continue
		}

		var widthAttr, heightAttr, viewBox string
		for _, a := range start.Attr {
			switch a.Name.Local {
			case "width":
				widthAttr = a.Value
			case "height":
				heightAttr = a.Value
			case "viewBox":
				viewBox = a.Value
			}
		}

		w, hasW := parseSVGLength(widthAttr)
		h, hasH := parseSVGLength(heightAttr)
		vbW, vbH, hasViewBox := parseSVGViewBox(viewBox)
		switch {
		case hasW && hasH:
			return w, h, nil
		case hasW && hasViewBox:
			return w, max(1, int(math.Round(float64(w)*float64(vbH)/float64(vbW)))), nil
		case hasH && hasViewBox:
			return max(1, int(math.Round(float64(h)*float64(vbW)/float64(vbH)))), h, nil
		case hasViewBox:
			return vbW, vbH, nil
		}
		return 0, 0, fmt.Errorf("%s: no usable width/height or viewBox on <svg>", path)
	}
}

// parseSVGLength parses SVG/CSS absolute lengths into CSS pixels. Unitless
// values and px are already CSS pixels. Percentages and font/viewport-relative
// units are rejected because they need a containing block or font context.
func parseSVGLength(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	unit := "px"
	for _, candidate := range []string{"px", "cm", "mm", "q", "in", "pt", "pc"} {
		if strings.HasSuffix(s, candidate) {
			unit = candidate
			s = strings.TrimSpace(strings.TrimSuffix(s, candidate))
			break
		}
	}
	if strings.HasSuffix(s, "%") || strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyz") {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	switch unit {
	case "in":
		v *= 96
	case "cm":
		v *= 96 / 2.54
	case "mm":
		v *= 96 / 25.4
	case "q":
		v *= 96 / 101.6 // 1q = 1/40cm.
	case "pt":
		v *= 96.0 / 72.0
	case "pc":
		v *= 16 // 1pc = 12pt = 16px.
	}
	return max(1, int(math.Round(v))), true
}

// parseSVGViewBox parses a viewBox attribute ("minX minY width height") and
// returns its width and height.
func parseSVGViewBox(s string) (width, height int, ok bool) {
	fields := strings.Fields(strings.ReplaceAll(s, ",", " "))
	if len(fields) != 4 {
		return 0, 0, false
	}
	w, err1 := strconv.ParseFloat(fields[2], 64)
	h, err2 := strconv.ParseFloat(fields[3], 64)
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return int(w + 0.5), int(h + 0.5), true
}
