package halfblock

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io"
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

// svgMaxDim caps the long edge of the rasterized SVG in pixels. SVGs are
// vector and have no native resolution, so we rasterize at a fixed size
// large enough to survive downstream scaleToFit/zoom without blurring, while
// bounding memory use for pathological inputs.
const svgMaxDim = 2048

// RasterizeSVG rasterizes path (an SVG file) to an image.Image using
// rsvg-convert. It requires rsvg-convert to be on $PATH.
func RasterizeSVG(path string) (image.Image, error) {
	cmd := exec.Command("rsvg-convert",
		"--format=png",
		"-w", strconv.Itoa(svgMaxDim),
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

// ProbeSVGDimensions returns the declared pixel width and height of the SVG
// at path, read from the root <svg> element's width/height attributes,
// falling back to the viewBox attribute when width/height are absent or not
// expressed in plain pixels. It reads only the root start element, not the
// full document.
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

		if w, ok := parseSVGLength(widthAttr); ok {
			if h, ok := parseSVGLength(heightAttr); ok {
				return w, h, nil
			}
		}
		if w, h, ok := parseSVGViewBox(viewBox); ok {
			return w, h, nil
		}
		return 0, 0, fmt.Errorf("%s: no usable width/height or viewBox on <svg>", path)
	}
}

// parseSVGLength parses a plain-pixel SVG length (e.g. "128" or "128px").
// It rejects percentages and other relative units, which have no fixed size.
func parseSVGLength(s string) (int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "px")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return int(v + 0.5), true
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
