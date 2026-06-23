package halfblock

import (
	"os"

	"golang.org/x/sys/unix"
)

// TermWidth returns the current terminal width in columns, or 0 if it cannot
// be determined (e.g. stdout is not a terminal).
func TermWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0
	}
	return int(ws.Col)
}
