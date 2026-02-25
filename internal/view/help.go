package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpText = `[::b]pm2ui — Key Bindings[-::-]

[yellow]Navigation[-]
  j / ↓          Move down
  k / ↑          Move up
  /              Filter by name
  Esc            Go back / clear filter

[yellow]Process Actions[-]
  l              View logs
  r              Restart process (--update-env)
  s              Stop process (confirms)
  Enter          Start stopped process
  d              Delete process (confirms)

[yellow]Sorting (Process Table)[-]
  Shift+N        Sort by name
  Shift+S        Sort by status
  Shift+P        Sort by PID
  Shift+U        Sort by uptime

[yellow]Log Viewer[-]
  t              Toggle stdout/stderr/both
  a              Toggle autoscroll
  w              Toggle word wrap

[yellow]Commands (:)[-]
  :restart all   Restart all processes
  :stop all      Stop all processes (confirms)
  :reload all    Graceful reload (cluster mode)
  :save          Persist process list to disk
  :flush         Clear all log files
  :q!            Quit

[yellow]General[-]
  ?              Toggle this help
  q              Quit

[gray]Press Esc or ? to close[-]`

func newHelpView() *tview.TextView {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetTextColor(tcell.ColorDefault)
	tv.SetBorderPadding(1, 1, 2, 2)
	tv.SetBorder(true)
	tv.SetBorderColor(tcell.ColorYellow)
	tv.SetTitle(" Help ")
	tv.SetTitleColor(tcell.ColorYellow)
	tv.SetText(helpText)
	return tv
}
