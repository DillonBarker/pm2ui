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
  i              Describe process (live details, paths, versions)
  Space          Toggle multi-select (filters logs, enables bulk actions)
  u              Start stopped process(es)
  r              Restart process(es) (--update-env)
  s              Stop process(es) (confirms)
  d              Delete process(es) (confirms)

  With a Space-selection active, u/r/s/d act on ALL selected services.

[yellow]Sorting (Process Table)[-]
  Shift+N        Sort by name
  Shift+S        Sort by status
  Shift+P        Sort by PID
  Shift+C        Sort by CPU
  Shift+M        Sort by memory
  Shift+R        Sort by restarts
  Shift+U        Sort by uptime

[yellow]Log Panel (right side)[-]
  /              Search logs (case-insensitive regex, logs focused)
  m              Insert a timestamped mark line
  c              Clear panel (keeps tailing, search and selection)
  t              Toggle stdout/stderr/both
  a              Toggle autoscroll
  w              Toggle word wrap
  p              Pause/resume log tailing
  T              Toggle arrival timestamps
  f              Fullscreen logs
  S              Save log buffer to file (logs focused)
  ↑/PgUp/k/g     Scroll up (pauses tailing); G/End resumes
  Esc            Back to process list (leaves single-service view)

[yellow]Commands (:) — works from any pane, Tab completes[-]
  :ns <name>     Scope table to a pm2 namespace (:ns clears)
  :restart all   Restart all processes
  :stop all      Stop all processes (confirms)
  :reload all    Graceful reload (cluster mode)
  :save          Persist process list to disk
  :flush         Clear all log files
  :q / :q!       Quit

[yellow]Log History[-]
  0              Tail (live from end)
  1              Head (from start of file)
  2 / 3          Last 50 / 100 lines
  4 / 5 / 6      Last 200 / 500 / 1000 lines

[yellow]General[-]
  l              Focus logs (aqua border = focused pane)
  Esc            Back to process list
  ?              Toggle this help
  Mouse          Click to focus/select; wheel-up pauses tailing
                 (disable with mouseEnabled: false in config)

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
