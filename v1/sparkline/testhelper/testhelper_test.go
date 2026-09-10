package testhelper

import (
	"bytes"
	"image"
	"os"
	"testing"
)

func TestSavePNGMetadataOrderIsDeterministic(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	metadata := map[string]string{"Zeta": "last", "Alpha": "first", "Mode": "half"}
	first := t.TempDir() + "/first.png"
	second := t.TempDir() + "/second.png"
	if err := SavePNG(first, img, metadata); err != nil {
		t.Fatal(err)
	}
	if err := SavePNG(second, img, metadata); err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("SavePNG output changed metadata chunk order")
	}
}
