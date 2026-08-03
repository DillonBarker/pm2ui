package view

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/DillonBarker/pm2ui/internal/config"
	"github.com/DillonBarker/pm2ui/internal/pm2"
	"github.com/DillonBarker/pm2ui/internal/ui"
)

// batchCap bounds lines buffered between flushes; overflow drops oldest.
const batchCap = 4000

type logMode int

const (
	logModeAll logMode = iota
	logModeSingle
)

var serviceColorPalette = []string{
	"#61afef", // blue
	"#98c379", // green
	"#e5c07b", // yellow
	"#c678dd", // purple
	"#56b6c2", // cyan
	"#e06c75", // red
	"#d19a66", // orange
	"#7fbbb3", // teal
	"#f472b6", // pink
	"#a9b665", // olive
	"#4ade80", // lime
	"#fb923c", // amber
	"#a78bfa", // lavender
	"#34d399", // emerald
	"#f87171", // coral
	"#60a5fa", // sky
	"#fbbf24", // gold
	"#e879f9", // fuchsia
	"#2dd4bf", // turquoise
	"#f9a8d4", // rose
}

// LogsView displays streaming logs for one or more processes.
type LogsView struct {
	panel           *ui.LogPanel
	statusBar       *ui.StatusBar
	logFrame        *tview.Frame
	layout          *tview.Flex
	app             *ui.App
	searchBar       *tview.InputField
	searchContainer *tview.Flex
	Searching       bool
	tailer          *pm2.LogTailer
	multiTailer     *pm2.MultiLogTailer
	process         pm2.Process
	allProcesses    []pm2.Process
	// selection holds the Space-selected service names shown as a display
	// filter over the all-services stream (nil = show everything).
	selection   []string
	currentMode logMode
	// override, when set via the 0-6 history keys, replaces the per-mode
	// default depth (allTail for merged mode, singleTail for single mode).
	override      *pm2.LogFilter
	allTail       int
	singleTail    int
	colorMu       sync.Mutex
	serviceColors map[string]string
	readStopCh    chan struct{}
	// gen identifies the active tail session; queued appends from stale
	// sessions are dropped. Only touched on the event loop.
	gen           int
	batchInterval time.Duration
	// paused and timestamps are read by the readLines goroutine.
	paused     atomic.Bool
	timestamps atomic.Bool
}

// NewLogsView creates a new log viewer.
func NewLogsView(app *ui.App, cfg config.Config) *LogsView {
	panel := ui.NewLogPanel()
	panel.SetMaxLines(cfg.MaxLogLines)
	statusBar := ui.NewStatusBar()
	statusBar.SetKeyHints([]ui.KeyHint{
		{Key: "/", Action: "search"},
		{Key: "m", Action: "mark"},
		{Key: "c", Action: "clear"},
		{Key: "p", Action: "pause"},
		{Key: "a", Action: "scroll"},
		{Key: "w", Action: "wrap"},
		{Key: "t", Action: "stream"},
		{Key: "f", Action: "full"},
		{Key: "S", Action: "save"},
		{Key: "0-6", Action: "history"},
	})

	logFrame := tview.NewFrame(panel).
		SetBorders(0, 0, 0, 0, 1, 1)
	logFrame.SetBorder(true)
	logFrame.SetBorderColor(tcell.ColorDarkCyan)
	logFrame.SetBackgroundColor(tcell.ColorDefault)

	searchBar := tview.NewInputField()
	searchBar.SetLabel(" / ")
	searchBar.SetLabelColor(tcell.ColorYellow)
	searchBar.SetFieldBackgroundColor(tcell.ColorDefault)
	searchBar.SetFieldTextColor(tcell.ColorWhite)
	searchBar.SetBackgroundColor(tcell.ColorDefault)

	searchContainer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(searchBar, 1, 0, true)
	searchContainer.SetBorder(true)
	searchContainer.SetBorderColor(tcell.ColorYellow)
	searchContainer.SetTitle(" Search Logs ")
	searchContainer.SetTitleColor(tcell.ColorYellow)
	searchContainer.SetBackgroundColor(tcell.ColorDefault)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusBar, 2, 0, false).
		AddItem(searchContainer, 0, 0, false). // hidden until '/' opens it
		AddItem(logFrame, 0, 1, true).
		AddItem(panel.Indicator(), 1, 0, false)

	panel.Indicator().SetBackgroundColor(tcell.ColorDarkSlateGray)

	lv := &LogsView{
		panel:           panel,
		statusBar:       statusBar,
		logFrame:        logFrame,
		layout:          layout,
		app:             app,
		searchBar:       searchBar,
		searchContainer: searchContainer,
		serviceColors:   make(map[string]string),
		readStopCh:      make(chan struct{}),
		allTail:         cfg.AllTailLines,
		singleTail:      cfg.DefaultTailLines,
		batchInterval:   cfg.LogBatchInterval,
	}

	searchBar.SetChangedFunc(func(text string) {
		if text == "" {
			lv.panel.ClearSearch()
			return
		}
		re, err := regexp.Compile("(?i)" + text)
		if err != nil {
			re = regexp.MustCompile("(?i)" + regexp.QuoteMeta(text))
		}
		lv.panel.SetSearch(re)
	})

	searchBar.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			lv.ClearSearchUI()
			return nil
		case tcell.KeyEnter:
			lv.Searching = false
			if searchBar.GetText() == "" {
				lv.layout.ResizeItem(lv.searchContainer, 0, 0)
			}
			lv.app.SetFocus(lv.panel)
			return nil
		}
		return event
	})

	return lv
}

