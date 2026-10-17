package modes

import (
	"strings"
	"testing"
)

func TestAutoFitGridCols(t *testing.T) {
	card30 := []string{"123456789012345678901234567890"}
	cards := [][]string{card30, card30, card30}

	tests := []struct {
		name      string
		cards     [][]string
		termWidth int
		wantCols  int
	}{
		{
			name:      "zero or negative term width defaults to 1",
			cards:     cards,
			termWidth: 0,
			wantCols:  1,
		},
		{
			name:      "narrow term width fits 1 col",
			cards:     cards,
			termWidth: 50,
			wantCols:  1,
		},
		{
			name:      "medium term width fits 2 cols",
			cards:     cards,
			termWidth: 70, // 2*30 + 4 = 64 <= 70
			wantCols:  2,
		},
		{
			name:      "wide term width fits 3 cols",
			cards:     cards,
			termWidth: 100, // 3*30 + 8 = 98 <= 100
			wantCols:  3,
		},
		{
			name:      "empty cards returns 1",
			cards:     nil,
			termWidth: 120,
			wantCols:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AutoFitGridCols(tt.cards, tt.termWidth)
			if got != tt.wantCols {
				t.Errorf("AutoFitGridCols(width=%d) = %d, want %d", tt.termWidth, got, tt.wantCols)
			}
		})
	}
}

func TestFormatCardsGrid(t *testing.T) {
	cardA := []string{
		"Card A Title",
		"Line A1",
		"Line A2",
	}
	cardB := []string{
		"Card B Title",
		"Line B1",
	}
	cardC := []string{
		"Card C Title",
		"Line C1",
		"Line C2",
		"Line C3",
	}

	t.Run("1 column formatting", func(t *testing.T) {
		lines := FormatCardsGrid([][]string{cardA, cardB}, 1, 80)
		// Row 1: blank line + 3 lines of card A
		// Row 2: blank line + 2 lines of card B
		if len(lines) != 7 {
			t.Fatalf("expected 7 lines, got %d: %v", len(lines), lines)
		}
		if lines[0] != "" || lines[1] != "Card A Title" || lines[4] != "" || lines[5] != "Card B Title" {
			t.Errorf("unexpected 1-col output: %v", lines)
		}
	})

	t.Run("2 column formatting with padding and gutter", func(t *testing.T) {
		lines := FormatCardsGrid([][]string{cardA, cardB}, 2, 80)
		// 1 row: blank line + max(3, 2) = 3 lines
		if len(lines) != 4 {
			t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
		}
		if lines[0] != "" {
			t.Errorf("expected leading blank line, got %q", lines[0])
		}
		// Col A title (12 chars) -> padded to 12 + 4 spaces gutter + Col B title
		if !strings.HasPrefix(lines[1], "Card A Title    Card B Title") {
			t.Errorf("line 1 mismatch: %q", lines[1])
		}
		if !strings.HasPrefix(lines[2], "Line A1         Line B1") {
			t.Errorf("line 2 mismatch: %q", lines[2])
		}
		// Card B has no line 3, so only Line A2 should be printed (trimmed right)
		if lines[3] != "Line A2" {
			t.Errorf("line 3 mismatch: %q, want %q", lines[3], "Line A2")
		}
	})

	t.Run("3 column formatting", func(t *testing.T) {
		lines := FormatCardsGrid([][]string{cardA, cardB, cardC}, 3, 120)
		if len(lines) != 5 {
			t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
		}
		if !strings.Contains(lines[1], "Card A Title") || !strings.Contains(lines[1], "Card B Title") || !strings.Contains(lines[1], "Card C Title") {
			t.Errorf("row line 1 should contain all 3 titles: %q", lines[1])
		}
	})

	t.Run("ANSI formatting preserves visual column positions", func(t *testing.T) {
		cardANSI := []string{
			"\x1b[31mRed\x1b[0m",
			"Line1",
		}
		cardPlain := []string{
			"Plain",
			"Line2",
		}
		lines := FormatCardsGrid([][]string{cardANSI, cardPlain}, 2, 80)
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
		// "Red" visual width is 3. Max width in col 0 is max(3, 5)=5.
		// "Red" + 2 spaces + 4 gutter spaces + "Plain"
		// Visual width of col 0 is 5. Position of "Plain" starts at col 5+4 = 9.
		if !strings.Contains(lines[1], "\x1b[31mRed\x1b[0m  ") {
			t.Errorf("ANSI padding incorrect in %q", lines[1])
		}
	})
}
