package core

import "image/color"

type Cell struct {
	Ch          rune
	Fg          color.RGBA
	Bg          color.RGBA
	HasFg       bool
	HasBg       bool
	Transparent bool
}

type Grid struct {
	Width  int
	Height int
	Cells  [][]Cell
}
