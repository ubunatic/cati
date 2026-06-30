// Package sparkline renders scalar values as Unicode block-element sparklines.
//
// Two modes are provided:
//
//   - Vertical:  ▁▂▃▄▅▆▇█  (bars grow upward from cell bottom)
//   - Quad:      sparkline fractional blocks plus quad/half/full block masks
//
// Each cell encodes one value in [0,1].  The caller is responsible for
// setting fg (bar/filled colour) and bg (empty colour) per cell.
package sparkline

type Mode int

const (
	Vertical Mode = iota
	Quad
	Sextant
	Best
)

func ModeName(m Mode) string {
	switch m {
	case Vertical:
		return "spark/vert"
	case Quad:
		return "spark/quad"
	case Sextant:
		return "spark/sextant"
	case Best:
		return "spark/best"
	default:
		return "spark/vert"
	}
}

func Modes() []Mode {
	return []Mode{Vertical, Quad, Sextant, Best}
}

func Cycle(m Mode) Mode {
	ms := Modes()
	for i, v := range ms {
		if v == m {
			return ms[(i+1)%len(ms)]
		}
	}
	return ms[0]
}

func CyclePrev(m Mode) Mode {
	ms := Modes()
	n := len(ms)
	for i, v := range ms {
		if v == m {
			return ms[(i+n-1)%n]
		}
	}
	return ms[n-1]
}

var lowerBlocks = [...]rune{
	'\u2581',
	'\u2582',
	'\u2583',
	'\u2584',
	'\u2585',
	'\u2586',
	'\u2587',
	'\u2588',
}

type Cell struct {
	Ch       rune
	SwapFgBg bool
	Value    float64
}

func Char(m Mode, v float64) (r rune, swapFgBg bool) {
	idx := level(v)
	switch m {
	case Vertical:
		return lowerBlocks[idx], false
	default:
		return lowerBlocks[idx], false
	}
}

func RenderCells(m Mode, values []float64) []Cell {
	cells := make([]Cell, len(values))
	for i, v := range values {
		ch, swap := Char(m, v)
		cells[i] = Cell{Ch: ch, SwapFgBg: swap, Value: v}
	}
	return cells
}

func String(m Mode, values []float64) string {
	cells := RenderCells(m, values)
	out := make([]rune, len(cells))
	for i, c := range cells {
		out[i] = c.Ch
	}
	return string(out)
}

func level(v float64) int {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 7
	}
	idx := int(v * 8)
	if idx > 7 {
		idx = 7
	}
	return idx
}
