package cmd

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runComplete(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	fullArgs := append([]string{"__complete"}, args...)
	root.SetArgs(fullArgs)
	_, err := root.ExecuteC()
	return buf.String(), err
}

func TestCompletion_RootPositionalFiles(t *testing.T) {
	root := New()
	out, err := runComplete(root, "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}

	// Should contain subcommands
	if !strings.Contains(out, "play\t") {
		t.Errorf("expected 'play' subcommand in completion output, got:\n%s", out)
	}
	if !strings.Contains(out, "browse\t") {
		t.Errorf("expected 'browse' subcommand in completion output, got:\n%s", out)
	}
	if !strings.Contains(out, "modes\t") {
		t.Errorf("expected 'modes' subcommand in completion output, got:\n%s", out)
	}

	// Subcommand prefix matching
	outPrefix, err := runComplete(New(), "pl")
	if err != nil {
		t.Fatalf("runComplete prefix error: %v", err)
	}
	if !strings.Contains(outPrefix, "play\t") {
		t.Errorf("expected 'play' subcommand for 'pl' prefix, got:\n%s", outPrefix)
	}

	// Should contain all ticket-listed image extensions (both lower and upper)
	requiredExts := []string{
		"png", "PNG",
		"jpg", "JPG",
		"jpeg", "JPEG",
		"svg", "SVG",
		"webp", "WEBP",
		"gif", "GIF",
		"bmp", "BMP",
		"tiff", "TIFF",
		"tif", "TIF",
	}
	for _, ext := range requiredExts {
		if !strings.Contains(out, "\n"+ext+"\n") && !strings.HasPrefix(out, ext+"\n") {
			t.Errorf("expected extension %q in completion output, got:\n%s", ext, out)
		}
	}

	// Should end with ShellCompDirectiveFilterFileExt (directive :8)
	if !strings.Contains(out, ":8") {
		t.Errorf("expected directive :8 (ShellCompDirectiveFilterFileExt) in completion output, got:\n%s", out)
	}
}

func TestCompletion_PlayFlagAndModes(t *testing.T) {
	// 1. Positional completion with --play flag active (expecting play mode, directive :4)
	root := New()
	out, err := runComplete(root, "--play", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}
	if !strings.Contains(out, "once\t") || !strings.Contains(out, "repeat\t") || !strings.Contains(out, "preview\t") {
		t.Errorf("expected play modes in --play completion output, got:\n%s", out)
	}
	if !strings.Contains(out, ":4") {
		t.Errorf("expected directive :4 (NoFileComp) when completing play mode, got:\n%s", out)
	}

	// 2. Positional completion with play mode already provided (transitions to file filter directive :8)
	root2 := New()
	out2, err := runComplete(root2, "--play", "once", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}
	if strings.Contains(out2, "repeat\t") || strings.Contains(out2, "preview\t") {
		t.Errorf("expected play modes excluded after mode chosen, got:\n%s", out2)
	}
	if !strings.Contains(out2, ":8") {
		t.Errorf("expected directive :8 (FilterFileExt) after play mode chosen, got:\n%s", out2)
	}

	// 3. Direct flag completion for --play= (directive :4)
	root3 := New()
	out3, err := runComplete(root3, "--play=")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}
	if !strings.Contains(out3, "once\t") || !strings.Contains(out3, "repeat\t") || !strings.Contains(out3, "preview\t") {
		t.Errorf("expected play modes in --play= completion output, got:\n%s", out3)
	}
	if !strings.Contains(out3, ":4") {
		t.Errorf("expected directive :4 (ShellCompDirectiveNoFileComp) for --play=, got:\n%s", out3)
	}

	// 4. Short alias -p=
	root4 := New()
	out4, err := runComplete(root4, "-p=")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}
	if !strings.Contains(out4, "once\t") || !strings.Contains(out4, ":4") {
		t.Errorf("expected play modes for short flag -p=, got:\n%s", out4)
	}
}

func TestCompletion_AspectFlag(t *testing.T) {
	root := New()
	out, err := runComplete(root, "--aspect", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}

	if !strings.Contains(out, "default\t") || !strings.Contains(out, "aligned\t") {
		t.Errorf("expected aspect modes in completion output, got:\n%s", out)
	}
	if !strings.Contains(out, ":4") {
		t.Errorf("expected directive :4 in --aspect completion output, got:\n%s", out)
	}
}

func TestCompletion_ModeFlagMatchesRegistry(t *testing.T) {
	root := New()
	out, err := runComplete(root, "--mode", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}

	// Every render mode in runtime registry must be suggested
	for _, m := range renderModes {
		if !strings.Contains(out, m.name+"\t") && !strings.Contains(out, "\n"+m.name+"\n") {
			t.Errorf("expected registered render mode %q in --mode completions, got:\n%s", m.name, out)
		}
		for _, a := range m.aliases {
			if !strings.Contains(out, a+"\t") && !strings.Contains(out, "\n"+a+"\n") {
				t.Errorf("expected alias %q for mode %q in --mode completions, got:\n%s", a, m.name, out)
			}
		}
	}

	// Short flag -m ""
	rootShort := New()
	outShort, err := runComplete(rootShort, "-m", "")
	if err != nil {
		t.Fatalf("runComplete -m error: %v", err)
	}
	if !strings.Contains(outShort, "half\t") || !strings.Contains(outShort, ":4") {
		t.Errorf("expected modes in -m completion, got:\n%s", outShort)
	}
}

