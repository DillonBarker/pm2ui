package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/DillonBarker/pm2ui/internal/config"
	"github.com/DillonBarker/pm2ui/internal/model"
	"github.com/DillonBarker/pm2ui/internal/pm2"
	"github.com/DillonBarker/pm2ui/internal/ui"
)

// Layout orchestrates views and page navigation.
type Layout struct {
	app          *ui.App
	client       *pm2.Client
	watcher      *pm2.Watcher
	processView  *ProcessView
	logsView     *LogsView
	processModel *model.ProcessTable
	flashWidget  *ui.FlashWidget
	splitView    *tview.Flex
	cfg          config.Config
	logsFocused  bool
	fullscreen   bool
	startupWarn  error
	// describeName is the process shown on the describe page ("" = closed);
	// its view refreshes on every watcher poll.
	describeName string
	describeView *tview.TextView
	// prevProcs is only touched on the watcher's poll goroutine.
	prevProcs map[string]procSnapshot
}

type procSnapshot struct {
	status   string
	restarts int
}

// NewLayout creates the main application layout. cfgWarn, if non-nil, is
// surfaced as a flash once the UI is up (config problems fall back to
// defaults rather than aborting).
func NewLayout(cfg config.Config, cfgWarn error) *Layout {
	app := ui.NewApp(cfg.MouseEnabled)
	client := pm2.NewClient()
	watcher := pm2.NewWatcher(client, cfg.RefreshInterval)
	processModel := model.NewProcessTable()
	processView := NewProcessView(app, processModel)
	logsView := NewLogsView(app, cfg)
	flashWidget := ui.NewFlashWidget()
	flashWidget.SetApp(app.Application)

	l := &Layout{
		app:          app,
		client:       client,
		watcher:      watcher,
		processView:  processView,
		logsView:     logsView,
		processModel: processModel,
		flashWidget:  flashWidget,
		cfg:          cfg,
		startupWarn:  cfgWarn,
	}

	l.wireActions()
	l.processView.SetOnCommand(func(cmd string) {
		if cmd == "ns" || strings.HasPrefix(cmd, "ns ") {
			ns := strings.TrimSpace(strings.TrimPrefix(cmd, "ns"))
			l.processModel.SetNamespace(ns)
			l.processView.Refresh()
			if ns == "" {
				l.Flash(ui.FlashInfo, "Namespace scope cleared")
			} else {
				l.Flash(ui.FlashInfo, fmt.Sprintf("Scoped to namespace %q", ns))
			}
			return
		}
		switch cmd {
		case "q", "q!":
			l.logsView.Stop()
			l.app.Stop()
		case "restart all":
			go func() {
				if err := l.client.RestartAll(); err != nil {
					l.app.QueueUpdateDraw(func() {
						l.Flash(ui.FlashError, fmt.Sprintf("Restart all failed: %v", err))
					})
					return
				}
				l.watcher.Refresh()
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashInfo, "All processes restarted")
				})
			}()
		case "stop all":
			l.confirm("Stop all processes?", func() {
				go func() {
					if err := l.client.StopAll(); err != nil {
						l.app.QueueUpdateDraw(func() {
							l.Flash(ui.FlashError, fmt.Sprintf("Stop all failed: %v", err))
						})
						return
					}
					l.watcher.Refresh()
					l.app.QueueUpdateDraw(func() {
						l.Flash(ui.FlashInfo, "All processes stopped")
					})
				}()
			})
		case "reload all":
			go func() {
				if err := l.client.ReloadAll(); err != nil {
					l.app.QueueUpdateDraw(func() {
						l.Flash(ui.FlashError, fmt.Sprintf("Reload all failed: %v", err))
					})
					return
				}
				l.watcher.Refresh()
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashInfo, "All processes reloaded")
				})
			}()
		case "save":
			go func() {
				if err := l.client.Save(); err != nil {
					l.app.QueueUpdateDraw(func() {
						l.Flash(ui.FlashError, fmt.Sprintf("Save failed: %v", err))
					})
					return
				}
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashInfo, "Process list saved")
				})
			}()
		case "flush":
			go func() {
				if err := l.client.Flush(); err != nil {
					l.app.QueueUpdateDraw(func() {
						l.Flash(ui.FlashError, fmt.Sprintf("Flush failed: %v", err))
					})
					return
				}
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashInfo, "Logs flushed")
				})
			}()
		}
	})
	return l
}