// StartSearch opens the log search bar and focuses it.
func (lv *LogsView) StartSearch() {
	if lv.Searching {
		return
	}
	lv.Searching = true
	lv.layout.ResizeItem(lv.searchContainer, 3, 0)
	lv.app.SetFocus(lv.searchContainer)
}

// ClearSearchUI clears the search filter and hides the bar.
func (lv *LogsView) ClearSearchUI() {
	lv.Searching = false
	lv.searchBar.SetText("") // triggers ClearSearch via the changed func
	lv.layout.ResizeItem(lv.searchContainer, 0, 0)
	lv.app.SetFocus(lv.panel)
}

// HasActiveSearch reports whether a search filter is applied.
func (lv *LogsView) HasActiveSearch() bool {
	return lv.panel.HasSearch() || lv.Searching
}

// Layout returns the root layout.
func (lv *LogsView) Layout() *tview.Flex {
	return lv.layout
}

// Mode returns the current log mode.
func (lv *LogsView) Mode() logMode {
	return lv.currentMode
}

// effectiveFilter returns the history filter for the given mode: the user's
// 0-6 override if set, otherwise the per-mode default depth.
func (lv *LogsView) effectiveFilter(mode logMode) pm2.LogFilter {
	if lv.override != nil {
		return *lv.override
	}
	if mode == logModeAll {
		return pm2.LogFilter{Mode: pm2.FilterLastN, Lines: lv.allTail}
	}
	return pm2.LogFilter{Mode: pm2.FilterLastN, Lines: lv.singleTail}
}

// ShowAll starts tailing all given processes (default mode).
func (lv *LogsView) ShowAll(procs []pm2.Process) {
	lv.beginSession(logModeAll)
	lv.allProcesses = procs
	lv.updateHeader()

	lv.multiTailer = pm2.NewMultiLogTailer(procs, lv.effectiveFilter(logModeAll))
	lv.multiTailer.Start()
	go lv.readLines(lv.gen, lv.multiTailer.Lines(), lv.readStopCh)
}

// ShowSingle starts tailing a single process.
func (lv *LogsView) ShowSingle(proc pm2.Process) {
	lv.beginSession(logModeSingle)
	lv.process = proc
	lv.updateHeader()

	lv.tailer = pm2.NewLogTailer(proc.PM2Env.PMOutLogPath, proc.PM2Env.PMErrLogPath, lv.effectiveFilter(logModeSingle))
	lv.tailer.Start()
	go lv.readLines(lv.gen, lv.tailer.Lines(), lv.readStopCh)
}

// ApplySelection shows logs for the Space-selected services as a display
// filter over the all-services stream — no tailer restart, no buffer loss.
// If the all-services session isn't running (e.g. single mode), it starts
// one first. An empty selection restores the full stream.
func (lv *LogsView) ApplySelection(all []pm2.Process, selected []pm2.Process) {
	if lv.currentMode != logModeAll || lv.multiTailer == nil {
		lv.ShowAll(all)
	}
	var names []string
	for _, p := range selected {
		names = append(names, p.Name)
	}
	lv.selection = names
	lv.panel.SetServiceFilter(names)
	lv.updateHeader()
}

// IsShowingSingle reports whether single-mode logs for name are active.
func (lv *LogsView) IsShowingSingle(name string) bool {
	return lv.currentMode == logModeSingle && lv.process.Name == name
}

