package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"
	"unsafe"
)

func TestInteractiveSmallGradientViaPipedStdin(t *testing.T) {
	if _, err := os.Stat("testdata/gradient_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	tests := []struct {
		name  string
		input string
	}{
		{"cycle to spark", "r\nr\nr\nq\n"},
		{"zoom out", "-\n-\n-\nq\n"},
		{"spark then zoom out", "r\nr\nr\n-\n-\n-\nq\n"},
		{"zoom out then spark", "-\n-\n-\nr\nr\nr\nq\n"},
		{"mixed zoom and render", "r\n-\nr\n+\nr\n-\nq\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, "-i", "testdata/gradient_32x32.png")
			cmd.Stdin = strings.NewReader(tc.input)
			cmd.Env = append(os.Environ(), "TERM=xterm-256color")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(string(out)))
			}
			assertNoSparkSizeMismatchOutput(t, string(out))
		})
	}
}

func TestInteractiveSmallGradientViaPTY(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/gradient_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	tests := []struct {
		name string
		keys string
	}{
		{"cycle to spark", "rrrrq"},
		{"zoom out", "---q"},
		{"spark then zoom out", "rrr---q"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCatiPTY(t, bin, []string{"-i", "testdata/gradient_32x32.png"}, tc.keys)
			if err != nil {
				t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
			}
			assertNoSparkSizeMismatchOutput(t, out)
		})
	}
}

func TestInteractiveSolidRedZoomOutDoesNotLeakRightEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/solid_red_4x4.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	out, err := runCatiPTY(t, bin, []string{"-i", "testdata/solid_red_4x4.png"}, "--q")
	if err != nil {
		t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
	}
	plain := stripANSIForTest(out)
	for _, leak := range []string{"██▙", "▀▀▘"} {
		if strings.Contains(plain, leak) {
			t.Fatalf("solid red zoom-out output contains right-edge leak %q\noutput:\n%s", leak, plain)
		}
	}
}

func TestInteractiveGradientHintShowsSourcePixelsPerCell(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/gradient_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"default fit", []string{"-i", "testdata/gradient_32x32.png"}, "src px/cell=1"},
		{"explicit one", []string{"-i", "testdata/gradient_32x32.png", "-z=1"}, "src px/cell=1"},
		{"explicit half", []string{"-i", "testdata/gradient_32x32.png", "-z=0.5"}, "src px/cell=0.5"},
		{"explicit four", []string{"-i", "testdata/gradient_32x32.png", "-z=4"}, "src px/cell=4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCatiPTY(t, bin, tc.args, "q")
			if err != nil {
				t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
			}
			if got := lastZoomHintForTest(out); got != tc.want {
				t.Fatalf("last zoom hint = %q, want %q\noutput:\n%s", got, tc.want, stripANSIForTest(out))
			}
		})
	}
}

func TestInteractiveModeHalfStartsHalfblockAndCyclesToQuad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/gradient_horiz_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	tests := []struct {
		name string
		keys string
		want string
	}{
		{"starts halfblock", "q", "halfblock"},
		{"r cycles to quad", "rq", "quad/splithalf"},
		{"third r reaches spark", "rrrq", "spark/quad"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCatiPTY(t, bin, []string{"-i", "testdata/gradient_horiz_32x32.png", "--mode=half"}, tc.keys)
			if err != nil {
				t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
			}
			if !strings.Contains(stripANSIForTest(out), tc.want) {
				t.Fatalf("output missing mode %q\noutput:\n%s", tc.want, stripANSIForTest(out))
			}
		})
	}
}

