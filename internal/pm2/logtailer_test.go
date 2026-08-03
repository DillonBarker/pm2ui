package pm2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

// collect reads up to n lines from ch, waiting at most timeout for each.
func collect(t *testing.T, ch <-chan LogLine, n int, timeout time.Duration) []LogLine {
	t.Helper()
	var out []LogLine
	for len(out) < n {
		select {
		case line := <-ch:
			out = append(out, line)
		case <-time.After(timeout):
			return out
		}
	}
	return out
}

func plainTexts(lines []LogLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Plain
	}
	return out
}

func openForTest(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestReadLastLines_Exact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n")
	f := openForTest(t, path)

	lines, offset := readLastLines(f, 5)
	want := []string{"l6", "l7", "l8", "l9", "l10"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Errorf("lines = %v, want %v", lines, want)
	}
	info, _ := f.Stat()
	if offset != info.Size() {
		t.Errorf("offset = %d, want %d", offset, info.Size())
	}
}

func TestReadLastLines_FewerThanN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "a\nb\nc\n")
	f := openForTest(t, path)

	lines, _ := readLastLines(f, 10)
	want := []string{"a", "b", "c"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Errorf("lines = %v, want %v", lines, want)
	}
}

func TestReadLastLines_NoTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "a\nb\nc")
	f := openForTest(t, path)

	lines, offset := readLastLines(f, 10)
	// "c" is still being written; it must be left for the live tail.
	want := []string{"a", "b"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Errorf("lines = %v, want %v", lines, want)
	}
	if offset != 4 { // after "a\nb\n"
		t.Errorf("offset = %d, want 4", offset)
	}
}

func TestReadLastLines_LargeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	var b strings.Builder
	for i := range 3000 {
		fmt.Fprintf(&b, "line-%04d padding padding padding padding padding\n", i)
	}
	writeFile(t, path, b.String())
	f := openForTest(t, path)

	lines, _ := readLastLines(f, 100)
	if len(lines) != 100 {
		t.Fatalf("got %d lines, want 100", len(lines))
	}
	if !strings.HasPrefix(lines[0], "line-2900") {
		t.Errorf("first = %q, want prefix line-2900", lines[0])
	}
	if !strings.HasPrefix(lines[99], "line-2999") {
		t.Errorf("last = %q, want prefix line-2999", lines[99])
	}
}

func TestTail_Truncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "old1\nold2\n")

	lt := NewLogTailer(path, "", LogFilter{Mode: FilterTail})
	lt.Start()
	defer lt.Stop()

	time.Sleep(300 * time.Millisecond) // let the tailer seek to EOF first
	appendFile(t, path, "new1\n")
	got := collect(t, lt.Lines(), 1, 3*time.Second)
	if len(got) != 1 || got[0].Plain != "new1" {
		t.Fatalf("before truncate: got %v", plainTexts(got))
	}

	// Simulate `pm2 flush`.
	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "after\n")

	got = collect(t, lt.Lines(), 1, 3*time.Second)
	if len(got) != 1 || got[0].Plain != "after" {
		t.Fatalf("after truncate: got %v", plainTexts(got))
	}
}

func TestTail_Rotate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "old\n")

	lt := NewLogTailer(path, "", LogFilter{Mode: FilterTail})
	lt.Start()
	defer lt.Stop()

	time.Sleep(300 * time.Millisecond) // let the tailer reach EOF
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "fresh\n")

	got := collect(t, lt.Lines(), 1, 5*time.Second)
	if len(got) != 1 || got[0].Plain != "fresh" {
		t.Fatalf("after rotate: got %v", plainTexts(got))
	}
}

func TestTail_WaitsForFileToExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")

	lt := NewLogTailer(path, "", LogFilter{Mode: FilterTail})
	lt.Start()
	defer lt.Stop()

	time.Sleep(300 * time.Millisecond)
	writeFile(t, path, "born\n")

	got := collect(t, lt.Lines(), 1, 5*time.Second)
	if len(got) != 1 || got[0].Plain != "born" {
		t.Fatalf("got %v", plainTexts(got))
	}
}

func TestTail_PartialLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "")

	lt := NewLogTailer(path, "", LogFilter{Mode: FilterTail})
	lt.Start()
	defer lt.Stop()

	time.Sleep(300 * time.Millisecond) // let the tailer seek to EOF first
	appendFile(t, path, "par")
	time.Sleep(300 * time.Millisecond) // tailer sees the fragment, must hold it
	appendFile(t, path, "tial\n")

	got := collect(t, lt.Lines(), 1, 3*time.Second)
	if len(got) != 1 || got[0].Plain != "partial" {
		t.Fatalf("got %v", plainTexts(got))
	}
}

func TestHead_StopsAfterN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "h1\nh2\nh3\nh4\nh5\n")

	lt := NewLogTailer(path, "", LogFilter{Mode: FilterHead, Lines: 3})
	lt.Start()
	defer lt.Stop()

	got := collect(t, lt.Lines(), 3, 3*time.Second)
	want := []string{"h1", "h2", "h3"}
	if strings.Join(plainTexts(got), ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", plainTexts(got), want)
	}

	// Head mode must not keep tailing.
	appendFile(t, path, "h6\n")
	extra := collect(t, lt.Lines(), 1, 500*time.Millisecond)
	if len(extra) != 0 {
		t.Fatalf("head mode tailed extra lines: %v", plainTexts(extra))
	}
}

