package braille

// Mode selects the braille candidate search and coloring strategy.
type Mode int

const (
	// ModeDots uses 2x4 braille dot positions where foreground-colored dots
	// represent pixels, and the undotted area is transparent (dot-only).
	ModeDots Mode = iota

	// ModeForegroundBackground uses full Unicode braille cells: foreground-colored
	// dots and a background color fill for the undotted area.
	ModeForegroundBackground
)

func (m Mode) String() string {
	switch m {
	case ModeDots:
		return "braille/dots"
	case ModeForegroundBackground:
		return "braille"
	default:
		return "braille"
	}
}
