package spec

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// CellFrac is a cell coordinate in [0, 1], written as a number or "a/b".
// Fractions keep numerator and denominator so n*size/d is computed exactly.
type CellFrac struct{ n, d float64 }

// of scales the fraction to a cell size in pixels.
func (f CellFrac) of(size float64) float64 { return f.n * size / f.d }

func (f *CellFrac) UnmarshalYAML(node *yaml.Node) error {
	num, den, isFrac := strings.Cut(node.Value, "/")
	if !isFrac {
		v, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return fmt.Errorf("line %d: invalid cell coordinate %q", node.Line, node.Value)
		}
		*f = CellFrac{v, 1}
		return f.check(node)
	}
	n, errN := strconv.Atoi(strings.TrimSpace(num))
	d, errD := strconv.Atoi(strings.TrimSpace(den))
	if errN != nil || errD != nil || d <= 0 {
		return fmt.Errorf("line %d: invalid cell fraction %q", node.Line, node.Value)
	}
	*f = CellFrac{float64(n), float64(d)}
	return f.check(node)
}

func (f CellFrac) check(node *yaml.Node) error {
	if f.n < 0 || f.n > f.d {
		return fmt.Errorf("line %d: cell coordinate %q outside [0, 1]", node.Line, node.Value)
	}
	return nil
}

// VectorShapeDef describes one glyph of a vector_linear_splits set. Rects and
// Polys form a union; Split and Invert stand alone; no field means empty.
type VectorShapeDef struct {
	Glyph  string        `yaml:"glyph"`
	Rects  [][4]CellFrac `yaml:"rects"`
	Polys  [][]string    `yaml:"polys"`
	Split  []string      `yaml:"split"`
	Invert string        `yaml:"invert"`
}

type VectorShapesSpec struct {
	Points map[string][2]CellFrac `yaml:"points"`
	Glyphs []VectorShapeDef       `yaml:"glyphs"`
}

type cellPoint struct{ x, y float64 }

// vectorShapes rasterizes the spec's vector shapes at w x h. A pixel is on when
// at least half of its 32x32 supersamples fall inside the shape (boundaries
// inclusive); invert entries are exact complements.
func vectorShapes(vs VectorShapesSpec, w, h int) ([]GlyphShape, error) {
	fw, fh := float64(w), float64(h)
	point := func(name string) (cellPoint, error) {
		p, ok := vs.Points[name]
		if !ok {
			return cellPoint{}, fmt.Errorf("unknown point %q", name)
		}
		return cellPoint{p[0].of(fw), p[1].of(fh)}, nil
	}
	evalMask := func(inside func(X, Y float64) bool) []bool {
		mask := make([]bool, w*h)
		const n = 32
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				count := 0
				for sy := 0; sy < n; sy++ {
					Y := float64(y) + (float64(sy)+0.5)/n
					for sx := 0; sx < n; sx++ {
						if inside(float64(x)+(float64(sx)+0.5)/n, Y) {
							count++
						}
					}
				}
				mask[y*w+x] = count*2 >= n*n
			}
		}
		return mask
	}

	byGlyph := make(map[rune]VectorShapeDef, len(vs.Glyphs))
	order := make([]rune, 0, len(vs.Glyphs))
	for _, def := range vs.Glyphs {
		runes := []rune(def.Glyph)
		if len(runes) != 1 {
			return nil, fmt.Errorf("vector glyph %q must be one rune", def.Glyph)
		}
		if _, dup := byGlyph[runes[0]]; dup {
			return nil, fmt.Errorf("duplicate vector glyph %q", def.Glyph)
		}
		byGlyph[runes[0]] = def
		order = append(order, runes[0])
	}

	masks := make(map[rune][]bool, len(order))
	for _, r := range order {
		def := byGlyph[r]
		kinds := 0
		for _, set := range []bool{len(def.Rects)+len(def.Polys) > 0, len(def.Split) > 0, def.Invert != ""} {
			if set {
				kinds++
			}
		}
		if kinds > 1 {
			return nil, fmt.Errorf("vector glyph %q mixes rects/polys, split and invert", def.Glyph)
		}
		if def.Invert != "" {
			continue // resolved below, once every direct shape exists
		}
		var inside func(X, Y float64) bool
		switch {
		case len(def.Split) > 0:
			if len(def.Split) != 3 {
				return nil, fmt.Errorf("vector glyph %q split needs [P1, P2, FILL]", def.Glyph)
			}
			var pts [3]cellPoint
			for i, name := range def.Split {
				p, err := point(name)
				if err != nil {
					return nil, fmt.Errorf("vector glyph %q: %w", def.Glyph, err)
				}
				pts[i] = p
			}
			p1, dx, dy := pts[0], pts[1].x-pts[0].x, pts[1].y-pts[0].y
			fillSide := (pts[2].x-p1.x)*dy - (pts[2].y-p1.y)*dx
			inside = func(X, Y float64) bool {
				s := (X-p1.x)*dy - (Y-p1.y)*dx
				if fillSide >= 0 {
					return s >= 0
				}
				return s <= 0
			}
		default:
			var parts []func(X, Y float64) bool
			for _, rc := range def.Rects {
				x0, y0, x1, y1 := rc[0].of(fw), rc[1].of(fh), rc[2].of(fw), rc[3].of(fh)
				parts = append(parts, func(X, Y float64) bool { return X >= x0 && X <= x1 && Y >= y0 && Y <= y1 })
			}
			for _, names := range def.Polys {
				if len(names) < 3 {
					return nil, fmt.Errorf("vector glyph %q polygon needs >= 3 points", def.Glyph)
				}
				poly := make([]cellPoint, len(names))
				for i, name := range names {
					p, err := point(name)
					if err != nil {
						return nil, fmt.Errorf("vector glyph %q: %w", def.Glyph, err)
					}
					poly[i] = p
				}
				parts = append(parts, func(X, Y float64) bool { return insideConvex(poly, X, Y) })
			}
			inside = func(X, Y float64) bool {
				for _, part := range parts {
					if part(X, Y) {
						return true
					}
				}
				return false
			}
		}
		masks[r] = evalMask(inside)
	}
	for _, r := range order {
		def := byGlyph[r]
		if def.Invert == "" {
			continue
		}
		target := []rune(def.Invert)
		if len(target) != 1 || byGlyph[target[0]].Invert != "" || masks[target[0]] == nil {
			return nil, fmt.Errorf("vector glyph %q inverts %q, which must be a listed non-invert glyph", def.Glyph, def.Invert)
		}
		inv := make([]bool, w*h)
		for i, on := range masks[target[0]] {
			inv[i] = !on
		}
		masks[r] = inv
	}

	shapes := make([]GlyphShape, 0, len(order))
	for _, r := range order {
		shapes = append(shapes, GlyphShape{Glyph: r, Mask: masks[r]})
	}
	return shapes, nil
}

// insideConvex reports whether (X, Y) lies in the convex polygon, edges included,
// for either winding order.
func insideConvex(poly []cellPoint, X, Y float64) bool {
	var pos, neg bool
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		c := (b.x-a.x)*(Y-a.y) - (b.y-a.y)*(X-a.x)
		pos = pos || c > 0
		neg = neg || c < 0
	}
	return !(pos && neg)
}
