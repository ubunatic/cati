package ansi

import "testing"

func TestVisibleLineWidthsIgnoreANSIColorByteLength(t *testing.T) {
	out := "\x1b[2K\r\x1b[48;2;1;2;3m \x1b[0m\x1b[48;2;111;222;33mX\x1b[0m\n" +
		"\x1b[2K\r\x1b[38;2;9;99;199mY\x1b[0mZ\n"
	got := LineWidths(out)
	want := []int{2, 2}
	if len(got) != len(want) {
		t.Fatalf("visibleLineWidths len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("visibleLineWidths[%d] = %d, want %d (%v)", i, got[i], want[i], got)
		}
	}
}
