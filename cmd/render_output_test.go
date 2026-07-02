package cmd

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestVisibleLineWidthsIgnoreANSIColorByteLength(t *testing.T) {
	out := "\x1b[2K\r\x1b[48;2;1;2;3m \x1b[0m\x1b[48;2;111;222;33mX\x1b[0m\n" +
		"\x1b[2K\r\x1b[38;2;9;99;199mY\x1b[0mZ\n"
	got := visibleLineWidths(out)
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

func TestRenderCheckedVideoSizedSextantJobsWidth80(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	for y := 0; y < 720; y++ {
		for x := 0; x < 1280; x++ {
			src.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / 1279),
				G: uint8(y * 255 / 719),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	rc, err := parseRenderMode("x")
	if err != nil {
		t.Fatalf("parseRenderMode(x): %v", err)
	}
	rc.jobs = 10
	vp, err := prepareRenderedImageChecked(src, nil, 80, 0, rc, "")
	if err != nil {
		t.Fatalf("prepareRenderedImageChecked: %v", err)
	}
	var out bytes.Buffer
	if err := renderChecked(&out, vp, rc); err != nil {
		t.Fatalf("renderChecked: %v", err)
	}
	if err := validateRenderedANSI(out.String(), renderCells{Cols: 80, Rows: renderedCellSize(vp, rc).Rows}, "six"); err != nil {
		t.Fatalf("validateRenderedANSI: %v", err)
	}
}

func TestRenderCheckedHalfSplitUsesTwoByTwoRows(t *testing.T) {
	rc, err := parseRenderMode("hs")
	if err != nil {
		t.Fatalf("parseRenderMode(hs): %v", err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 60, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			src.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / 59),
				G: uint8(y * 255 / 59),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	var out bytes.Buffer
	if err := renderChecked(&out, src, rc); err != nil {
		t.Fatalf("renderChecked half/split: %v", err)
	}
	if err := validateRenderedANSI(out.String(), renderedCellSize(src, rc), "half/split"); err != nil {
		t.Fatalf("validateRenderedANSI: %v", err)
	}
}

func TestValidateRenderedANSIRejectsUnevenRows(t *testing.T) {
	err := validateRenderedANSI("\x1b[2K\rXX\n\x1b[2K\rX\n", renderCells{Cols: 2, Rows: 2}, "test")
	if err == nil {
		t.Fatal("validateRenderedANSI accepted uneven rows")
	}
}