// UpdateProcessSet reconciles the all-services session with a new process
// list: new services start streaming, removed ones stop. The existing
// buffer is untouched.
func (lv *LogsView) UpdateProcessSet(procs []pm2.Process) {
	if lv.currentMode != logModeAll || lv.multiTailer == nil {
		return
	}
	current := make(map[string]bool, len(lv.allProcesses))
	for _, p := range lv.allProcesses {
		current[p.Name] = true
	}
	next := make(map[string]bool, len(procs))
	for _, p := range procs {
		next[p.Name] = true
		if !current[p.Name] {
			lv.multiTailer.Add(p)
		}
	}
	for name := range current {
		if !next[name] {
			lv.multiTailer.Remove(name)
		}
	}
	lv.allProcesses = procs
}

// beginSession tears down the previous tail session and prepares a new one.
// Must be called from the event loop.
func (lv *LogsView) beginSession(mode logMode) {
	lv.Stop()
	lv.stopReading()
	lv.gen++
	lv.currentMode = mode
	lv.selection = nil
	if lv.HasActiveSearch() {
		lv.Searching = false
		lv.searchBar.SetText("")
		lv.layout.ResizeItem(lv.searchContainer, 0, 0)
	}
	lv.panel.Reset()
}

// Stop stops the active tailer.
func (lv *LogsView) Stop() {
	if lv.tailer != nil {
		lv.tailer.Stop()
		lv.tailer = nil
	}
	if lv.multiTailer != nil {
		lv.multiTailer.Stop()
		lv.multiTailer = nil
	}
}

func (lv *LogsView) stopReading() {
	close(lv.readStopCh)
	lv.readStopCh = make(chan struct{})
}

// readLines batches incoming lines and flushes them to the panel on a fixed
// interval with a single queued draw per flush. Runs on its own goroutine.
func (lv *LogsView) readLines(gen int, ch <-chan pm2.LogLine, stopCh <-chan struct{}) {
	ticker := time.NewTicker(lv.batchInterval)
	defer ticker.Stop()

	var batch []ui.LogEntry
	dropped := 0

	flush := func() {
		if len(batch) == 0 && dropped == 0 {
			return
		}
		entries := batch
		batch = nil
		n := dropped
		dropped = 0
		lv.app.QueueUpdateDraw(func() {
			if gen != lv.gen {
				return // stale session; a newer view owns the panel now
			}
			if n > 0 {
				entries = append([]ui.LogEntry{{
					Display: fmt.Sprintf("[red](dropped %d lines)[-]", n),
					Plain:   fmt.Sprintf("(dropped %d lines)", n),
				}}, entries...)
			}
			lv.panel.AppendLines(entries)
		})
	}

	for {
		select {
		case line := <-ch:
			batch = append(batch, lv.decorate(line))
			if len(batch) >= batchCap {
				if lv.paused.Load() {
					// Can't render while paused; keep newest, note the loss.
					over := len(batch) - batchCap
					dropped += over
					batch = append([]ui.LogEntry(nil), batch[over:]...)
				} else {
					// Big burst (e.g. initial dump): flush early instead of
					// dropping — QueueUpdateDraw provides the backpressure.
					flush()
				}
			}
		case <-ticker.C:
			if !lv.paused.Load() {
				flush()
			}
		case <-stopCh:
			return
		}
	}
}

// decorate renders a LogLine into display and plain forms, adding the
// service prefix (multi mode) or stderr marker (single mode).
func (lv *LogsView) decorate(line pm2.LogLine) ui.LogEntry {
	display := line.Text
	plain := line.Plain
	if line.ProcessName != "" {
		color := lv.colorFor(line.ProcessName)
		display = fmt.Sprintf("[%s]%s[-] %s", color, line.ProcessName, display)
		plain = line.ProcessName + " " + plain
	} else if line.Stream == pm2.LogStderr {
		display = "[red]┃[-] " + display
		plain = "┃ " + plain
	}
	if lv.timestamps.Load() {
		ts := line.Time.Format("15:04:05")
		display = fmt.Sprintf("[gray]%s[-] %s", ts, display)
		plain = ts + " " + plain
	}
	return ui.LogEntry{Display: display, Plain: plain, Process: line.ProcessName}
}

func (lv *LogsView) colorFor(name string) string {
	lv.colorMu.Lock()
	defer lv.colorMu.Unlock()
	if c, ok := lv.serviceColors[name]; ok {
		return c
	}
	c := serviceColorPalette[len(lv.serviceColors)%len(serviceColorPalette)]
	lv.serviceColors[name] = c
	return c
}

// SetFilter overrides the history depth and restarts the current tailer.
// Selecting the already-active filter is a no-op.
func (lv *LogsView) SetFilter(f pm2.LogFilter) {
	if lv.override != nil && *lv.override == f {
		return
	}
	lv.override = &f
	lv.restartCurrent()
	lv.updateHeader()
}

