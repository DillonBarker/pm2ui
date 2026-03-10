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

type FilterMode int

const (
	FilterTail  FilterMode = iota
	FilterHead
	FilterLastN
)

type LogFilter struct {
	Mode  FilterMode
	Lines int
}

// LogLine represents a single line from a log file.
type LogLine struct {
	Text        string
	Stream      LogStream
	ProcessName string
}

// MultiLogTailer merges log output from multiple processes into one channel.
type MultiLogTailer struct {
	procs   []Process
	tailers []*LogTailer
	lines   chan LogLine
	stopCh  chan struct{}
	mu      sync.Mutex
	stopped bool
}

// NewMultiLogTailer creates a tailer that merges logs from all given processes.
func NewMultiLogTailer(procs []Process, filter LogFilter) *MultiLogTailer {
	perSvcFilter := filter
	if filter.Mode == FilterLastN && len(procs) > 1 {
		n := filter.Lines / len(procs)
		if n < 1 {
			n = 1
		}
		perSvcFilter.Lines = n
	}
	mt := &MultiLogTailer{
		procs:  procs,
		lines:  make(chan LogLine, 1000),
		stopCh: make(chan struct{}),
	}
	for _, p := range procs {
		mt.tailers = append(mt.tailers, NewLogTailer(p.PM2Env.PMOutLogPath, p.PM2Env.PMErrLogPath, perSvcFilter))
	}
	return mt
}

// Lines returns the merged channel of log lines.
func (mt *MultiLogTailer) Lines() <-chan LogLine {
	return mt.lines
}

// Start begins tailing all processes and merging their output.
func (mt *MultiLogTailer) Start() {
	for i, t := range mt.tailers {
		t := t
		name := mt.procs[i].Name
		t.Start()
		go func() {
			for {
				select {
				case line := <-t.Lines():
					line.ProcessName = name
					select {
					case mt.lines <- line:
					case <-mt.stopCh:
						return
					}
				case <-mt.stopCh:
					return
				}
			}
		}()
	}
}

// Stop stops all sub-tailers.
func (mt *MultiLogTailer) Stop() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if !mt.stopped {
		mt.stopped = true
		close(mt.stopCh)
		for _, t := range mt.tailers {
			t.Stop()
		}
	}
}

// LogTailer tails PM2 log files for a process.
type LogTailer struct {
	outPath string
	errPath string
	filter  LogFilter
	stream  LogStream
	lines   chan LogLine
	stopCh  chan struct{}
	mu      sync.Mutex
	stopped bool
}

// NewLogTailer creates a tailer for the given log file paths.
func NewLogTailer(outPath, errPath string, filter LogFilter) *LogTailer {
	return &LogTailer{
		outPath: outPath,
		errPath: errPath,
		filter:  filter,
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

	switch lt.filter.Mode {
	case FilterLastN:
		for _, line := range readLastLines(f, lt.filter.Lines) {
			lt.sendLine(line, stream)
		}
	case FilterHead:
		readFromStart(f)
	default:
		readLastLines(f, 0)
	}

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

func readFromStart(f *os.File) {
	_, _ = f.Seek(0, io.SeekStart)
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
	if n == 0 {
		_, _ = f.Seek(0, io.SeekEnd)
		return nil
	}

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
	var b strings.Builder
	last := 0
	for _, loc := range ansiRegex.FindAllStringIndex(s, -1) {
		segment := s[last:loc[0]]
		b.WriteString(strings.ReplaceAll(segment, "[", "[[]"))
		match := s[loc[0]:loc[1]]
		codes := ansiRegex.FindStringSubmatch(match)
		if len(codes) >= 2 {
			b.WriteString(ansiCodeToTag(codes[1]))
		}
		last = loc[1]
	}
	b.WriteString(strings.ReplaceAll(s[last:], "[", "[[]"))
	return b.String()
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
