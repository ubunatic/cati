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
