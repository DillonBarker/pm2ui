package pm2

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"
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
	FilterTail FilterMode = iota
	FilterHead
	FilterLastN
)

const defaultHeadLines = 200

type LogFilter struct {
	Mode  FilterMode
	Lines int
}

// LogLine represents a single line from a log file.
type LogLine struct {
	Text        string // tview-tagged (ANSI converted)
	Plain       string // ANSI stripped, no tview tags
	Stream      LogStream
	ProcessName string
	Time        time.Time
}

// MultiLogTailer merges log output from multiple processes into one channel.
// Processes can be added and removed while running.
type MultiLogTailer struct {
	filter  LogFilter
	lines   chan LogLine
	stopCh  chan struct{}
	mu      sync.Mutex
	tailers map[string]*LogTailer
	stream  LogStream
	started bool
	stopped bool
}

// NewMultiLogTailer creates a tailer that merges logs from all given processes.
// Each process gets the full filter line budget.
func NewMultiLogTailer(procs []Process, filter LogFilter) *MultiLogTailer {
	mt := &MultiLogTailer{
		filter:  filter,
		stream:  LogBoth,
		lines:   make(chan LogLine, 1000),
		stopCh:  make(chan struct{}),
		tailers: make(map[string]*LogTailer, len(procs)),
	}
	for _, p := range procs {
		mt.tailers[p.Name] = NewLogTailer(p.PM2Env.PMOutLogPath, p.PM2Env.PMErrLogPath, filter)
	}
	return mt
}

// Lines returns the merged channel of log lines.
func (mt *MultiLogTailer) Lines() <-chan LogLine {
	return mt.lines
}

// SetStream changes which streams to tail across all processes.
func (mt *MultiLogTailer) SetStream(s LogStream) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.stream = s
	for _, t := range mt.tailers {
		t.SetStream(s)
	}
}

// Stream returns the current stream mode.
func (mt *MultiLogTailer) Stream() LogStream {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return mt.stream
}

// Start begins tailing all processes and merging their output.
func (mt *MultiLogTailer) Start() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if mt.started || mt.stopped {
		return
	}
	mt.started = true
	for name, t := range mt.tailers {
		mt.run(name, t)
	}
}

// run starts a tailer and forwards its lines. Caller must hold mu.
func (mt *MultiLogTailer) run(name string, t *LogTailer) {
	t.SetStream(mt.stream)
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
			case <-t.Done():
				return
			case <-mt.stopCh:
				return
			}
		}
	}()
}

// Add begins tailing a new process without disturbing existing tailers.
func (mt *MultiLogTailer) Add(p Process) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if mt.stopped {
		return
	}
	if _, ok := mt.tailers[p.Name]; ok {
		return
	}
	t := NewLogTailer(p.PM2Env.PMOutLogPath, p.PM2Env.PMErrLogPath, mt.filter)
	mt.tailers[p.Name] = t
	if mt.started {
		mt.run(p.Name, t)
	}
}

