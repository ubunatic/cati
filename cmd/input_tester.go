package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"codeberg.org/ubunatic/cati/v1/halfblock"
	"codeberg.org/ubunatic/cati/internal/input"
	spec "codeberg.org/ubunatic/cati/spec"
	"golang.org/x/term"
)

func runInputTest() error {
	// Load spec; fall back to defaults silently.
	inputSpec, _ := input.Load(fs.FS(spec.FS))

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		oldState = nil
	}
	defer func() {
		if oldState != nil {
			_ = term.Restore(fd, oldState)
		}
	}()

	logPath := fmt.Sprintf("/tmp/cati-input-test-%s.log", time.Now().Format("20060102-150405"))
	logFile, err := os.Create(logPath)
	if err != nil {
		logFile = nil
	}
	defer func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}()
	writeLog := func(line string) {
		if logFile != nil {
			_, _ = fmt.Fprintln(logFile, line)
		}
	}

	// Enable full mouse tracking.
	fmt.Fprint(os.Stdout, inputSpec.MouseEnableMotion())
	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	defer func() {
		fmt.Fprint(os.Stdout, inputSpec.MouseDisableMotion())
		halfblock.ShowCursor(os.Stdout)
		halfblock.EraseDown(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGWINCH)
	defer signal.Stop(sigs)

	const maxLines = 20
	type displayLine struct {
		text      string
		covered   bool
		eventType string
	}
	var lines []displayLine
	var unexpected []string

	addLine := func(ev input.Event, tok string) {
		covered := ev.Type != input.EventUnknown
		name := inputSpec.EventName(ev)
		var text string
		switch ev.Type {
		case input.EventMouse:
			m := ev.Mouse
			text = fmt.Sprintf("[mouse  ] %-14s btn=%d col=%d row=%d",
				"\""+name+"\"", m.Button, m.Col, m.Row)
		case input.EventFocus:
			text = fmt.Sprintf("[focus  ] %-14s", "\""+name+"\"")
		case input.EventDefocus:
			text = fmt.Sprintf("[defocus] %-14s", "\""+name+"\"")
		case input.EventResize:
			text = fmt.Sprintf("[resize ] %-14s", "\""+name+"\"")
		default:
			seq := hexEscape(tok)
			covStr := "YES"
			if !covered {
				covStr = "NO "
			}
			text = fmt.Sprintf("[key    ] %-14s seq=%-12s covered=%s",
				"\""+name+"\"", seq, covStr)
		}
		if !covered {
			text += "  \x1b[31m← unexpected\x1b[m"
			unexpected = append(unexpected, tok)
		}
		lines = append(lines, displayLine{text: text, covered: covered, eventType: string(ev.Type)})
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		writeLog(text)
	}

	redraw := func() {
		halfblock.CursorHome(os.Stdout)
		sep := strings.Repeat("─", 55)
		fmt.Fprintf(os.Stdout, "\x1b[2K\x1b[1mcati --input-test\x1b[m  (C-c to exit)\r\n")
		fmt.Fprintf(os.Stdout, "\x1b[2K%s\r\n", sep)
		for _, l := range lines {
			fmt.Fprintf(os.Stdout, "\x1b[2K%s\r\n", l.text)
		}
		// Fill remaining lines.
		for i := len(lines); i < maxLines; i++ {
			fmt.Fprintf(os.Stdout, "\x1b[2K\r\n")
		}
		fmt.Fprintf(os.Stdout, "\x1b[2K%s\r\n", sep)
		if logFile != nil {
			fmt.Fprintf(os.Stdout, "\x1b[2Klog: %s\r\n", logPath)
		}
	}

	rawInputs := make(chan string, 64)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			rawInputs <- string(buf[:n])
		}
	}()

	redraw()

	for {
		select {
		case <-sigs:
			ev := input.Event{Type: input.EventResize}
			addLine(ev, "")
			redraw()

		case raw := <-rawInputs:
			tokens := inputSpec.Tokenize(raw)
			quit := false
			for _, tok := range tokens {
				// Ctrl-C is the only hardcoded exit safeguard.
				if tok == "\x03" {
					quit = true
					break
				}
				ev := inputSpec.Classify(tok)
				addLine(ev, tok)
			}
			redraw()
			if quit {
				goto done
			}
		}
	}

done:
	// Print unexpected summary to stdout after restoring terminal.
	if oldState != nil {
		_ = term.Restore(fd, oldState)
		oldState = nil
	}
	fmt.Fprint(os.Stdout, inputSpec.MouseDisableMotion())
	halfblock.ShowCursor(os.Stdout)
	halfblock.EraseDown(os.Stdout)
	fmt.Fprint(os.Stdout, "\r\n")

	if len(unexpected) > 0 {
		fmt.Fprintf(os.Stdout, "\nUnexpected tokens (%d):\n", len(unexpected))
		for _, u := range unexpected {
			fmt.Fprintf(os.Stdout, "  %s\n", hexEscape(u))
		}
	} else {
		fmt.Fprintln(os.Stdout, "\nAll captured tokens are covered by spec/input.yaml.")
	}
	if logFile != nil {
		fmt.Fprintf(os.Stdout, "Log: %s\n", logPath)
	}
	return nil
}

// hexEscape returns a printable representation of s.
// Valid UTF-8 printable codepoints (including non-ASCII like ö, €) are kept as-is;
// everything else is shown as \xNN bytes.
func hexEscape(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c >= 0x20 && c < 0x7f {
			b.WriteByte(c)
			i++
			continue
		}
		if c >= 0x80 {
			if r, size := utf8.DecodeRuneInString(s[i:]); r != utf8.RuneError && size > 1 {
				b.WriteRune(r)
				i += size
				continue
			}
		}
		fmt.Fprintf(&b, "\\x%02x", c)
		i++
	}
	return b.String()
}
