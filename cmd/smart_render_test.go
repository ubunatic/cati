package cmd

import (
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/v1/sparkline/testhelper"
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

func TestSmartCandidatePixelWidthsUsesBoundedNativeWindow(t *testing.T) {
	got := smartCandidatePixelWidths(10, 1)
	if len(got) != 2 || got[0] != 10 || got[1] != 9 {
		t.Fatalf("pixel widths(10) = %v, want [10 9]", got)
	}
	if got := smartCandidatePixelWidths(1, 1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("pixel widths(1) = %v, want [1]", got)
	}
	if got := smartCandidatePixelWidths(20, 2); len(got) != 2 || got[0] != 20 || got[1] != 18 {
		t.Fatalf("pixel widths(20, step 2) = %v, want [20 18]", got)
	}
}

func TestNativeSmartStepComesFromRenderModeSpec(t *testing.T) {
	for _, name := range []string{"half/split", "quad", "spark", "spark+quad", "six", "six+half", "spark+six"} {
		rc, err := findRenderModeByName(name)
		if err != nil {
			t.Fatal(err)
		}
		step, ok := nativeSmartStepSize(rc)
		if !ok || step != 1 {
			t.Fatalf("nativeSmartStep(%q) = false", name)
		}
	}
	rc, err := findRenderModeByName("half")
	if err != nil {
		t.Fatal(err)
	}
	if nativeSmartStep(rc) {
		t.Fatal("nativeSmartStep(half) = true, want terminal-column fallback")
	}
	rc, err = findRenderModeByName("spark")
	if err != nil {
		t.Fatal(err)
	}
	if pixels, cellW, ok := nativeSmartTerminalStep(rc); !ok || pixels != 1 || cellW != 4 {
		t.Fatalf("spark native terminal step = %d/%d, want 1/4", pixels, cellW)
	}
}

func TestFitNativeWidthAllowsSubCellRenderWidth(t *testing.T) {
	rc, err := findRenderModeByName("quad")
	if err != nil {
		t.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 20, 10))
	got, err := fitNativeWidthChecked(src, 19, 8, rc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 19 {
		t.Fatalf("native render width = %d, want 19", got.Bounds().Dx())
	}
}

func TestSmartPrepareNativeModesPreserveTargetColumns(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for _, name := range []string{"half/split", "quad", "spark", "spark+quad", "six", "six+half", "spark+six"} {
		rc, err := findRenderModeByName(name)
		if err != nil {
			t.Fatal(err)
		}
		rc.smart = true
		got, err := smartPrepare(src, 12, 8, rc)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cells := renderedCellSize(got, rc); cells.Cols != 12 {
			t.Fatalf("%s: smart width = %d, want 12", name, cells.Cols)
		}
	}
}

func TestSmartRenderGolden(t *testing.T) {
	src := goldenSourceLoad(t, "testdata/demo_checker_20x20/source.png")
	rc, err := findRenderModeByName("quad")
	if err != nil {
		t.Fatal(err)
	}
	rc.smart = true
	got, err := smartPrepare(src, 12, 12, rc)
	if err != nil {
		t.Fatal(err)
	}
	path := "testdata/demo_checker_20x20/render_smart_quad_12ch.png"
	meta := map[string]string{
		"Algorithm":     "quad",
		"Smart":         "true",
		"RequestedCols": "12",
		"CandidateStep": "1 render pixel",
		"Policy":        "psnr;max_reduction=0.10;tie_break=widest",
	}
	if *updateGolden {
		if err := testhelper.SavePNG(path, got, meta); err != nil {
			t.Fatal(err)
		}
		return
	}
	want := goldenLoad(t, path)
	if want == nil {
		t.Fatalf("smart golden missing: run with -update to create %s", path)
	}
	if !goldenEqual(got, want) {
		t.Fatalf("smart render differs from %s", path)
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

func TestNativeCandidateCanWinPSNRSelection(t *testing.T) {
	ref := image.NewRGBA(image.Rect(0, 0, 2, 1))
	ref.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	ref.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	winner, ok := smartScoreCandidates(ref, map[int]image.Image{
		20: image.NewRGBA(image.Rect(0, 0, 2, 1)),
		19: ref,
	})
	if !ok || winner.width != 19 {
		t.Fatalf("native candidate winner = %+v, %v; want pixel width 19", winner, ok)
	}
}
