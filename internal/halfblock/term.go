package halfblock

import (
	"os"

	"golang.org/x/sys/unix"
)

// termSize returns the current terminal dimensions via a single ioctl call.
// Returns (0, 0) when stdout is not a terminal or the call fails.
func termSize() (cols, rows int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}
	return int(ws.Col), int(ws.Row)
}

// TermWidth returns the current terminal width in columns, or 0 if it cannot
// be determined (e.g. stdout is not a terminal).
func TermWidth() int {
	cols, _ := termSize()
	return cols
}

// TermHeight returns the current terminal height in rows, or 0 if it cannot
// be determined (e.g. stdout is not a terminal).
func TermHeight() int {
	_, rows := termSize()
	return rows
}
