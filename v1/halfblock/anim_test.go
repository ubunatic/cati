package halfblock

import (
	"bytes"
	"io"
	"testing"
)

func TestTerminalControlSequences(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(io.Writer)
		expected string
	}{
		{
			name:     "HideCursor",
			fn:       HideCursor,
			expected: ANSIHideCursor,
		},
		{
			name:     "ShowCursor",
			fn:       ShowCursor,
			expected: ANSIShowCursor,
		},
		{
			name:     "ClearScreen",
			fn:       ClearScreen,
			expected: ANSIClearScreen,
		},
		{
			name:     "CursorHome",
			fn:       CursorHome,
			expected: ANSICursorHome,
		},
		{
			name:     "EraseDown",
			fn:       EraseDown,
			expected: ANSIEraseDown,
		},
		{
			name:     "EnableMouse",
			fn:       EnableMouse,
			expected: ANSIMouseOn,
		},
		{
			name:     "DisableMouse",
			fn:       DisableMouse,
			expected: ANSIMouseOff,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tc.fn(&buf)
			if got := buf.String(); got != tc.expected {
				t.Errorf("%s() = %q, want %q", tc.name, got, tc.expected)
			}
		})
	}
}
