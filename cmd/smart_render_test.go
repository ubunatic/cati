package cmd

import (
	"image"
	"image/color"
	"testing"
)

func TestSmartCandidateWidths(t *testing.T) {
	if got := smartCandidateWidths(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("widths(1) = %v, want [1]", got)
	}
	got := smartCandidateWidths(20)
	if len(got) != 3 || got[0] != 20 || got[2] != 18 {
		t.Fatalf("widths(20) = %v, want [20 19 18]", got)
	}
}

func TestSmartPrepareCentersAndFillsRequestedWidth(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 16; x++ {
			src.SetRGBA(x, y, color.RGBA{uint8(x * 16), uint8(y * 32), 0, 255})
		}
	}
	rc := renderCfg{smart: true}
	got, err := smartPrepare(src, 10, 8, rc)
	if err != nil {
		t.Fatal(err)
	}
	if cells := renderedCellSize(got, rc); cells.Cols != 10 {
		t.Fatalf("smart width = %d, want 10", cells.Cols)
	}
	if got.Bounds().Dy() == 0 {
		t.Fatal("smart render lost image height")
	}
}

func TestChooseSmartCandidatePrefersBestAndWidestTie(t *testing.T) {
	got, ok := chooseSmartCandidate([]smartCandidate{{width: 10, score: 2}, {width: 9, score: 3}, {width: 8, score: 3}})
	if !ok || got.width != 9 {
		t.Fatalf("winner = %+v, %v; want width 9", got, ok)
	}
}

func TestSmartScoreUsesFixedReferenceAndIgnoresPadding(t *testing.T) {
	ref := image.NewRGBA(image.Rect(0, 0, 2, 1))
	ref.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	ref.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	best := image.NewRGBA(image.Rect(0, 0, 2, 1))
	best.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	best.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	other := image.NewRGBA(image.Rect(0, 0, 1, 1))
	other.SetRGBA(0, 0, color.RGBA{0, 0, 255, 255})
	got, ok := smartScoreCandidates(ref, map[int]image.Image{7: other, 8: best})
	if !ok || got.width != 8 {
		t.Fatalf("winner = %+v, %v; want width 8", got, ok)
	}
}
