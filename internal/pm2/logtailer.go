package pm2

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// LogStream indicates which log stream to tail.
type LogStream int

const (
	LogStdout LogStream = iota
	LogStderr
	LogBoth
)

// LogLine represents a single line from a log file.
type LogLine struct {
	Text   string
	Stream LogStream
}

// LogTailer tails PM2 log files for a process.
type LogTailer struct {
	outPath  string
	errPath  string
	stream   LogStream
	lines    chan LogLine
	stopCh   chan struct{}
	mu       sync.Mutex
	stopped  bool
}

// NewLogTailer creates a tailer for the given log file paths.
func NewLogTailer(outPath, errPath string) *LogTailer {
	return &LogTailer{
		outPath: outPath,
		errPath: errPath,
		stream:  LogBoth,
		lines:   make(chan LogLine, 1000),
		stopCh:  make(chan struct{}),
	}
}

// Lines returns the channel to read log lines from.
func (lt *LogTailer) Lines() <-chan LogLine {
	return lt.lines
}

// SetStream changes which streams to tail.
func (lt *LogTailer) SetStream(s LogStream) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.stream = s
}

// Stream returns the current stream mode.
func (lt *LogTailer) Stream() LogStream {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.stream
}

// Start begins tailing logs.
func (lt *LogTailer) Start() {
	if lt.outPath != "" {
		go lt.tailFile(lt.outPath, LogStdout)
	}
	if lt.errPath != "" {
		go lt.tailFile(lt.errPath, LogStderr)
	}
}

// Stop stops the tailer.
func (lt *LogTailer) Stop() {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if !lt.stopped {
		lt.stopped = true
		close(lt.stopCh)
	}
}

func (lt *LogTailer) tailFile(path string, stream LogStream) {
	f, err := os.Open(path)
	if err != nil {
		lt.sendLine("(cannot open log: "+err.Error()+")", stream)
		return
	}
	defer f.Close()

	// Seek to get last 100 lines
	lines := readLastLines(f, 100)
	for _, line := range lines {
		if !lt.shouldSend(stream) {
			continue
		}
		lt.sendLine(line, stream)
	}

	// Now tail for new data
	reader := bufio.NewReader(f)
	for {
		select {
		case <-lt.stopCh:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return
		}

		line = strings.TrimRight(line, "\n\r")
		if lt.shouldSend(stream) {
			lt.sendLine(line, stream)
		}
	}
}

func (lt *LogTailer) shouldSend(stream LogStream) bool {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.stream == LogBoth || lt.stream == stream
}

func (lt *LogTailer) sendLine(text string, stream LogStream) {
	converted := ansiToTview(text)
	select {
	case lt.lines <- LogLine{Text: converted, Stream: stream}:
	case <-lt.stopCh:
	}
}

// readLastLines reads the last n lines from a file.
func readLastLines(f *os.File, n int) []string {
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}

	// Read from the end
	size := info.Size()
	bufSize := int64(64 * 1024)
	if bufSize > size {
		bufSize = size
	}

	offset := size - bufSize
	if offset < 0 {
		offset = 0
	}

	_, _ = f.Seek(offset, io.SeekStart)
	buf := make([]byte, bufSize)
	nRead, _ := io.ReadFull(f, buf)
	buf = buf[:nRead]

	// Split into lines
	lines := strings.Split(string(buf), "\n")
	// If we started mid-line and not at beginning of file, drop first partial line
	if offset > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	// Remove trailing empty
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	// Seek to end for future reads
	_, _ = f.Seek(0, io.SeekEnd)

	return lines
}

// ansiToTview converts ANSI escape codes to tview color tags.
var ansiRegex = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func ansiToTview(s string) string {
	// Escape tview's tag brackets first
	s = strings.ReplaceAll(s, "[", "[[]")

	result := ansiRegex.ReplaceAllStringFunc(s, func(match string) string {
		codes := ansiRegex.FindStringSubmatch(match)
		if len(codes) < 2 {
			return ""
		}
		return ansiCodeToTag(codes[1])
	})

	return result
}

func ansiCodeToTag(code string) string {
	parts := strings.Split(code, ";")
	for _, p := range parts {
		switch p {
		case "0", "":
			return "[-:-:-]"
		case "1":
			return "[::b]"
		case "2":
			return "[::d]"
		case "3":
			return "[::i]"
		case "4":
			return "[::u]"
		case "30":
			return "[black]"
		case "31":
			return "[red]"
		case "32":
			return "[green]"
		case "33":
			return "[yellow]"
		case "34":
			return "[blue]"
		case "35":
			return "[purple]"
		case "36":
			return "[cyan]"
		case "37":
			return "[white]"
		case "90":
			return "[gray]"
		case "91":
			return "[red]"
		case "92":
			return "[green]"
		case "93":
			return "[yellow]"
		case "94":
			return "[blue]"
		case "95":
			return "[purple]"
		case "96":
			return "[cyan]"
		case "97":
			return "[white]"
		}
	}
	return ""
}
