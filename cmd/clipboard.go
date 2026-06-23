package cmd

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os/exec"
)

// copyImageToClipboard encodes img as PNG and writes it to the system clipboard.
//
// It tries the following tools in order and uses the first one found:
//  1. wl-copy  (Wayland / wl-clipboard)
//  2. xclip    (X11)
//  3. xsel     (X11, fallback)
//
// Returns an error if encoding fails or no clipboard tool is available / succeeds.
func copyImageToClipboard(img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}
	data := buf.Bytes()

	type tool struct {
		name string
		args []string
	}
	tools := []tool{
		{"wl-copy", []string{"--type", "image/png"}},
		{"xclip", []string{"-selection", "clipboard", "-t", "image/png", "-i"}},
		{"xsel", []string{"--clipboard", "--input"}},
	}

	var lastErr error
	for _, t := range tools {
		path, err := exec.LookPath(t.name)
		if err != nil {
			continue // tool not installed
		}
		cmd := exec.Command(path, t.args...)
		cmd.Stdin = bytes.NewReader(data)
		// Suppress any diagnostic output so it doesn't pollute the terminal display.
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("%s: %w", t.name, err)
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no clipboard tool found (install wl-copy or xclip)")
}
