package spec

import (
	"strings"
	"testing"
)

// Octant numbers from Unicode 17.0 UnicodeData.txt, U+1CD00..U+1CDE5.
// https://www.unicode.org/Public/17.0.0/ucd/UnicodeData.txt
// This oracle is independent of the generator's reserved-mask enumeration.
const unicodeOctantNumbers = "3 23 123 4 14 124 34 134 234 5 15 25 125 135 235 1235 45 145 245 1245 345 1345 2345 12345 6 16 26 126 36 136 236 1236 146 246 1246 346 1346 2346 12346 56 156 256 1256 356 1356 2356 12356 456 1456 2456 12456 3456 13456 23456 17 27 127 37 137 237 1237 47 147 247 1247 347 1347 2347 12347 157 257 1257 357 2357 12357 457 1457 12457 3457 13457 23457 67 167 267 1267 367 1367 2367 12367 467 1467 2467 12467 3467 13467 23467 123467 567 1567 2567 12567 3567 13567 23567 123567 4567 14567 24567 124567 34567 134567 234567 1234567 18 28 128 38 138 238 1238 48 148 248 1248 348 1348 2348 12348 58 158 258 1258 358 1358 2358 12358 458 1458 2458 12458 3458 13458 23458 123458 168 268 1268 368 2368 12368 468 1468 12468 3468 13468 23468 568 1568 2568 12568 3568 13568 23568 123568 4568 14568 24568 124568 34568 134568 234568 1234568 178 278 1278 378 1378 2378 12378 478 1478 2478 12478 3478 13478 23478 123478 578 1578 2578 12578 3578 13578 23578 123578 4578 14578 24578 124578 34578 134578 234578 1234578 678 1678 2678 12678 3678 13678 23678 123678 4678 14678 24678 124678 34678 134678 234678 1234678 15678 25678 125678 35678 235678 1235678 45678 145678 1245678 1345678 2345678"

func TestOctantUnicodeMapping(t *testing.T) {
	shapes := generatedOctantShapes()
	byRune := make(map[rune]int)
	for _, shape := range shapes {
		mask := 0
		for i, on := range shape.Mask {
			if on {
				mask |= 1 << i
			}
		}
		if _, exists := byRune[shape.Glyph]; exists {
			t.Fatalf("duplicate glyph %U", shape.Glyph)
		}
		byRune[shape.Glyph] = mask
	}
	numbers := strings.Fields(unicodeOctantNumbers)
	if len(numbers) != 230 || len(byRune) != 256 {
		t.Fatalf("inventory sizes native=%d total=%d", len(numbers), len(byRune))
	}
	for i, digits := range numbers {
		expected := 0
		for _, digit := range digits {
			expected |= 1 << uint(digit-'1')
		}
		glyph := rune(0x1CD00 + i)
		if got, ok := byRune[glyph]; !ok || got != expected {
			t.Errorf("%U octants %s: mask=%02X present=%v, want %02X", glyph, digits, got, ok, expected)
		}
	}
	// Unicode's remaining exact 2x4 shapes: blocks, quadrants, quarter bars,
	// middle quarter rectangles, and the four corner eighth rectangles.
	legacy := map[rune]int{
		' ': 0x00, '█': 0xFF, '▀': 0x0F, '▄': 0xF0, '▌': 0x55, '▐': 0xAA,
		'▘': 0x05, '▝': 0x0A, '▖': 0x50, '▗': 0xA0, '▚': 0xA5, '▞': 0x5A,
		'▛': 0x5F, '▜': 0xAF, '▙': 0xF5, '▟': 0xFA,
		'▂': 0xC0, '▆': 0xFC, 0x1FB82: 0x03, 0x1FB85: 0x3F,
		0x1FBE6: 0x14, 0x1FBE7: 0x28,
		0x1CEA8: 0x01, 0x1CEAB: 0x02, 0x1CEA3: 0x40, 0x1CEA0: 0x80,
	}
	for glyph, expected := range legacy {
		if got, ok := byRune[glyph]; !ok || got != expected {
			t.Errorf("%U: mask=%02X present=%v, want %02X", glyph, got, ok, expected)
		}
	}
}