func TestLastN_ThenTails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "a\nb\nc\n")

	lt := NewLogTailer(path, "", LogFilter{Mode: FilterLastN, Lines: 2})
	lt.Start()
	defer lt.Stop()

	got := collect(t, lt.Lines(), 2, 3*time.Second)
	want := []string{"b", "c"}
	if strings.Join(plainTexts(got), ",") != strings.Join(want, ",") {
		t.Fatalf("initial dump: got %v, want %v", plainTexts(got), want)
	}

	appendFile(t, path, "d\n")
	got = collect(t, lt.Lines(), 1, 3*time.Second)
	if len(got) != 1 || got[0].Plain != "d" {
		t.Fatalf("live tail after dump: got %v", plainTexts(got))
	}
}

func TestANSIConversion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	writeFile(t, path, "")

	lt := NewLogTailer(path, "", LogFilter{Mode: FilterTail})
	lt.Start()
	defer lt.Stop()

	time.Sleep(300 * time.Millisecond) // let the tailer seek to EOF first
	appendFile(t, path, "\x1b[1;31mred bold\x1b[0m plain\n")
	got := collect(t, lt.Lines(), 1, 3*time.Second)
	if len(got) != 1 {
		t.Fatalf("got %d lines", len(got))
	}
	if got[0].Plain != "red bold plain" {
		t.Errorf("Plain = %q, want %q", got[0].Plain, "red bold plain")
	}
	if strings.Contains(got[0].Text, "\x1b") {
		t.Errorf("Text still contains raw escape: %q", got[0].Text)
	}
	if !strings.Contains(got[0].Text, "red bold") {
		t.Errorf("Text lost content: %q", got[0].Text)
	}
	if !strings.Contains(got[0].Text, "[") {
		t.Errorf("Text has no tview tags: %q", got[0].Text)
	}
	if got[0].Time.IsZero() {
		t.Error("Time not stamped")
	}
}

func TestMultiTailer_FullBudgetPerProcess(t *testing.T) {
	dir := t.TempDir()
	var procs []Process
	for i := range 2 {
		out := filepath.Join(dir, fmt.Sprintf("p%d.log", i))
		writeFile(t, out, "x1\nx2\nx3\n")
		p := Process{Name: fmt.Sprintf("p%d", i)}
		p.PM2Env.PMOutLogPath = out
		procs = append(procs, p)
	}

	mt := NewMultiLogTailer(procs, LogFilter{Mode: FilterLastN, Lines: 3})
	mt.Start()
	defer mt.Stop()

	// Each process must contribute its full 3 lines, not 3/len(procs).
	got := collect(t, mt.Lines(), 6, 3*time.Second)
	if len(got) != 6 {
		t.Fatalf("got %d lines, want 6", len(got))
	}
}

func TestMultiTailer_AddRemove(t *testing.T) {
	dir := t.TempDir()
	mkProc := func(name string) Process {
		out := filepath.Join(dir, name+".log")
		writeFile(t, out, "")
		p := Process{Name: name}
		p.PM2Env.PMOutLogPath = out
		return p
	}
	p1 := mkProc("first")

	mt := NewMultiLogTailer([]Process{p1}, LogFilter{Mode: FilterTail})
	mt.Start()
	defer mt.Stop()
	time.Sleep(300 * time.Millisecond)

	// A process added while running starts streaming.
	p2 := mkProc("second")
	mt.Add(p2)
	time.Sleep(300 * time.Millisecond)
	appendFile(t, p2.PM2Env.PMOutLogPath, "from-second\n")
	got := collect(t, mt.Lines(), 1, 3*time.Second)
	if len(got) != 1 || got[0].ProcessName != "second" {
		t.Fatalf("added process not streaming: %v", plainTexts(got))
	}

	// A removed process stops streaming.
	mt.Remove("first")
	time.Sleep(200 * time.Millisecond)
	appendFile(t, p1.PM2Env.PMOutLogPath, "from-first\n")
	extra := collect(t, mt.Lines(), 1, 500*time.Millisecond)
	if len(extra) != 0 {
		t.Fatalf("removed process still streaming: %v", plainTexts(extra))
	}
}

func TestMultiTailer_SetStream(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.log")
	errPath := filepath.Join(dir, "err.log")
	writeFile(t, out, "")
	writeFile(t, errPath, "")
	p := Process{Name: "p"}
	p.PM2Env.PMOutLogPath = out
	p.PM2Env.PMErrLogPath = errPath

	mt := NewMultiLogTailer([]Process{p}, LogFilter{Mode: FilterTail})
	mt.Start()
	defer mt.Stop()

	if mt.Stream() != LogBoth {
		t.Fatalf("default stream = %v, want LogBoth", mt.Stream())
	}
	mt.SetStream(LogStderr)
	time.Sleep(200 * time.Millisecond) // let the change land before writing

	appendFile(t, out, "to-stdout\n")
	appendFile(t, errPath, "to-stderr\n")

	got := collect(t, mt.Lines(), 1, 3*time.Second)
	if len(got) != 1 || got[0].Plain != "to-stderr" {
		t.Fatalf("got %v, want only to-stderr", plainTexts(got))
	}
	extra := collect(t, mt.Lines(), 1, 300*time.Millisecond)
	if len(extra) != 0 {
		t.Fatalf("stdout leaked through: %v", plainTexts(extra))
	}
}
