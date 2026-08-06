package output

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestSanitizeTerminal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "hello world", "hello world"},
		{"empty", "", ""},
		{"escape", "a\x1bb", "a\ufffdb"},
		{"osc52 clipboard write", "\x1b]52;c;aGVsbG8=\x07", "\ufffd]52;c;aGVsbG8=\ufffd"},
		{"csi cursor move", "safe\x1b[2Kforged", "safe\ufffd[2Kforged"},
		{"carriage return overwrite", "real\rfake", "real\ufffdfake"},
		{"newline", "one\ntwo", "one\ufffdtwo"},
		{"tab", "a\tb", "a\ufffdb"},
		{"null", "a\x00b", "a\ufffdb"},
		{"delete", "a\x7fb", "a\ufffdb"},
		{"c1 csi", "a\u009bb", "a\ufffdb"},
		{"c1 range boundaries", "\u0080\u009f", "\ufffd\ufffd"},
		{"space preserved", "a b", "a b"},
		{"latin1 letter preserved", "café", "café"},
		{"cjk preserved", "中文", "中文"},
		{"emoji preserved", "ok \U0001F600", "ok \U0001F600"},
		{"box drawing preserved", "─├└", "─├└"},
		{"existing replacement char preserved", "a\ufffdb", "a\ufffdb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeTerminal(tt.in); got != tt.want {
				t.Errorf("SanitizeTerminal(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeTerminalIsIdempotent(t *testing.T) {
	in := "\x1b]52;c;x\x07line\nend"
	once := SanitizeTerminal(in)
	if twice := SanitizeTerminal(once); twice != once {
		t.Errorf("second pass changed output: %q then %q", once, twice)
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"shorter than max", "abc", 10, "abc"},
		{"exactly max", "abc", 3, "abc"},
		{"ascii truncated", "abcdef", 3, "abc"},
		{"multibyte not split", "héllo", 3, "hél"},
		{"emoji not split", "ab\U0001F600cd", 3, "ab\U0001F600"},
		{"zero max", "abc", 0, ""},
		{"negative max", "abc", -1, ""},
		{"empty input", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateRunes(tt.in, tt.max); got != tt.want {
				t.Errorf("TruncateRunes(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func TestPrintfSanitizesStringArgs(t *testing.T) {
	got := captureStdout(t, func() {
		Printf("  [%s] %s\n", "user", "hi\x1b]52;c;aGk=\x07there")
	})
	want := "  [user] hi\ufffd]52;c;aGk=\ufffdthere\n"
	if got != want {
		t.Errorf("Printf = %q, want %q", got, want)
	}
}

func TestPrintfKeepsFormatStringLiterals(t *testing.T) {
	got := captureStdout(t, func() {
		Printf("a\tb\nc\n")
	})
	if got != "a\tb\nc\n" {
		t.Errorf("Printf mangled its format string: %q", got)
	}
}

func TestPrintfKeepsNonStringVerbs(t *testing.T) {
	got := captureStdout(t, func() {
		Printf("%d items in %.1fs (%v)\n", 3, 1.5, true)
	})
	if got != "3 items in 1.5s (true)\n" {
		t.Errorf("Printf = %q", got)
	}
}

func TestPrintfSanitizesErrorArgs(t *testing.T) {
	got := captureStdout(t, func() {
		Printf("failed: %v\n", errors.New("boom\x1b[2K"))
	})
	if got != "failed: boom\ufffd[2K\n" {
		t.Errorf("Printf = %q", got)
	}
}

func TestPrintfHandlesNilError(t *testing.T) {
	var err error
	got := captureStdout(t, func() {
		Printf("err=%v\n", err)
	})
	if got != "err=<nil>\n" {
		t.Errorf("Printf = %q", got)
	}
}

func TestPrintfSanitizesStringerArgs(t *testing.T) {
	got := captureStdout(t, func() {
		Printf("took %v\n", time.Duration(0))
	})
	if got != "took 0s\n" {
		t.Errorf("Printf = %q", got)
	}
}

func TestPrintlnSanitizesArgs(t *testing.T) {
	got := captureStdout(t, func() {
		Println("safe", "bad\rline")
	})
	if got != "safe bad\ufffdline\n" {
		t.Errorf("Println = %q", got)
	}
}

func TestPrintlnNoArgs(t *testing.T) {
	got := captureStdout(t, func() {
		Println()
	})
	if got != "\n" {
		t.Errorf("Println() = %q", got)
	}
}

func TestErrorfWritesSanitizedToStderr(t *testing.T) {
	got := captureStderr(t, func() {
		Errorf("api error: %v\n", errors.New("nope\x1b"))
	})
	if got != "api error: nope\ufffd\n" {
		t.Errorf("Errorf = %q", got)
	}
}

func TestErrorlnWritesSanitizedToStderr(t *testing.T) {
	got := captureStderr(t, func() {
		Errorln("bad\x1bvalue")
	})
	if got != "bad\ufffdvalue\n" {
		t.Errorf("Errorln = %q", got)
	}
}

func TestErrorlnSanitizesPreformattedMessage(t *testing.T) {
	got := captureStderr(t, func() {
		Errorln("server said: reset\x1b[1;1H")
	})
	if got != "server said: reset\ufffd[1;1H\n" {
		t.Errorf("Errorln = %q", got)
	}
}

func TestPrintLinesKeepsLineBreaksAndSanitizesTheRest(t *testing.T) {
	got := captureStdout(t, func() {
		PrintLines("{\n  \"name\": \"a\x1b]52;c;x\x07b\"\n}\n")
	})
	want := "{\n  \"name\": \"a\ufffd]52;c;x\ufffdb\"\n}\n"
	if got != want {
		t.Errorf("PrintLines = %q, want %q", got, want)
	}
}

func TestPrintLinesReplacesCarriageReturn(t *testing.T) {
	got := captureStdout(t, func() {
		PrintLines("real\rfake\n")
	})
	if got != "real\ufffdfake\n" {
		t.Errorf("PrintLines = %q", got)
	}
}

func TestPrintLinesAddsNoTrailingNewline(t *testing.T) {
	got := captureStdout(t, func() {
		PrintLines("no newline here")
	})
	if got != "no newline here" {
		t.Errorf("PrintLines = %q", got)
	}
}

func TestFprintLinesWritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	FprintLines(&buf, "line one\nline\x1btwo\n")
	if buf.String() != "line one\nline\ufffdtwo\n" {
		t.Errorf("FprintLines = %q", buf.String())
	}
}

func TestPrintProxiedPassesBytesThrough(t *testing.T) {
	proxied := "\x1b[32mok\x1b[0m\n"
	got := captureStdout(t, func() {
		PrintProxied(proxied)
	})
	if got != proxied {
		t.Errorf("PrintProxied altered its input: %q", got)
	}
}

func TestFprintProxiedPassesBytesThrough(t *testing.T) {
	var buf bytes.Buffer
	proxied := "progress\rdone\x1b[K"
	FprintProxied(&buf, proxied)
	if buf.String() != proxied {
		t.Errorf("FprintProxied altered its input: %q", buf.String())
	}
}
