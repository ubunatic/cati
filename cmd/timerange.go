package cmd

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// TimeRange holds an optional playback window [Start, End).
// A zero value means "no restriction".
type TimeRange struct {
	Start float64 // seconds from beginning (≥ 0)
	End   float64 // seconds from beginning (0 = open-ended)
}

// IsZero reports whether the range is unconstrained.
func (r TimeRange) IsZero() bool { return r.Start == 0 && r.End == 0 }

// Duration returns the playback duration in seconds (math.MaxFloat64 if open-ended).
func (r TimeRange) Duration() float64 {
	if r.End <= 0 {
		return math.MaxFloat64
	}
	return r.End - r.Start
}

// parseTimeRange parses --range values in the following forms:
//
//	"5s"      → Start=0, End=5
//	"5s:7s"   → Start=5, End=7
//	""        → zero TimeRange (no restriction)
//
// Accepted time formats: bare seconds ("5"), duration strings ("5s", "1m30s",
// "1.5m"), or "mm:ss"/"hh:mm:ss" clock notation.
// Range separator is ":" only when at least one side carries a unit suffix
// (s/m/h) or when the left side is empty ("|:7s" → open start).
func parseTimeRange(s string) (TimeRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return TimeRange{}, nil
	}

	// Count colons to detect clock notation before attempting range split.
	colonCount := strings.Count(s, ":")

	// A single colon may be range ("5s:7s") or mm:ss ("1:30").
	// Two colons may be range with clock sides or hh:mm:ss.
	// Strategy: split on the LAST colon that is preceded by a unit suffix or
	// is the very first character (empty start), otherwise treat as clock.
	//
	// Simpler heuristic that covers all documented formats:
	//   - If either side contains a unit suffix (s/m/h), it's a range.
	//   - If the string starts with ":", it's a range with empty start.
	//   - Otherwise treat the whole token as a single time value.

	if colonCount == 0 {
		// No colon: single time value → end-only range.
		endSec, err := parseTimeSec(s)
		if err != nil {
			return TimeRange{}, fmt.Errorf("--range %q: %w", s, err)
		}
		return TimeRange{Start: 0, End: endSec}, nil
	}

	// Find the range-separator colon: the first ":" where at least one adjacent
	// token contains a unit suffix. This avoids misreading "1:30" as a range.
	sepIdx := findRangeSep(s)
	if sepIdx < 0 {
		// No range separator found → treat whole string as a single time value.
		endSec, err := parseTimeSec(s)
		if err != nil {
			return TimeRange{}, fmt.Errorf("--range %q: %w", s, err)
		}
		return TimeRange{Start: 0, End: endSec}, nil
	}

	left := s[:sepIdx]
	right := s[sepIdx+1:]
	startSec, err := parseTimeSec(left)
	if err != nil {
		return TimeRange{}, fmt.Errorf("--range %q: invalid start %q: %w", s, left, err)
	}
	endSec, err := parseTimeSec(right)
	if err != nil {
		return TimeRange{}, fmt.Errorf("--range %q: invalid end %q: %w", s, right, err)
	}
	if endSec > 0 && endSec <= startSec {
		return TimeRange{}, fmt.Errorf("--range %q: end (%gs) must be after start (%gs)", s, endSec, startSec)
	}
	return TimeRange{Start: startSec, End: endSec}, nil
}

// findRangeSep returns the index of the colon that acts as the range
// separator in s, or -1 if s should be treated as a plain time value.
//
// A colon is a range separator when the substring to its LEFT or RIGHT
// contains a unit suffix (s, m, h), or when it is the very first character
// (empty start → open-ended).
func findRangeSep(s string) int {
	for i, c := range s {
		if c != ':' {
			continue
		}
		left := s[:i]
		right := s[i+1:]
		if hasUnitSuffix(left) || hasUnitSuffix(right) || left == "" {
			return i
		}
	}
	return -1
}

// hasUnitSuffix reports whether s contains a time unit letter (s, m, h)
// that is NOT part of a purely numeric token (e.g. "5s" → true, "05" → false).
func hasUnitSuffix(s string) bool {
	for _, c := range s {
		if c == 's' || c == 'm' || c == 'h' {
			return true
		}
	}
	return false
}

// parseTimeSec parses a time string and returns the value in seconds.
//
// Accepted forms:
//   - "" → 0 (open-ended marker)
//   - "5" or "5.5" → bare seconds (no letters allowed)
//   - "5s", "1m30s", "1h5m3s", "1.5m" → Go duration (time.ParseDuration)
//   - "mm:ss" or "hh:mm:ss" → clock notation
func parseTimeSec(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}

	// Try Go duration first (handles "5s", "1m30s", "1.5h", etc.).
	if d, err := time.ParseDuration(s); err == nil {
		return d.Seconds(), nil
	}

	// Try clock notation: [hh:]mm:ss[.frac] — must contain exactly one or two colons.
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		switch len(parts) {
		case 2:
			m, err1 := parseFloat(parts[0])
			sec, err2 := parseFloat(parts[1])
			if err1 != nil || err2 != nil {
				return 0, fmt.Errorf("invalid time %q", s)
			}
			return m*60 + sec, nil
		case 3:
			h, err1 := parseFloat(parts[0])
			m, err2 := parseFloat(parts[1])
			sec, err3 := parseFloat(parts[2])
			if err1 != nil || err2 != nil || err3 != nil {
				return 0, fmt.Errorf("invalid time %q", s)
			}
			return h*3600 + m*60 + sec, nil
		default:
			return 0, fmt.Errorf("invalid time %q", s)
		}
	}

	// Bare numeric seconds — reject anything with non-numeric, non-decimal chars.
	for _, c := range s {
		if (c < '0' || c > '9') && c != '.' {
			return 0, fmt.Errorf("invalid time %q (use e.g. 5s, 1m30s, 1:30)", s)
		}
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return f, nil
}

func parseFloat(s string) (float64, error) {
	for _, c := range s {
		if (c < '0' || c > '9') && c != '.' {
			return 0, fmt.Errorf("invalid number %q", s)
		}
	}
	var f float64
	_, err := fmt.Sscanf(s, "%g", &f)
	return f, err
}
