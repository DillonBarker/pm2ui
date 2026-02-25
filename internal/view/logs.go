package view

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/DillonBarker/pm2ui/internal/pm2"
	"github.com/DillonBarker/pm2ui/internal/ui"
)

// LogsView displays streaming logs for a process.
type LogsView struct {
	panel     *ui.LogPanel
	statusBar *ui.StatusBar
	logFrame  *tview.Frame
	layout    *tview.Flex
	app       *ui.App
	tailer    *pm2.LogTailer
	process   pm2.Process
}

// NewLogsView creates a new log viewer.
func NewLogsView(app *ui.App) *LogsView {
	panel := ui.NewLogPanel()
	statusBar := ui.NewStatusBar()
	statusBar.SetKeyHints([]ui.KeyHint{
		{Key: "t", Action: "toggle stream"},
		{Key: "a", Action: "autoscroll"},
		{Key: "w", Action: "wrap"},
		{Key: "Esc", Action: "back"},
		{Key: "q", Action: "quit"},
	})

	// Log panel in a bordered frame
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
		panel:     panel,
		statusBar: statusBar,
		logFrame:  logFrame,
		layout:    layout,
		app:       app,
	}

	lv.setupKeys()
	return lv
}

// Layout returns the root layout.
func (lv *LogsView) Layout() *tview.Flex {
	return lv.layout
}

// Show starts tailing logs for the given process.
func (lv *LogsView) Show(proc pm2.Process) {
	lv.process = proc
	lv.panel.Reset()
	lv.updateHeader()

	if lv.tailer != nil {
		lv.tailer.Stop()
	}

	lv.tailer = pm2.NewLogTailer(proc.PM2Env.PMOutLogPath, proc.PM2Env.PMErrLogPath)
	lv.tailer.Start()

	go lv.readLines()
}

// Stop stops the current tailer.
func (lv *LogsView) Stop() {
	if lv.tailer != nil {
		lv.tailer.Stop()
		lv.tailer = nil
	}
}

func (lv *LogsView) readLines() {
	if lv.tailer == nil {
		return
	}
	for line := range lv.tailer.Lines() {
		text := line.Text
		if line.Stream == pm2.LogStderr {
			// Prefix stderr with a red marker; don't wrap the whole line so
			// ANSI colors within the text are preserved.
			text = "[red]┃[-] " + text
		}
		lv.app.QueueUpdateDraw(func() {
			lv.panel.AppendLine(text)
		})
	}
}

func (lv *LogsView) updateHeader() {
	streamStr := "both"
	if lv.tailer != nil {
		switch lv.tailer.Stream() {
		case pm2.LogStdout:
			streamStr = "stdout"
		case pm2.LogStderr:
			streamStr = "stderr"
		}
	}
	lv.logFrame.SetTitle(fmt.Sprintf(" %s — %s ", lv.process.Name, streamStr))
	lv.logFrame.SetTitleColor(tcell.ColorAqua)
}

func (lv *LogsView) setupKeys() {
	lv.panel.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 't':
				lv.toggleStream()
				return nil
			case 'a':
				lv.panel.ToggleAutoScroll()
				return nil
			case 'w':
				lv.panel.ToggleWordWrap()
				return nil
			}
		}
		return event
	})
}

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
