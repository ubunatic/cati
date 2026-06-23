package halfblock

import (
	"fmt"
	"io"
)

// ── Terminal control sequences ────────────────────────────────────────────────

const (
	// ANSIHideCursor hides the terminal cursor.
	ANSIHideCursor = "\x1b[?25l"
	// ANSIShowCursor restores the terminal cursor.
	ANSIShowCursor = "\x1b[?25h"
	// ANSIClearScreen erases the display and moves the cursor to the home position.
	ANSIClearScreen = "\x1b[2J\x1b[H"
	// ANSICursorHome moves the cursor to the top-left (0,0) without clearing.
	ANSICursorHome = "\x1b[H"
	// ANSIEraseDown clears from the cursor to the end of the screen.
	ANSIEraseDown = "\x1b[J"

	// ANSIMouseOn enables button-event mouse tracking (clicks, drag, scroll) and
	// SGR extended coordinate encoding (required for terminals wider than 223 cols
	// and for reliable button/position parsing).
	// ?1002h = button-event tracking (reports movement while a button is held).
	// ?1006h = SGR extended coordinates.
	ANSIMouseOn = "\x1b[?1002h\x1b[?1006h"
	// ANSIMouseOff disables the modes enabled by ANSIMouseOn.
	ANSIMouseOff = "\x1b[?1002l\x1b[?1006l"
)

// HideCursor writes the hide-cursor escape to w.
func HideCursor(w io.Writer) { fmt.Fprint(w, ANSIHideCursor) }

// ShowCursor writes the show-cursor escape to w.
func ShowCursor(w io.Writer) { fmt.Fprint(w, ANSIShowCursor) }

// ClearScreen clears the screen and homes the cursor.
func ClearScreen(w io.Writer) { fmt.Fprint(w, ANSIClearScreen) }

// CursorHome moves the cursor to (0,0) without clearing the screen.
func CursorHome(w io.Writer) { fmt.Fprint(w, ANSICursorHome) }

// EraseDown erases from the cursor to the end of the screen.
func EraseDown(w io.Writer) { fmt.Fprint(w, ANSIEraseDown) }

// EnableMouse enables normal mouse tracking with SGR extended coordinates.
func EnableMouse(w io.Writer) { fmt.Fprint(w, ANSIMouseOn) }

// DisableMouse disables mouse tracking.
func DisableMouse(w io.Writer) { fmt.Fprint(w, ANSIMouseOff) }
