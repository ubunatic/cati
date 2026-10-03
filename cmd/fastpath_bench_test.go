package cmd

import (
	"io"
	"testing"

	"ubunatic.com/cati/v1/halfblock"
)

func BenchmarkFastpathH2vsHalf(b *testing.B) {
	img, err := halfblock.LoadImage("assets/doom1.png")
	if err != nil {
		b.Fatalf("LoadImage: %v", err)
	}

	rcHalf, _ := parseRenderMode("half")
	rcH2, _ := parseRenderMode("h2")

	b.Run("half_std", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prepared, _ := prepareExplicitGridImage(img, 320, 100, rcHalf, "default")
			_ = renderChecked(io.Discard, prepared, rcHalf)
		}
	})

	b.Run("h2_fastpath", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prepared, _ := prepareExplicitGridImage(img, 320, 100, rcH2, "default")
			_ = renderChecked(io.Discard, prepared, rcH2)
		}
	})
}

func BenchmarkFastpathS2vsSix(b *testing.B) {
	img, err := halfblock.LoadImage("assets/doom1.png")
	if err != nil {
		b.Fatalf("LoadImage: %v", err)
	}
	padded := padSourceImage(img, 0, 1)

	rcSix, _ := parseRenderMode("six")
	rcS2, _ := parseRenderMode("s2")

	b.Run("six_std", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prepared, _ := prepareExplicitGridImage(padded, 160, 67, rcSix, "aligned")
			_ = renderChecked(io.Discard, prepared, rcSix)
		}
	})

	b.Run("s2_fastpath", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prepared, _ := prepareExplicitGridImage(padded, 160, 67, rcS2, "aligned")
			_ = renderChecked(io.Discard, prepared, rcS2)
		}
	})
}