// Run starts the TUI application.
func (l *Layout) Run() error {
	initialized := false
	l.watcher.OnUpdate(func(procs []pm2.Process) {
		l.processModel.Update(procs)
		setChanged := l.notifyStateChanges(procs)
		if !initialized {
			initialized = true
			l.app.QueueUpdateDraw(func() {
				l.logsView.ShowAll(procs)
			})
			return
		}
		if setChanged {
			// Processes appeared or vanished: reconcile the log stream
			// in place instead of restarting it.
			l.app.QueueUpdateDraw(func() {
				l.logsView.UpdateProcessSet(procs)
			})
		}
		l.refreshDescribe(procs)
	})

	l.watcher.OnError(func(err error) {
		l.app.QueueUpdateDraw(func() {
			l.Flash(ui.FlashError, fmt.Sprintf("PM2: %v", err))
		})
	})

	l.processModel.OnChange(func(procs []pm2.Process) {
		l.processView.UpdateProcesses(procs)
	})

	// Add flash widget to process view layout
	l.processView.Layout().AddItem(l.flashWidget, 1, 0, false)

	// Split view: processes left, logs right
	l.splitView = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(l.processView.Layout(), 0, 1, true).
		AddItem(l.logsView.Layout(), 0, l.cfg.SplitRatio, false)

	l.app.Pages.AddPage("main", l.splitView, true, true)

	l.setupGlobalKeys()

	l.watcher.Start()
	defer l.watcher.Stop()

	if l.startupWarn != nil {
		warn := l.startupWarn
		go func() {
			time.Sleep(300 * time.Millisecond) // let the app start drawing
			l.app.QueueUpdateDraw(func() {
				l.Flash(ui.FlashWarn, fmt.Sprintf("Config: %v (using defaults)", warn))
			})
		}()
	}

	return l.app.Run()
}

func (l *Layout) wireActions() {
	// Focus state is centralized here so keyboard (SetFocus) and mouse
	// clicks stay in sync; every focus change lands in these callbacks.
	l.logsView.panel.SetFocusFunc(func() {
		l.logsFocused = true
		l.logsView.SetFocused(true)
		l.processView.SetFocused(false)
		l.logsView.statusBar.SetDimmed(false)
		l.processView.statusBar.SetDimmed(true)
	})
	l.processView.table.SetFocusFunc(func() {
		l.logsFocused = false
		l.logsView.SetFocused(false)
		l.processView.SetFocused(true)
		l.logsView.statusBar.SetDimmed(true)
		l.processView.statusBar.SetDimmed(false)
	})

	l.processView.SetOnViewLogs(func(proc pm2.Process) {
		if !l.logsView.IsShowingSingle(proc.Name) {
			l.logsView.ShowSingle(proc)
		}
		l.focusLogs()
	})

	l.processView.SetOnDescribe(func(proc pm2.Process) {
		l.describeView = newDescribeView(proc)
		l.describeName = proc.Name
		l.app.Pages.AddPage("describe", l.describeView, true, true)
		l.app.SetFocus(l.describeView)
	})

	l.processView.SetOnSelectionChange(func(selected []pm2.Process) {
		l.logsView.ApplySelection(l.processModel.Raw(), selected)
	})

	l.processView.SetOnRestart(func(procs []pm2.Process) {
		l.runBulk("Restart", "Restarted", procs, l.client.Restart, nil)
	})

	l.processView.SetOnStop(func(procs []pm2.Process) {
		l.confirm(bulkPrompt("Stop", procs), func() {
			l.runBulk("Stop", "Stopped", procs, l.client.Stop, nil)
		})
	})

	l.processView.SetOnStart(func(procs []pm2.Process) {
		var stopped []pm2.Process
		for _, p := range procs {
			if p.PM2Env.Status != pm2.StatusOnline {
				stopped = append(stopped, p)
			}
		}
		if len(stopped) == 0 {
			return
		}
		l.runBulk("Start", "Started", stopped, l.client.Start, nil)
	})

	l.processView.SetOnDelete(func(procs []pm2.Process) {
		l.confirm(bulkPrompt("Delete", procs), func() {
			l.runBulk("Delete", "Deleted", procs, l.client.Delete, func(succeeded []string) {
				l.processView.DeselectNames(succeeded)
			})
		})
	})
}

// bulkPrompt phrases a confirmation for one or many targets.
func bulkPrompt(verb string, procs []pm2.Process) string {
	if len(procs) == 1 {
		return fmt.Sprintf("%s %s?", verb, procs[0].Name)
	}
	return fmt.Sprintf("%s %d processes?", verb, len(procs))
}