func TestInteractiveAndStaticHalfblockRenderMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("assets/samples/sample-003-darth-daughter.jpg"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	args := []string{"assets/samples/sample-003-darth-daughter.jpg", "-m=h", "-w", "50"}

	staticCmd := exec.Command(bin, args...)
	staticCmd.Env = append(os.Environ(), "TERM=xterm-256color")
	staticOut, err := staticCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("static cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(string(staticOut)))
	}

	interactiveOut, err := runCatiPTYSize(t, bin, append([]string{"-i"}, args...), "q", 120, 80)
	if err != nil {
		t.Fatalf("interactive cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(interactiveOut))
	}

	gotStatic := normalizedRenderOutputForTest(string(staticOut))
	gotInteractive := normalizedRenderOutputForTest(firstRenderedFrameForTest(interactiveOut))
	if gotInteractive != gotStatic {
		intLines := strings.Split(gotInteractive, "\n")
		statLines := strings.Split(gotStatic, "\n")
		max := len(intLines)
		if len(statLines) > max {
			max = len(statLines)
		}
		for i := 0; i < max; i++ {
			var in, st string
			if i < len(intLines) {
				in = intLines[i]
			}
			if i < len(statLines) {
				st = statLines[i]
			}
			if in != st {
				t.Fatalf("interactive and static render output differ at line %d\ninteractive: %q\nstatic: %q", i+1, in, st)
			}
		}
		t.Fatalf("interactive and static render output differ")
	}
}

func TestInteractiveGradientHeightFitZoomOutHintSequence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/gradient_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	out, err := runCatiPTYSize(t, bin, []string{"-i", "testdata/gradient_32x32.png", "-z=h"}, "-----q", 37, 95)
	if err != nil {
		t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
	}
	hints := uniqueZoomHintsForTest(out)
	if len(hints) < 6 {
		t.Fatalf("zoom hints = %v, want at least 6\noutput:\n%s", hints, stripANSIForTest(out))
	}
	if !strings.HasPrefix(hints[0], "src px/cell=0.") {
		t.Fatalf("initial zoom hint = %q, want sub-1 px/cell\nhints: %v\noutput:\n%s", hints[0], hints, stripANSIForTest(out))
	}
	want := []string{
		"src px/cell=1",
		"src px/cell=1.25",
		"src px/cell=1.5",
		"src px/cell=1.75",
		"src px/cell=2",
	}
	for i, wantHint := range want {
		got := hints[i+1]
		if got != wantHint {
			t.Fatalf("zoom hint after %d '-' = %q, want %q\nhints: %v\noutput:\n%s", i+1, got, wantHint, hints, stripANSIForTest(out))
		}
	}
}

func TestInteractiveGradientInfoShowsZoomDerivation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/gradient_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	out, err := runCatiPTYSize(t, bin, []string{"-i", "testdata/gradient_32x32.png", "-z=h"}, "--iq", 37, 95)
	if err != nil {
		t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
	}
	info := lastInfoLineForTest(out)
	for _, want := range []string{
		"info raw=1.23",
		"ladder=1.25",
		"trim=none",
		"cells=26x13",
		"src=32x32",
	} {
		if !strings.Contains(info, want) {
			t.Fatalf("info line missing %q\ninfo: %q\noutput:\n%s", want, info, stripANSIForTest(out))
		}
	}
}

func TestInteractiveGradientInfoTogglesOff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/gradient_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	out, err := runCatiPTYSize(t, bin, []string{"-i", "testdata/gradient_32x32.png", "-z=h"}, "iiq", 37, 95)
	if err != nil {
		t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
	}
	if got := lastInfoOrZoomHintForTest(out); !strings.HasPrefix(got, "src px/cell=") {
		t.Fatalf("last info/zoom hint = %q, want normal zoom hint\noutput:\n%s", got, stripANSIForTest(out))
	}
}

func TestInteractiveGradientConvergentModesShowPerfectSSIM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/gradient_32x32.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	tests := []struct {
		name string
		mode string
	}{
		{"default quad", "quad/splithalf"},
		{"edge snap", "quad/edge-snap"},
		{"spark quad", "spark/quad"},
		{"spark best", "spark/best"},
		{"sextant 2x3", "sextant/2x3"},
		{"halfblock", "halfblock"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCatiPTY(t, bin, []string{"-i", "testdata/gradient_32x32.png", "-z=1", "--mode=" + tc.mode}, "q")
			if err != nil {
				t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
			}
			ssim := lastSSIMForModeForTest(out, tc.mode)
			if ssim != "1.000" {
				t.Fatalf("last SSIM for %s = %q, want 1.000\noutput:\n%s", tc.mode, ssim, stripANSIForTest(out))
			}
		})
	}
}

