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
  Esc            Back to all logs / clear filter

[yellow]Process Actions[-]
  Enter          View logs for selected service
  Space          Toggle multi-select (watch subset of services)
  u              Start stopped process
  r              Restart process (--update-env)
  s              Stop process (confirms)
  d              Delete process (confirms)

[yellow]Sorting (Process Table)[-]
  Shift+N        Sort by name
  Shift+S        Sort by status
  Shift+P        Sort by PID
  Shift+U        Sort by uptime

[yellow]Log Panel (right side)[-]
  t              Toggle stdout/stderr/both (single-service mode)
  a              Toggle autoscroll
  w              Toggle word wrap
  Esc            Return to all-services / multi-select logs

[yellow]Commands (:)[-]
  :restart all   Restart all processes
  :stop all      Stop all processes (confirms)
  :reload all    Graceful reload (cluster mode)
  :save          Persist process list to disk
  :flush         Clear all log files
  :q!            Quit

[yellow]Log History[-]
  0              Tail (live from end)
  1              Head (from start of file)
  2 / 3          Last 50 / 100 lines
  4 / 5 / 6      Last 200 / 500 / 1000 lines

[yellow]General[-]
  l              Focus logs (scroll)
  Esc            Back to process list
  ?              Toggle this help

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