// runBulk applies a pm2 action to every target on a worker goroutine and
// flashes one summary. onDone (optional) runs on the event loop with the
// names that succeeded.
func (l *Layout) runBulk(verb, done string, procs []pm2.Process, action func(string) error, onDone func(succeeded []string)) {
	go func() {
		var succeeded, failed []string
		var firstErr error
		for _, p := range procs {
			if err := action(p.Name); err != nil {
				failed = append(failed, p.Name)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			succeeded = append(succeeded, p.Name)
		}
		l.watcher.Refresh()
		l.app.QueueUpdateDraw(func() {
			switch {
			case len(failed) > 0:
				l.Flash(ui.FlashError, fmt.Sprintf("%s failed for %s: %v", verb, strings.Join(failed, ", "), firstErr))
			case len(succeeded) == 1:
				l.Flash(ui.FlashInfo, fmt.Sprintf("%s %s", done, succeeded[0]))
			default:
				l.Flash(ui.FlashInfo, fmt.Sprintf("%s %d processes", done, len(succeeded)))
			}
			if onDone != nil && len(succeeded) > 0 {
				onDone(succeeded)
			}
		})
	}()
}

func (l *Layout) setupGlobalKeys() {
	l.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Overlay pages (help, confirm, describe) own the keyboard; global
		// shortcuts must not fire underneath them.
		if front, _ := l.app.Pages.GetFrontPage(); front != "main" {
			isEsc := event.Key() == tcell.KeyEsc
			isRune := func(r rune) bool {
				return event.Key() == tcell.KeyRune && event.Rune() == r
			}
			if (front == "help" && (isEsc || isRune('?'))) ||
				(front == "describe" && (isEsc || isRune('i'))) {
				if front == "describe" {
					l.describeName = ""
					l.describeView = nil
				}
				l.app.Pages.RemovePage(front)
				l.focusTable()
				return nil
			}
			return event
		}

		if l.processView.Filtering {
			if event.Key() == tcell.KeyEscape {
				l.processView.ClearFilter()
				return nil
			}
			return event
		}

		if l.processView.Commanding {
			return event
		}

		if l.logsView.Searching {
			return event // search bar owns input; its capture handles Esc/Enter
		}

		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'l':
				l.focusLogs()
				return nil
			case '?':
				l.showHelp()
				return nil
			case ':':
				if l.fullscreen {
					l.toggleFullscreen() // command bar lives in the process pane
				}
				l.processView.startCmd()
				return nil
			case 'm':
				l.logsView.AddMark()
				return nil
			case 'c':
				l.logsView.ClearPanel()
				return nil
			case 'a':
				l.logsView.ToggleAutoScroll()
				return nil
			case 'w':
				l.logsView.ToggleWordWrap()
				return nil
			case 't':
				l.logsView.ToggleStream()
				return nil
			case 'p':
				l.logsView.TogglePause()
				return nil
			case 'T':
				l.logsView.ToggleTimestamps()
				return nil
			case 'f':
				l.toggleFullscreen()
				return nil
			case 'S':
				if l.logsFocused {
					l.saveLogs()
					return nil
				}
				return event // table uses S for sort-by-status
			case '/':
				if l.logsFocused {
					l.logsView.StartSearch()
					return nil
				}
				return event // table handles '/' as process filter
			case '0':
				l.logsView.SetFilter(pm2.LogFilter{})
				return nil
			case '1':
				l.logsView.SetFilter(pm2.LogFilter{Mode: pm2.FilterHead})
				return nil
			case '2':
				l.logsView.SetFilter(pm2.LogFilter{Mode: pm2.FilterLastN, Lines: 50})
				return nil
			case '3':
				l.logsView.SetFilter(pm2.LogFilter{Mode: pm2.FilterLastN, Lines: 100})
				return nil
			case '4':
				l.logsView.SetFilter(pm2.LogFilter{Mode: pm2.FilterLastN, Lines: 200})
				return nil
			case '5':
				l.logsView.SetFilter(pm2.LogFilter{Mode: pm2.FilterLastN, Lines: 500})
				return nil
			case '6':
				l.logsView.SetFilter(pm2.LogFilter{Mode: pm2.FilterLastN, Lines: 1000})
				return nil
			}
		case tcell.KeyEsc:
			if l.logsView.HasActiveSearch() {
				l.logsView.ClearSearchUI()
				return nil
			}
			if l.fullscreen {
				l.toggleFullscreen()
				l.focusTable()
				return nil
			}
			if l.logsFocused {
				// One press goes all the way back: leave single mode too.
				if l.logsView.Mode() == logModeSingle {
					l.logsView.ApplySelection(l.processModel.Raw(), l.processView.SelectedProcesses())
				}
				l.focusTable()
				return nil
			}
			if l.logsView.Mode() == logModeSingle {
				l.logsView.ApplySelection(l.processModel.Raw(), l.processView.SelectedProcesses())
				l.focusTable()
				return nil
			}
			if len(l.processView.SelectedProcesses()) > 0 {
				l.processView.ClearSelections()
				return nil
			}
			if l.processModel.Filter() != "" {
				l.processView.ClearFilter()
				return nil
			}
		}
		return event
	})
}

