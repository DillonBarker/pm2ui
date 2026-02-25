package view

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"

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
}

// NewLayout creates the main application layout.
func NewLayout() *Layout {
	app := ui.NewApp()
	client := pm2.NewClient()
	watcher := pm2.NewWatcher(client, 2*time.Second)
	processModel := model.NewProcessTable()
	processView := NewProcessView(app, processModel)
	logsView := NewLogsView(app)
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
	}

	l.wireActions()
	l.processView.SetOnCommand(func(cmd string) {
		switch cmd {
		case "q!":
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
	// Wire watcher -> model
	l.watcher.OnUpdate(func(procs []pm2.Process) {
		l.processModel.Update(procs)
	})

	// Wire watcher errors -> flash
	l.watcher.OnError(func(err error) {
		l.app.QueueUpdateDraw(func() {
			l.Flash(ui.FlashError, fmt.Sprintf("PM2: %v", err))
		})
	})

	// Wire model -> view
	l.processModel.OnChange(func(procs []pm2.Process) {
		l.processView.UpdateProcesses(procs)
	})

	// Add flash widget to process view layout
	l.processView.Layout().AddItem(l.flashWidget, 1, 0, false)

	// Setup pages
	l.app.Pages.AddPage("processes", l.processView.Layout(), true, true)
	l.app.Pages.AddPage("logs", l.logsView.Layout(), true, false)

	// Global key handler
	l.setupGlobalKeys()

	// Start watcher
	l.watcher.Start()
	defer l.watcher.Stop()

	return l.app.Run()
}

func (l *Layout) wireActions() {
	l.processView.SetOnViewLogs(func(proc pm2.Process) {
		l.logsView.Show(proc)
		l.app.Pages.SwitchToPage("logs")
		l.app.SetFocus(l.logsView.panel)
	})

	l.processView.SetOnRestart(func(proc pm2.Process) {
		go func() {
			if err := l.client.Restart(proc.Name); err != nil {
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashError, fmt.Sprintf("Restart failed: %v", err))
				})
				return
			}
			l.watcher.Refresh()
			l.app.QueueUpdateDraw(func() {
				l.Flash(ui.FlashInfo, fmt.Sprintf("Restarted %s", proc.Name))
			})
		}()
	})

	l.processView.SetOnStop(func(proc pm2.Process) {
		l.confirm(fmt.Sprintf("Stop %s?", proc.Name), func() {
			go func() {
				if err := l.client.Stop(proc.Name); err != nil {
					l.app.QueueUpdateDraw(func() {
						l.Flash(ui.FlashError, fmt.Sprintf("Stop failed: %v", err))
					})
					return
				}
				l.watcher.Refresh()
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashInfo, fmt.Sprintf("Stopped %s", proc.Name))
				})
			}()
		})
	})

	l.processView.SetOnStart(func(proc pm2.Process) {
		if proc.PM2Env.Status == pm2.StatusOnline {
			return
		}
		go func() {
			if err := l.client.Start(proc.Name); err != nil {
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashError, fmt.Sprintf("Start failed: %v", err))
				})
				return
			}
			l.watcher.Refresh()
			l.app.QueueUpdateDraw(func() {
				l.Flash(ui.FlashInfo, fmt.Sprintf("Started %s", proc.Name))
			})
		}()
	})

	l.processView.SetOnDelete(func(proc pm2.Process) {
		l.confirm(fmt.Sprintf("Delete %s?", proc.Name), func() {
			go func() {
				if err := l.client.Delete(proc.Name); err != nil {
					l.app.QueueUpdateDraw(func() {
						l.Flash(ui.FlashError, fmt.Sprintf("Delete failed: %v", err))
					})
					return
				}
				l.watcher.Refresh()
				l.app.QueueUpdateDraw(func() {
					l.Flash(ui.FlashInfo, fmt.Sprintf("Deleted %s", proc.Name))
				})
			}()
		})
	})
}

func (l *Layout) setupGlobalKeys() {
	l.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case '?':
				l.showHelp()
				return nil
			}
		case tcell.KeyEsc:
			name, _ := l.app.Pages.GetFrontPage()
			switch name {
			case "logs":
				l.logsView.Stop()
				l.app.Pages.SwitchToPage("processes")
				l.app.SetFocus(l.processView.table)
				return nil
			case "help":
				l.app.Pages.RemovePage("help")
				l.app.SetFocus(l.processView.table)
				return nil
			case "confirm":
				l.app.Pages.RemovePage("confirm")
				l.app.SetFocus(l.processView.table)
				return nil
			case "processes":
				// Clear any active filter when Esc is pressed on the processes page
				// (handles the case where the filter bar is visible but not in edit mode).
				if l.processModel.Filter() != "" {
					l.processView.ClearFilter()
					return nil
				}
			}
		}
		return event
	})
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
		l.app.SetFocus(l.processView.table)
	})
	l.app.Pages.AddPage("confirm", modal, true, true)
}

// Flash shows a flash message.
func (l *Layout) Flash(level ui.FlashLevel, msg string) {
	l.flashWidget.Show(level, msg)
}