func TestCompletion_PrescalerFlag(t *testing.T) {
	root := New()
	out, err := runComplete(root, "--prescaler", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}

	if !strings.Contains(out, "nearest-neighbor\t") || !strings.Contains(out, "nn\t") || !strings.Contains(out, "pyramid\t") {
		t.Errorf("expected prescaler options in completion output, got:\n%s", out)
	}
	if !strings.Contains(out, ":4") {
		t.Errorf("expected directive :4 in --prescaler completion output, got:\n%s", out)
	}

	// Short flag -S ""
	rootShort := New()
	outShort, err := runComplete(rootShort, "-S", "")
	if err != nil {
		t.Fatalf("runComplete -S error: %v", err)
	}
	if !strings.Contains(outShort, "pyramid\t") || !strings.Contains(outShort, ":4") {
		t.Errorf("expected prescaler options for -S, got:\n%s", outShort)
	}
}

func TestCompletion_CropFlag(t *testing.T) {
	root := New()
	out, err := runComplete(root, "--crop", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}

	if !strings.Contains(out, "auto\t") || !strings.Contains(out, "center\t") || !strings.Contains(out, "W:H\t") {
		t.Errorf("expected crop options in completion output, got:\n%s", out)
	}
	if !strings.Contains(out, ":4") {
		t.Errorf("expected directive :4 in --crop completion output, got:\n%s", out)
	}

	// Short flag -c ""
	rootShort := New()
	outShort, err := runComplete(rootShort, "-c", "")
	if err != nil {
		t.Fatalf("runComplete -c error: %v", err)
	}
	if !strings.Contains(outShort, "auto\t") || !strings.Contains(outShort, ":4") {
		t.Errorf("expected crop options for -c, got:\n%s", outShort)
	}
}

func TestCompletion_ZoomFlag(t *testing.T) {
	root := New()
	out, err := runComplete(root, "--zoom", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}

	if !strings.Contains(out, "0\t") || !strings.Contains(out, "1\t") || !strings.Contains(out, "1:1\t") || !strings.Contains(out, "w\t") || !strings.Contains(out, "h\t") {
		t.Errorf("expected zoom options in completion output, got:\n%s", out)
	}
	if !strings.Contains(out, ":4") {
		t.Errorf("expected directive :4 in --zoom completion output, got:\n%s", out)
	}

	// Short flag -z ""
	rootShort := New()
	outShort, err := runComplete(rootShort, "-z", "")
	if err != nil {
		t.Fatalf("runComplete -z error: %v", err)
	}
	if !strings.Contains(outShort, "1:1\t") || !strings.Contains(outShort, ":4") {
		t.Errorf("expected zoom options for -z, got:\n%s", outShort)
	}
}

func TestCompletion_RangeFlag(t *testing.T) {
	root := New()
	out, err := runComplete(root, "--range", "")
	if err != nil {
		t.Fatalf("runComplete error: %v", err)
	}

	if !strings.Contains(out, "5s\t") || !strings.Contains(out, "5s:7s\t") {
		t.Errorf("expected range options in completion output, got:\n%s", out)
	}
	if !strings.Contains(out, ":4") {
		t.Errorf("expected directive :4 in --range completion output, got:\n%s", out)
	}
}

func TestCompletion_ShellScriptGeneration(t *testing.T) {
	root := New()

	// Bash completion script generation
	var bashBuf bytes.Buffer
	if err := root.GenBashCompletionV2(&bashBuf, true); err != nil {
		t.Fatalf("GenBashCompletionV2 failed: %v", err)
	}
	if bashBuf.Len() == 0 || !strings.Contains(bashBuf.String(), "cati") {
		t.Errorf("expected non-empty bash completion script")
	}

	// Zsh completion script generation
	var zshBuf bytes.Buffer
	if err := root.GenZshCompletion(&zshBuf); err != nil {
		t.Fatalf("GenZshCompletion failed: %v", err)
	}
	if zshBuf.Len() == 0 || !strings.Contains(zshBuf.String(), "cati") {
		t.Errorf("expected non-empty zsh completion script")
	}

	// Fish completion script generation
	var fishBuf bytes.Buffer
	if err := root.GenFishCompletion(&fishBuf, true); err != nil {
		t.Fatalf("GenFishCompletion failed: %v", err)
	}
	if fishBuf.Len() == 0 || !strings.Contains(fishBuf.String(), "cati") {
		t.Errorf("expected non-empty fish completion script")
	}
}

func TestSupportedImageExtensions(t *testing.T) {
	exts := supportedImageExtensions()
	for ext := range imageExts {
		clean := strings.TrimPrefix(ext, ".")
		lower := strings.ToLower(clean)
		upper := strings.ToUpper(clean)
		if !slices.Contains(exts, lower) {
			t.Errorf("supportedImageExtensions missing lower %q", lower)
		}
		if !slices.Contains(exts, upper) {
			t.Errorf("supportedImageExtensions missing upper %q", upper)
		}
	}
}