// Remove stops tailing a process. Already-buffered lines are unaffected.
func (mt *MultiLogTailer) Remove(name string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if t, ok := mt.tailers[name]; ok {
		t.Stop()
		delete(mt.tailers, name)
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

// Done is closed when the tailer is stopped.
func (lt *LogTailer) Done() <-chan struct{} {
	return lt.stopCh
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

// tailFile follows a log file with tail -F semantics: it waits for the file
// to exist, survives truncation (pm2 flush) and rotation (rename+recreate),
// and holds partial writes until the newline arrives.
func (lt *LogTailer) tailFile(path string, stream LogStream) {
	// If the file doesn't exist yet, everything in it once it appears is new
	// content — read it from the start instead of seeking to the end.
	f, err := os.Open(path)
	fresh := false
	if err != nil {
		if f = lt.openWhenReady(path); f == nil {
			return
		}
		fresh = true
	}
	defer func() { f.Close() }()

	var offset int64

	switch {
	case lt.filter.Mode == FilterHead:
		n := lt.filter.Lines
		if n <= 0 {
			n = defaultHeadLines
		}
		lt.sendHead(f, n, stream)
		return
	case fresh:
		// offset stays 0: stream the newborn file from the top.
	case lt.filter.Mode == FilterLastN:
		lines, off := readLastLines(f, lt.filter.Lines)
		for _, line := range lines {
			if lt.shouldSend(stream) {
				lt.sendLine(line, stream)
			}
		}
		offset = off
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return
		}
	default: // FilterTail
		off, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return
		}
		offset = off
	}

	reader := bufio.NewReader(f)
	var pending strings.Builder

	for {
		select {
		case <-lt.stopCh:
			return
		default:
		}

		chunk, err := reader.ReadString('\n')
		offset += int64(len(chunk))
		if err == nil {
			line := strings.TrimRight(pending.String()+chunk, "\r\n")
			pending.Reset()
			if lt.shouldSend(stream) {
				lt.sendLine(line, stream)
			}
			continue
		}
		if err != io.EOF {
			return
		}

		// EOF: hold any partial fragment until its newline arrives.
		pending.WriteString(chunk)

		if !lt.sleepOrStop(100 * time.Millisecond) {
			return
		}

		pathInfo, statErr := os.Stat(path)
		if statErr != nil {
			// File deleted; wait for it to come back.
			f.Close()
			if f = lt.openWhenReady(path); f == nil {
				return
			}
			reader = bufio.NewReader(f)
			offset = 0
			pending.Reset()
			continue
		}

		fInfo, err := f.Stat()
		if err != nil || !os.SameFile(fInfo, pathInfo) {
			// Rotated: reopen the new file from the start.
			f.Close()
			if f = lt.openWhenReady(path); f == nil {
				return
			}
			reader = bufio.NewReader(f)
			offset = 0
			pending.Reset()
			continue
		}

		if pathInfo.Size() < offset {
			// Truncated in place (pm2 flush): start over from the top.
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return
			}
			reader.Reset(f)
			offset = 0
			pending.Reset()
		}
	}
}

// openWhenReady opens path, polling until it exists or the tailer is stopped.
func (lt *LogTailer) openWhenReady(path string) *os.File {
	for {
		f, err := os.Open(path)
		if err == nil {
			return f
		}
		if !lt.sleepOrStop(500 * time.Millisecond) {
			return nil
		}
	}
}

// sleepOrStop waits d, returning false if the tailer was stopped meanwhile.
func (lt *LogTailer) sleepOrStop(d time.Duration) bool {
	select {
	case <-lt.stopCh:
		return false
	case <-time.After(d):
		return true
	}
}

// sendHead sends the first n lines of the file and returns.
func (lt *LogTailer) sendHead(f *os.File, n int, stream LogStream) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for i := 0; i < n && scanner.Scan(); i++ {
		if lt.shouldSend(stream) {
			lt.sendLine(strings.TrimRight(scanner.Text(), "\r"), stream)
		}
	}
}

func (lt *LogTailer) shouldSend(stream LogStream) bool {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.stream == LogBoth || lt.stream == stream
}

func (lt *LogTailer) sendLine(text string, stream LogStream) {
	line := LogLine{
		Text:   tview.TranslateANSI(text),
		Plain:  stripANSI(text),
		Stream: stream,
		Time:   time.Now(),
	}
	select {
	case lt.lines <- line:
	case <-lt.stopCh:
	}
}

// readLastLines reads the last n complete lines of f and returns them with
// the offset at which live tailing should resume. A trailing line without a
// newline is still being written and is left for the live tail.
func readLastLines(f *os.File, n int) ([]string, int64) {
	info, err := f.Stat()
	if err != nil {
		return nil, 0
	}
	size := info.Size()
	if n <= 0 || size == 0 {
		return nil, size
	}

	const chunkSize = 64 * 1024
	const maxScan = 4 * 1024 * 1024

	var (
		data  []byte
		start = size
	)
	for start > 0 && int64(len(data)) < maxScan {
		chunk := min(int64(chunkSize), start)
		start -= chunk
		buf := make([]byte, chunk)
		if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
			return nil, size
		}
		data = append(buf, data...)
		if bytes.Count(data, []byte{'\n'}) > n {
			break
		}
	}

	offset := size
	if len(data) > 0 && data[len(data)-1] != '\n' {
		last := bytes.LastIndexByte(data, '\n')
		if last < 0 {
			// No complete line in the scanned window; tail live only.
			if start == 0 {
				return nil, 0
			}
			return nil, size
		}
		offset = size - int64(len(data)-last-1)
		data = data[:last+1]
	}

	text := strings.TrimRight(string(data), "\n")
	if text == "" {
		return nil, offset
	}
	lines := strings.Split(text, "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:] // first scanned line may be partial
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines, offset
}

var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences.
func stripANSI(s string) string {
	return ansiEscapeRegex.ReplaceAllString(s, "")
}
