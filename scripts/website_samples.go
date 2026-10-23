// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

var sampleGroups = [][]string{
	{
		"assets/samples/sample-001-soldering-practice-2025.jpg",
		"testdata/sample_soldering_practice/render_halfblock_80ch.png",
		"testdata/sample_soldering_practice/render_quad_80ch.png",
		"testdata/sample_soldering_practice/render_spark_80ch.png",
	},
	{
		"assets/samples/sample-002-summer-vacation.jpg",
		"testdata/sample_summer_vacation/render_halfblock_50ch.png",
		"testdata/sample_summer_vacation/render_quad_50ch.png",
		"testdata/sample_summer_vacation/render_spark_50ch.png",
	},
	{
		"assets/samples/sample-003-darth-daughter.jpg",
		"testdata/sample_darth_daughter/render_halfblock_50ch.png",
		"testdata/sample_darth_daughter/render_quad_50ch.png",
		"testdata/sample_darth_daughter/render_spark_50ch.png",
	},
}

func main() {
	for _, group := range sampleGroups {
		if err := generateGroup(group); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func generateGroup(paths []string) error {
	photo, err := decode(filepath.Join(".", paths[0]))
	if err != nil {
		return fmt.Errorf("decode %s: %w", paths[0], err)
	}
	firstRender, err := decode(filepath.Join("website", paths[1]))
	if err != nil {
		return fmt.Errorf("decode %s: %w", paths[1], err)
	}

	renderBounds := firstRender.Bounds()
	target := image.Rectangle{Max: renderBounds.Size()}

	for _, rel := range paths[1:] {
		render, err := decode(filepath.Join("website", rel))
		if err != nil {
			return fmt.Errorf("decode %s: %w", rel, err)
		}
		if render.Bounds().Size() != renderBounds.Size() {
			return fmt.Errorf("%s size %v differs from group render size %v", rel, render.Bounds().Size(), renderBounds.Size())
		}
		out := image.NewRGBA(image.Rect(0, 0, target.Dx(), target.Dy()))
		draw.NearestNeighbor.Scale(out, out.Bounds(), render, render.Bounds(), draw.Src, nil)
		if err := encodePNG(filepath.Join("website", rel), out); err != nil {
			return err
		}
	}

	photoOut := filepath.Join("website", paths[0])
	if err := encodeJPEG(photoOut, fitAndPad(photo, target), 95); err != nil {
		return err
	}
	fmt.Printf("%s factor=1 target=%dx%d\n", paths[0], target.Dx(), target.Dy())
	return nil
}

func fitAndPad(src image.Image, target image.Rectangle) *image.RGBA {
	bounds := src.Bounds()
	widthScale := float64(target.Dx()) / float64(bounds.Dx())
	heightScale := float64(target.Dy()) / float64(bounds.Dy())
	scale := min(widthScale, heightScale)
	width := max(1, int(math.Round(float64(bounds.Dx())*scale)))
	height := max(1, int(math.Round(float64(bounds.Dy())*scale)))
	resized := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(resized, resized.Bounds(), src, bounds, draw.Src, nil)

	out := image.NewRGBA(target)
	offset := image.Pt((target.Dx()-width)/2, (target.Dy()-height)/2)
	draw.Draw(out, image.Rectangle{Min: offset, Max: offset.Add(resized.Bounds().Size())}, resized, image.Point{}, draw.Src)
	for y := 0; y < target.Dy(); y++ {
		for x := 0; x < target.Dx(); x++ {
			if x >= offset.X && x < offset.X+width && y >= offset.Y && y < offset.Y+height {
				continue
			}
			sx := min(max(x-offset.X, 0), width-1)
			sy := min(max(y-offset.Y, 0), height-1)
			out.Set(x, y, color.RGBAModel.Convert(resized.At(sx, sy)))
		}
	}
	return out
}

func decode(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err
}

func encodeJPEG(path string, img image.Image, quality int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: quality}); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func encodePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
