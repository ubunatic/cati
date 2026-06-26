// Package sparkline renders scalar values as Unicode block-element sparklines.
//
// Four modes are provided, differing in the block orientation:
//
//   - LowerHorizontal:  ▁▂▃▄▅▆▇█  (bars grow upward from cell bottom)
//   - UpperHorizontal:  same chars, fg↔bg swap (bars grow downward from top)
//   - LeftVertical:     ▏▎▍▌▋▊▉█  (bars grow rightward from cell left)
//   - RightVertical:    same chars, fg↔bg swap (bars grow leftward from right)
//
// Unicode defines lower and left fractional blocks with 1/8 resolution
// (U+2581–U+2588, U+2589–U+258F).  Finer upper/right blocks do not exist,
// so UpperHorizontal and RightVertical reuse the lower/left chars with
// foreground and background colours swapped: the "filled" region behind
// the block glyph becomes the visible bar, and the glyph foreground becomes
// the empty space.
//
// Each cell encodes one value in [0,1].  The caller is responsible for
// setting fg (bar/filled colour) and bg (empty colour) per cell, and for
// swapping them when Cell.SwapFgBg is true.
package sparkline

type Mode int

const (
	LowerHorizontal Mode = iota
	UpperHorizontal
	LeftVertical
	RightVertical
)

func ModeName(m Mode) string {
	switch m {
	case LowerHorizontal:
		return "spark/lower"
	case UpperHorizontal:
		return "spark/upper"
	case LeftVertical:
		return "spark/left"
	case RightVertical:
		return "spark/right"
	default:
		return "spark/lower"
	}
}

func Modes() []Mode {
	return []Mode{LowerHorizontal, UpperHorizontal, LeftVertical, RightVertical}
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

var leftBlocks = [...]rune{
	'\u258F',
	'\u258E',
	'\u258D',
	'\u258C',
	'\u258B',
	'\u258A',
	'\u2589',
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
	case LowerHorizontal:
		return lowerBlocks[idx], false
	case UpperHorizontal:
		return lowerBlocks[level(1-v)], true
	case LeftVertical:
		return leftBlocks[idx], false
	case RightVertical:
		return leftBlocks[level(1-v)], true
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