func (lv *LogsView) restartCurrent() {
	switch lv.currentMode {
	case logModeAll:
		selection := lv.selection
		lv.ShowAll(lv.allProcesses)
		if len(selection) > 0 {
			lv.selection = selection
			lv.panel.SetServiceFilter(selection)
		}
	case logModeSingle:
		lv.ShowSingle(lv.process)
	}
}

func (lv *LogsView) filterLabel() string {
	f := lv.effectiveFilter(lv.currentMode)
	switch f.Mode {
	case pm2.FilterHead:
		return "head"
	case pm2.FilterLastN:
		return fmt.Sprintf("last:%d", f.Lines)
	default:
		return "tail"
	}
}

func (lv *LogsView) updateHeader() {
	label := lv.filterLabel()
	stream := lv.streamLabel()
	switch lv.currentMode {
	case logModeAll:
		if len(lv.selection) > 0 {
			lv.logFrame.SetTitle(fmt.Sprintf(" Logs — %s (%s) [%s] ", strings.Join(lv.selection, ", "), stream, label))
		} else {
			lv.logFrame.SetTitle(fmt.Sprintf(" Logs — all services (%s) [%s] ", stream, label))
		}
	case logModeSingle:
		lv.logFrame.SetTitle(fmt.Sprintf(" Logs — %s (%s) [%s] ", lv.process.Name, stream, label))
	}
	lv.logFrame.SetTitleColor(tcell.ColorAqua)
}

func (lv *LogsView) ToggleAutoScroll() { lv.panel.ToggleAutoScroll() }
func (lv *LogsView) ToggleWordWrap()   { lv.panel.ToggleWordWrap() }
func (lv *LogsView) ToggleStream()     { lv.toggleStream() }

// AddMark appends a timestamped separator line — k9s's `m`. It bypasses
// search and selection filters so it always shows.
func (lv *LogsView) AddMark() {
	ts := time.Now().Format("15:04:05")
	lv.panel.AppendLines([]ui.LogEntry{{
		Display: fmt.Sprintf("[gray]────────── mark %s ──────────[-]", ts),
		Plain:   fmt.Sprintf("────────── mark %s ──────────", ts),
		Mark:    true,
	}})
}

// ClearPanel empties the visible logs but keeps tailers, search and
// selection filters running — k9s's `c`.
func (lv *LogsView) ClearPanel() {
	lv.panel.ClearContent()
}

// SetFocused highlights or dims the log frame border.
func (lv *LogsView) SetFocused(on bool) {
	if on {
		lv.logFrame.SetBorderColor(tcell.ColorAqua)
	} else {
		lv.logFrame.SetBorderColor(tcell.ColorDarkCyan)
	}
}

// TogglePause suspends flushing to the panel; lines buffer meanwhile.
func (lv *LogsView) TogglePause() {
	p := !lv.paused.Load()
	lv.paused.Store(p)
	lv.panel.SetPausedIndicator(p)
}

// ToggleTimestamps toggles arrival-time prefixes on incoming lines.
func (lv *LogsView) ToggleTimestamps() {
	lv.timestamps.Store(!lv.timestamps.Load())
}

// SaveBuffer writes the current log buffer (plain text) to a temp file.
func (lv *LogsView) SaveBuffer() (string, error) {
	entries := lv.panel.Buffer()
	if len(entries) == 0 {
		return "", fmt.Errorf("no log lines to save")
	}
	label := "all"
	switch {
	case lv.currentMode == logModeSingle:
		label = lv.process.Name
	case len(lv.selection) > 0:
		label = "selection"
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("pm2ui-%s-%d.log", label, time.Now().Unix()))
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.Plain)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (lv *LogsView) toggleStream() {
	next := func(s pm2.LogStream) pm2.LogStream {
		switch s {
		case pm2.LogBoth:
			return pm2.LogStdout
		case pm2.LogStdout:
			return pm2.LogStderr
		default:
			return pm2.LogBoth
		}
	}
	switch {
	case lv.tailer != nil:
		lv.tailer.SetStream(next(lv.tailer.Stream()))
	case lv.multiTailer != nil:
		lv.multiTailer.SetStream(next(lv.multiTailer.Stream()))
	default:
		return
	}
	lv.updateHeader()
}

// streamLabel returns the active stream mode as a display string.
func (lv *LogsView) streamLabel() string {
	s := pm2.LogBoth
	switch {
	case lv.tailer != nil:
		s = lv.tailer.Stream()
	case lv.multiTailer != nil:
		s = lv.multiTailer.Stream()
	}
	switch s {
	case pm2.LogStdout:
		return "stdout"
	case pm2.LogStderr:
		return "stderr"
	default:
		return "both"
	}
}