func TestInteractiveSolidRedAllModesShowPerfectSSIM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY integration test in short mode")
	}
	if _, err := os.Stat("testdata/solid_red_4x4.png"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	bin := buildCatiForTest(t)
	tests := []struct {
		name string
		mode string
	}{
		{"quad splithalf", "quad/splithalf"},
		{"quad edge snap", "quad/edge-snap"},
		{"spark quad", "spark/quad"},
		{"spark best", "spark/best"},
		{"sextant 2x3", "sextant/2x3"},
		{"halfblock", "halfblock"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCatiPTY(t, bin, []string{"-i", "testdata/solid_red_4x4.png", "-z=1", "--mode=" + tc.mode}, "q")
			if err != nil {
				t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
			}
			ssim := lastSSIMForModeForTest(out, tc.mode)
			if ssim != "1.000" {
				t.Fatalf("last SSIM for %s = %q, want 1.000\noutput:\n%s", tc.mode, ssim, stripANSIForTest(out))
			}
		})
	}
}

func TestInteractiveVideoFramesDoNotWrapAcrossRenderModes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY video integration test in short mode")
	}
	if _, err := os.Stat("testdata/baby-60p-nn.webm"); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	const rows, cols = 37, 95
	bin := buildCatiForTest(t)
	out, err := runCatiPTYSize(t, bin, []string{"-i", "testdata/baby-60p-nn.webm"}, "rrrq", rows, cols)
	if err != nil {
		t.Fatalf("cati exited with error: %v\noutput:\n%s", err, stripANSIForTest(out))
	}
	plain := stripANSIForTest(out)
	for _, mode := range []string{"quad/splithalf", "quad/edge-snap", "halfblock", "spark/quad"} {
		if !strings.Contains(plain, mode) {
			t.Fatalf("video output missing mode %q\noutput:\n%s", mode, plain)
		}
	}
	blocks := renderedFrameBlocksForTest(out)
	if len(blocks) < 4 {
		t.Fatalf("captured %d rendered frame blocks, want at least 4\noutput:\n%s", len(blocks), plain)
	}
	want := blocks[0]
	for i, got := range blocks {
		if got.Cols <= 0 || got.Rows <= 0 {
			t.Fatalf("block %d has invalid size %+v", i, got)
		}
		if got.Cols > cols {
			t.Fatalf("block %d width = %d, want <= %d", i, got.Cols, cols)
		}
		if got != want {
			t.Fatalf("block %d size = %dx%d, want %dx%d\nall blocks: %+v", i, got.Cols, got.Rows, want.Cols, want.Rows, blocks)
		}
	}
}

func buildCatiForTest(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "cati")
	build := exec.Command("go", "build", "-o", bin, "./cmd/cati")
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func assertNoSparkSizeMismatchOutput(t *testing.T, out string) {
	t.Helper()

	plain := stripANSIForTest(out)
	for _, bad := range []string{
		"render size mismatch for spark/quad",
		"got 8x4 cells",
	} {
		if strings.Contains(plain, bad) {
			t.Fatalf("output contains %q\noutput:\n%s", bad, plain)
		}
	}
}

