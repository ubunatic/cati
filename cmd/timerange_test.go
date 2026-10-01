package cmd

import (
	"math"
	"testing"
)

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		input     string
		wantStart float64
		wantEnd   float64
		wantErr   bool
	}{
		// Empty → no restriction
		{"", 0, 0, false},

		// Single token (end only)
		{"5s", 0, 5, false},
		{"5", 0, 5, false},
		{"1m30s", 0, 90, false},
		{"1.5m", 0, 90, false},
		{"0:05", 0, 5, false},
		{"1:30", 0, 90, false},
		{"1:00:00", 0, 3600, false},

		// Range form start:end
		{"5s:7s", 5, 7, false},
		{"0s:5s", 0, 5, false},
		{"1m:1m30s", 60, 90, false},

		// Open-ended start (":7s" → start=0, end=7)
		{":7s", 0, 7, false},

		// Errors
		{"7s:5s", 0, 0, true},  // end before start
		{"5s:5s", 0, 0, true},  // end == start
		{"bad", 0, 0, true},    // unparseable
		{"1x:2y", 0, 0, true},  // bad units
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseTimeRange(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseTimeRange(%q): expected error, got %+v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTimeRange(%q): unexpected error: %v", tc.input, err)
			}
			if math.Abs(got.Start-tc.wantStart) > 0.001 {
				t.Errorf("parseTimeRange(%q).Start = %.4f, want %.4f", tc.input, got.Start, tc.wantStart)
			}
			if math.Abs(got.End-tc.wantEnd) > 0.001 {
				t.Errorf("parseTimeRange(%q).End = %.4f, want %.4f", tc.input, got.End, tc.wantEnd)
			}
		})
	}
}