// notifyStateChanges flashes when a process crashes or its restart count
// climbs, and reports whether the set of process names changed. Runs on
// the watcher goroutine.
func (l *Layout) notifyStateChanges(procs []pm2.Process) (setChanged bool) {
	if l.prevProcs == nil {
		l.prevProcs = make(map[string]procSnapshot, len(procs))
		for _, p := range procs {
			l.prevProcs[p.Name] = procSnapshot{p.PM2Env.Status, p.PM2Env.RestartTime}
		}
		return false
	}

	type note struct {
		level ui.FlashLevel
		msg   string
	}
	var notes []note
	next := make(map[string]procSnapshot, len(procs))
	for _, p := range procs {
		snap := procSnapshot{p.PM2Env.Status, p.PM2Env.RestartTime}
		next[p.Name] = snap
		prev, ok := l.prevProcs[p.Name]
		if !ok {
			setChanged = true
			continue
		}
		switch {
		case snap.status == pm2.StatusErrored && prev.status != pm2.StatusErrored:
			notes = append(notes, note{ui.FlashError, fmt.Sprintf("%s errored", p.Name)})
		case snap.restarts > prev.restarts:
			notes = append(notes, note{ui.FlashWarn, fmt.Sprintf("%s restarted (↺ %d)", p.Name, snap.restarts)})
		}
	}
	if len(next) != len(l.prevProcs) {
		setChanged = true
	}
	l.prevProcs = next

	if len(notes) > 0 {
		l.app.QueueUpdateDraw(func() {
			for _, n := range notes {
				l.Flash(n.level, n.msg)
			}
		})
	}
	return setChanged
}

// refreshDescribe re-renders the describe page with fresh process data.
// Runs on the watcher goroutine.
func (l *Layout) refreshDescribe(procs []pm2.Process) {
	name := l.describeName
	if name == "" {
		return
	}
	for _, p := range procs {
		if p.Name == name {
			proc := p
			l.app.QueueUpdateDraw(func() {
				if l.describeName == proc.Name && l.describeView != nil {
					l.describeView.SetText(describeText(proc))
				}
			})
			return
		}
	}
}

// focusLogs and focusTable move keyboard focus; the SetFocusFunc callbacks
// update logsFocused, borders and hint dimming.
func (l *Layout) focusLogs()  { l.app.SetFocus(l.logsView.panel) }
func (l *Layout) focusTable() { l.app.SetFocus(l.processView.table) }

// toggleFullscreen collapses the process pane so logs take the full width.
func (l *Layout) toggleFullscreen() {
	l.fullscreen = !l.fullscreen
	if l.fullscreen {
		l.splitView.ResizeItem(l.processView.Layout(), 0, 0)
		l.focusLogs()
	} else {
		l.splitView.ResizeItem(l.processView.Layout(), 0, 1)
	}
}

func (l *Layout) saveLogs() {
	path, err := l.logsView.SaveBuffer()
	if err != nil {
		l.Flash(ui.FlashError, fmt.Sprintf("Save logs failed: %v", err))
		return
	}
	l.Flash(ui.FlashInfo, fmt.Sprintf("Logs saved to %s", path))
}

func (l *Layout) showHelp() {
	name, _ := l.app.Pages.GetFrontPage()
	if name == "help" {
		l.app.Pages.RemovePage("help")
		return
	}
	helpView := newHelpView()
	l.app.Pages.AddPage("help", helpView, true, true)
	l.app.SetFocus(helpView)
}

func (l *Layout) confirm(message string, onConfirm func()) {
	modal := ui.ConfirmDialog(message, onConfirm, func() {
		l.app.Pages.RemovePage("confirm")
		l.focusTable()
	})
	l.app.Pages.AddPage("confirm", modal, true, true)
}

// Flash shows a flash message.
func (l *Layout) Flash(level ui.FlashLevel, msg string) {
	l.flashWidget.Show(level, msg)
}