func lastZoomHintForTest(out string) string {
	matches := zoomHintsForTest(out)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func zoomHintsForTest(out string) []string {
	return regexp.MustCompile(`src px/cell=[0-9.]+`).FindAllString(stripANSIForTest(out), -1)
}

func uniqueZoomHintsForTest(out string) []string {
	var unique []string
	for _, hint := range zoomHintsForTest(out) {
		if len(unique) == 0 || unique[len(unique)-1] != hint {
			unique = append(unique, hint)
		}
	}
	return unique
}

func lastInfoLineForTest(out string) string {
	lines := strings.FieldsFunc(stripANSIForTest(out), func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	for i := len(lines) - 1; i >= 0; i-- {
		if idx := strings.Index(lines[i], "info raw="); idx >= 0 {
			return lines[i][idx:]
		}
	}
	return ""
}

func lastInfoOrZoomHintForTest(out string) string {
	re := regexp.MustCompile(`info raw=[^\r\n]*|src px/cell=[0-9.]+`)
	matches := re.FindAllString(stripANSIForTest(out), -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func lastSSIMForModeForTest(out, mode string) string {
	pattern := regexp.QuoteMeta(mode) + `.*?S:([0-9.]+)`
	matches := regexp.MustCompile(pattern).FindAllStringSubmatch(stripANSIForTest(out), -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

func firstRenderedFrameForTest(out string) string {
	start := strings.Index(out, "\x1b[H")
	if start < 0 {
		return out
	}
	end := strings.Index(out[start:], "\x1b[J")
	if end < 0 {
		return out[start:]
	}
	return out[start : start+end]
}

func normalizedRenderOutputForTest(out string) string {
	plain := stripANSIForTest(out)
	plain = strings.ReplaceAll(plain, "\r", "")
	plain = strings.TrimSuffix(plain, "\n")
	return plain
}

type frameBlockSize struct {
	Cols int
	Rows int
}

func renderedFrameBlocksForTest(out string) []frameBlockSize {
	var blocks []frameBlockSize
	for start := 0; ; {
		i := strings.Index(out[start:], "\x1b[H")
		if i < 0 {
			break
		}
		i += start + len("\x1b[H")
		j := strings.Index(out[i:], "\x1b[J")
		if j < 0 {
			break
		}
		chunk := stripANSIForTest(out[i : i+j])
		lines := strings.Split(chunk, "\n")
		var widths []int
		for _, line := range lines {
			line = strings.Trim(line, "\r")
			if line == "" {
				continue
			}
			widths = append(widths, utf8.RuneCountInString(line))
		}
		if len(widths) > 0 {
			w := widths[0]
			blocks = append(blocks, frameBlockSize{Cols: w, Rows: len(widths)})
		}
		start = i + j + len("\x1b[J")
	}
	return blocks
}

func runCatiPTY(t *testing.T, binary string, args []string, keys string) (string, error) {
	t.Helper()

	return runCatiPTYSize(t, binary, args, keys, 24, 80)
}

func runCatiPTYSize(t *testing.T, binary string, args []string, keys string, rows, cols uint16) (string, error) {
	t.Helper()

	master, slaveName := spawnPTYForTest(t)
	defer master.Close()

	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open slave PTY %s: %v", slaveName, err)
	}
	defer slave.Close()

	setPTYSizeForTest(t, slave, rows, cols)

	cmd := exec.Command(binary, args...)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start cati: %v", err)
	}
	_ = slave.Close()

	output := make(chan string, 100)
	go func() {
		defer close(output)
		buf := make([]byte, 65536)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				output <- string(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	go func() {
		time.Sleep(300 * time.Millisecond)
		for i := 0; i < len(keys); i++ {
			if _, err := master.Write([]byte{keys[i]}); err != nil {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	var out strings.Builder
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()

	for {
		select {
		case chunk, ok := <-output:
			if ok {
				out.WriteString(chunk)
			}
		case err := <-done:
			time.Sleep(50 * time.Millisecond)
			for {
				select {
				case chunk, ok := <-output:
					if !ok {
						return out.String(), err
					}
					out.WriteString(chunk)
				default:
					return out.String(), err
				}
			}
		case <-deadline.C:
			_ = cmd.Process.Kill()
			err := <-done
			t.Fatalf("cati did not exit after scripted keys; wait error: %v\noutput:\n%s", err, stripANSIForTest(out.String()))
		}
	}
}

func spawnPTYForTest(t *testing.T) (*os.File, string) {
	t.Helper()

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open /dev/ptmx: %v", err)
	}

	var ptsNum uint32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&ptsNum)))
	if errno != 0 {
		_ = master.Close()
		t.Fatalf("TIOCGPTN: %v", errno)
	}

	var unlock int
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock)))
	if errno != 0 {
		_ = master.Close()
		t.Fatalf("TIOCSPTLCK: %v", errno)
	}

	return master, fmt.Sprintf("/dev/pts/%d", ptsNum)
}

func setPTYSizeForTest(t *testing.T, slave *os.File, rows, cols uint16) {
	t.Helper()

	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	ws := winsize{Row: rows, Col: cols}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, slave.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		t.Fatalf("TIOCSWINSZ: %v", errno)
	}
}

func stripANSIForTest(s string) string {
	var out bytes.Buffer
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) {
					c := s[i]
					i++
					if c >= 0x40 && c <= 0x7e {
						break
					}
				}
				continue
			}
			if i < len(s) {
				i++
			}
			continue
		}
		if s[i] >= 0x20 || s[i] == '\n' || s[i] == '\r' {
			out.WriteByte(s[i])
		}
		i++
	}
	return out.String()
}
