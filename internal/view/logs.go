package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/DillonBarker/pm2ui/internal/pm2"
	"github.com/DillonBarker/pm2ui/internal/ui"
)

type logMode int

const (
	logModeAll    logMode = iota
	logModeSingle
	logModeMulti
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
	panel         *ui.LogPanel
	statusBar     *ui.StatusBar
	logFrame      *tview.Frame
	layout        *tview.Flex
	app           *ui.App
	tailer        *pm2.LogTailer
	multiTailer   *pm2.MultiLogTailer
	process       pm2.Process
	multiProcs    []pm2.Process
	allProcesses  []pm2.Process
	currentMode   logMode
	filter        pm2.LogFilter
	serviceColors map[string]string
	readStopCh    chan struct{}
}

// NewLogsView creates a new log viewer.
func NewLogsView(app *ui.App) *LogsView {
	panel := ui.NewLogPanel()
	statusBar := ui.NewStatusBar()
	statusBar.SetKeyHints([]ui.KeyHint{
		{Key: "t", Action: "toggle stream"},
		{Key: "a", Action: "autoscroll"},
		{Key: "w", Action: "wrap"},
		{Key: "0", Action: "tail"},
		{Key: "1", Action: "head"},
		{Key: "2", Action: "50L"},
		{Key: "3", Action: "100L"},
		{Key: "4", Action: "200L"},
		{Key: "5", Action: "500L"},
		{Key: "6", Action: "1000L"},
	})

	logFrame := tview.NewFrame(panel).
		SetBorders(0, 0, 0, 0, 1, 1)
	logFrame.SetBorder(true)
	logFrame.SetBorderColor(tcell.ColorDarkCyan)
	logFrame.SetBackgroundColor(tcell.ColorDefault)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusBar, 2, 0, false).
		AddItem(logFrame, 0, 1, true).
		AddItem(panel.Indicator(), 1, 0, false)

	panel.Indicator().SetBackgroundColor(tcell.ColorDarkSlateGray)

	lv := &LogsView{
		panel:         panel,
		statusBar:     statusBar,
		logFrame:      logFrame,
		layout:        layout,
		app:           app,
		serviceColors: make(map[string]string),
		readStopCh:    make(chan struct{}),
	}

	return lv
}

// Layout returns the root layout.
func (lv *LogsView) Layout() *tview.Flex {
	return lv.layout
}

// Mode returns the current log mode.
func (lv *LogsView) Mode() logMode {
	return lv.currentMode
}

// AllProcesses returns the process list last passed to ShowAll.
func (lv *LogsView) AllProcesses() []pm2.Process {
	return lv.allProcesses
}

// ShowAll starts tailing all given processes (default mode).
func (lv *LogsView) ShowAll(procs []pm2.Process) {
	lv.Stop()
	lv.stopReading()
	lv.currentMode = logModeAll
	lv.allProcesses = procs
	lv.panel.Reset()
	lv.updateHeader()

	lv.multiTailer = pm2.NewMultiLogTailer(procs, lv.filter)
	lv.multiTailer.Start()
	go lv.readLines(lv.multiTailer.Lines())
}

// ShowSingle starts tailing a single process.
func (lv *LogsView) ShowSingle(proc pm2.Process) {
	lv.Stop()
	lv.stopReading()
	lv.currentMode = logModeSingle
	lv.process = proc
	lv.panel.Reset()
	lv.updateHeader()

	lv.tailer = pm2.NewLogTailer(proc.PM2Env.PMOutLogPath, proc.PM2Env.PMErrLogPath, lv.filter)
	lv.tailer.Start()
	go lv.readLines(lv.tailer.Lines())
}

// ShowMulti starts tailing a subset of processes.
func (lv *LogsView) ShowMulti(procs []pm2.Process) {
	lv.Stop()
	lv.stopReading()
	lv.currentMode = logModeMulti
	lv.multiProcs = procs
	lv.panel.Reset()
	lv.updateHeader()

	lv.multiTailer = pm2.NewMultiLogTailer(procs, lv.filter)
	lv.multiTailer.Start()
	go lv.readLines(lv.multiTailer.Lines())
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

func (lv *LogsView) readLines(ch <-chan pm2.LogLine) {
	stopCh := lv.readStopCh
	for {
		select {
		case line := <-ch:
			text := line.Text
			if line.ProcessName != "" {
				color := lv.colorFor(line.ProcessName)
				text = fmt.Sprintf("[%s]%s[-] %s", color, line.ProcessName, text)
			} else if line.Stream == pm2.LogStderr {
				text = "[red]┃[-] " + text
			}
			captured := text
			lv.app.QueueUpdateDraw(func() {
				lv.panel.AppendLine(captured)
			})
		case <-stopCh:
			return
		}
	}
}

func (lv *LogsView) colorFor(name string) string {
	if c, ok := lv.serviceColors[name]; ok {
		return c
	}
	c := serviceColorPalette[len(lv.serviceColors)%len(serviceColorPalette)]
	lv.serviceColors[name] = c
	return c
}

// SetFilter changes the log filter and restarts the current tailer.
func (lv *LogsView) SetFilter(f pm2.LogFilter) {
	lv.filter = f
	lv.restartCurrent()
	lv.updateHeader()
}

func (lv *LogsView) restartCurrent() {
	switch lv.currentMode {
	case logModeAll:
		lv.ShowAll(lv.allProcesses)
	case logModeSingle:
		lv.ShowSingle(lv.process)
	case logModeMulti:
		lv.ShowMulti(lv.multiProcs)
	}
}

func (lv *LogsView) filterLabel() string {
	switch lv.filter.Mode {
	case pm2.FilterHead:
		return "head"
	case pm2.FilterLastN:
		return fmt.Sprintf("last:%d", lv.filter.Lines)
	default:
		return "tail"
	}
}

func (lv *LogsView) updateHeader() {
	label := lv.filterLabel()
	switch lv.currentMode {
	case logModeAll:
		lv.logFrame.SetTitle(fmt.Sprintf(" Logs — all services [%s] ", label))
	case logModeSingle:
		streamStr := "both"
		if lv.tailer != nil {
			switch lv.tailer.Stream() {
			case pm2.LogStdout:
				streamStr = "stdout"
			case pm2.LogStderr:
				streamStr = "stderr"
			}
		}
		lv.logFrame.SetTitle(fmt.Sprintf(" Logs — %s (%s) [%s] ", lv.process.Name, streamStr, label))
	case logModeMulti:
		names := make([]string, len(lv.multiProcs))
		for i, p := range lv.multiProcs {
			names[i] = p.Name
		}
		lv.logFrame.SetTitle(fmt.Sprintf(" Logs — %s [%s] ", strings.Join(names, ", "), label))
	}
	lv.logFrame.SetTitleColor(tcell.ColorAqua)
}

func (lv *LogsView) ToggleAutoScroll() { lv.panel.ToggleAutoScroll() }
func (lv *LogsView) ToggleWordWrap()   { lv.panel.ToggleWordWrap() }
func (lv *LogsView) ToggleStream()     { lv.toggleStream() }

func (lv *LogsView) toggleStream() {
	if lv.tailer == nil {
		return
	}
	current := lv.tailer.Stream()
	switch current {
	case pm2.LogBoth:
		lv.tailer.SetStream(pm2.LogStdout)
	case pm2.LogStdout:
		lv.tailer.SetStream(pm2.LogStderr)
	case pm2.LogStderr:
		lv.tailer.SetStream(pm2.LogBoth)
	}
	lv.updateHeader()
}
