package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/DillonBarker/pm2ui/internal/model"
	"github.com/DillonBarker/pm2ui/internal/pm2"
	"github.com/DillonBarker/pm2ui/internal/ui"
)

var processColumns = []ui.Column{
	{Title: "NAME", Expansion: 2},
	{Title: "STATUS", Expansion: 1},
	{Title: "PID", Expansion: 1, AlignRight: true},
	{Title: "UPTIME", Expansion: 1, AlignRight: true},
}

// ProcessView displays the process table.
type ProcessView struct {
	table           *ui.Table
	tableFrame      *tview.Frame
	statusBar       *ui.StatusBar
	filterBar       *tview.InputField
	filterContainer *tview.Flex
	cmdBar          *tview.TextView
	cmdText         string // current command text being typed
	cmdContainer    *tview.Flex
	layout          *tview.Flex
	model           *model.ProcessTable
	app             *ui.App
	selected        string // preserve selection by name across refreshes
	Filtering       bool
	Commanding      bool

	// callbacks
	onViewLogs func(proc pm2.Process)
	onRestart  func(proc pm2.Process)
	onStop     func(proc pm2.Process)
	onStart    func(proc pm2.Process)
	onDelete   func(proc pm2.Process)
	onCommand  func(cmd string)
}

// NewProcessView creates a new process table view.
func NewProcessView(app *ui.App, m *model.ProcessTable) *ProcessView {
	table := ui.NewTable(processColumns)
	statusBar := ui.NewStatusBar()
	statusBar.SetKeyHints([]ui.KeyHint{
		{Key: "l", Action: "logs"},
		{Key: "r", Action: "restart"},
		{Key: "s", Action: "stop"},
		{Key: "Enter", Action: "start"},
		{Key: "d", Action: "delete"},
		{Key: "/", Action: "filter"},
		{Key: ":", Action: "command"},
		{Key: "?", Action: "help"},
	})

	filterBar := tview.NewInputField()
	filterBar.SetLabel(" Filter: ")
	filterBar.SetLabelColor(tcell.ColorAqua)
	filterBar.SetFieldBackgroundColor(tcell.ColorDefault)
	filterBar.SetFieldTextColor(tcell.ColorWhite)
	filterBar.SetBackgroundColor(tcell.ColorDefault)

	// Use a bordered Flex as the container so Focus() properly cascades to filterBar.
	filterContainer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(filterBar, 1, 0, true)
	filterContainer.SetBorder(true)
	filterContainer.SetBorderColor(tcell.ColorOrange)
	filterContainer.SetTitle(" Filter ")
	filterContainer.SetTitleColor(tcell.ColorOrange)
	filterContainer.SetBackgroundColor(tcell.ColorDefault)

	// Table in a bordered frame
	tableFrame := tview.NewFrame(table).
		SetBorders(0, 0, 0, 0, 1, 1)
	tableFrame.SetBorder(true)
	tableFrame.SetBorderColor(tcell.ColorDarkCyan)
	tableFrame.SetTitle(" Processes ")
	tableFrame.SetTitleColor(tcell.ColorAqua)
	tableFrame.SetBackgroundColor(tcell.ColorDefault)

	// Command bar (k9s-style :q! prompt)
	// Use TextView instead of InputField to avoid resize bugs
	cmdBar := tview.NewTextView()
	cmdBar.SetDynamicColors(true)
	cmdBar.SetBackgroundColor(tcell.ColorDefault)
	cmdBar.SetTextColor(tcell.ColorWhite)

	cmdContainer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(cmdBar, 1, 0, true)
	cmdContainer.SetBorder(true)
	cmdContainer.SetBorderColor(tcell.ColorMediumSpringGreen)
	cmdContainer.SetTitle(" Command ")
	cmdContainer.SetTitleColor(tcell.ColorMediumSpringGreen)
	cmdContainer.SetBackgroundColor(tcell.ColorDefault)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusBar, 2, 0, false).
		AddItem(filterContainer, 0, 0, false). // hidden by default; shown below status bar when filtering
		AddItem(cmdContainer, 0, 0, false).    // hidden by default; shown below status bar when commanding
		AddItem(tableFrame, 0, 1, true)

	pv := &ProcessView{
		table:           table,
		tableFrame:      tableFrame,
		statusBar:       statusBar,
		filterBar:       filterBar,
		filterContainer: filterContainer,
		cmdBar:          cmdBar,
		cmdContainer:    cmdContainer,
		layout:          layout,
		model:           m,
		app:             app,
	}

	pv.setupKeys()
	return pv
}

// Layout returns the root flex layout.
func (pv *ProcessView) Layout() *tview.Flex {
	return pv.layout
}

// SetOnViewLogs sets the callback for when user presses 'l'.
func (pv *ProcessView) SetOnViewLogs(fn func(pm2.Process)) {
	pv.onViewLogs = fn
}

// SetOnRestart sets the callback for restart action.
func (pv *ProcessView) SetOnRestart(fn func(pm2.Process)) {
	pv.onRestart = fn
}

// SetOnStop sets the callback for stop action.
func (pv *ProcessView) SetOnStop(fn func(pm2.Process)) {
	pv.onStop = fn
}

// SetOnStart sets the callback for start action.
func (pv *ProcessView) SetOnStart(fn func(pm2.Process)) {
	pv.onStart = fn
}

// SetOnDelete sets the callback for delete action.
func (pv *ProcessView) SetOnDelete(fn func(pm2.Process)) {
	pv.onDelete = fn
}

// UpdateProcesses updates the table with the given process list.
func (pv *ProcessView) UpdateProcesses(procs []pm2.Process) {
	pv.app.QueueUpdateDraw(func() {
		pv.renderTable(procs)
		pv.statusBar.Update(procs, pv.model.TotalCount(), pv.model.Filter())
	})
}

// ClearFilter clears the active filter and hides the filter bar.
// Safe to call from the event loop.
func (pv *ProcessView) ClearFilter() {
	pv.model.SetFilter("")
	pv.filterBar.SetText("")
	pv.refreshFromFilter()
	pv.stopFilter()
}

// refreshFromFilter re-renders the table and status bar using the current
// model state. Call this directly from the event loop instead of going through
// UpdateProcesses (which uses QueueUpdateDraw and would deadlock).
func (pv *ProcessView) refreshFromFilter() {
	filtered := pv.model.Processes()
	pv.renderTable(filtered)
	pv.statusBar.Update(filtered, pv.model.TotalCount(), pv.model.Filter())
}

func (pv *ProcessView) renderTable(procs []pm2.Process) {
	// Remember selection
	if row := pv.table.SelectedDataRow(); row >= 0 {
		cell := pv.table.GetCell(row+1, 0)
		if cell != nil {
			pv.selected = tview.TranslateANSI(cell.Text)
		}
	}

	pv.table.ClearRows()
	pv.tableFrame.SetTitle(fmt.Sprintf(" Processes[[white]%d[-]] ", len(procs)))

	if len(procs) == 0 {
		pv.table.SetRow(0, "[gray]No processes found[-]", "", "", "")
		return
	}

	// Update sort indicator
	col, asc := pv.model.SortInfo()
	pv.table.SetSortIndicator(int(col), asc)

	newSelectedRow := -1
	for i, p := range procs {
		statusColor := statusToColor(p.PM2Env.Status)
		pv.table.SetRow(i,
			p.Name,
			fmt.Sprintf("[%s]%s[-]", statusColor, p.PM2Env.Status),
			fmt.Sprintf("%d", p.PID),
			p.FormatUptime(),
		)
		if p.Name == pv.selected {
			newSelectedRow = i + 1 // +1 for header
		}
	}

	if newSelectedRow > 0 {
		pv.table.Select(newSelectedRow, 0)
	} else if pv.table.GetRowCount() > 1 {
		pv.table.Select(1, 0)
	}
}

func (pv *ProcessView) selectedProcess() (pm2.Process, bool) {
	row := pv.table.SelectedDataRow()
	procs := pv.model.Processes()
	if row < 0 || row >= len(procs) {
		return pm2.Process{}, false
	}
	return procs[row], true
}

func (pv *ProcessView) setupKeys() {
	pv.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				row, col := pv.table.GetSelection()
				if row < pv.table.GetRowCount()-1 {
					pv.table.Select(row+1, col)
				}
				return nil
			case 'k':
				row, col := pv.table.GetSelection()
				if row > 1 { // skip header
					pv.table.Select(row-1, col)
				}
				return nil
			case '/':
				pv.startFilter()
				return nil
			case ':':
				pv.startCmd()
				return nil
			case 'l':
				if p, ok := pv.selectedProcess(); ok && pv.onViewLogs != nil {
					pv.onViewLogs(p)
				}
				return nil
			case 'r':
				if p, ok := pv.selectedProcess(); ok && pv.onRestart != nil {
					pv.onRestart(p)
				}
				return nil
			case 's':
				if p, ok := pv.selectedProcess(); ok && pv.onStop != nil {
					pv.onStop(p)
				}
				return nil
			case 'd':
				if p, ok := pv.selectedProcess(); ok && pv.onDelete != nil {
					pv.onDelete(p)
				}
				return nil
			case 'N':
				pv.model.SetSort(model.SortByName)
				return nil
			case 'S':
				pv.model.SetSort(model.SortByStatus)
				return nil
			case 'P':
				pv.model.SetSort(model.SortByPID)
				return nil
			case 'U':
				pv.model.SetSort(model.SortByUptime)
				return nil
			}
		case tcell.KeyEnter:
			if p, ok := pv.selectedProcess(); ok && pv.onStart != nil {
				pv.onStart(p)
			}
			return nil
		}
		return event
	})

	// Handle filter bar keys explicitly via SetInputCapture so they fire
	// reliably regardless of tview's internal done/finish routing.
	pv.filterBar.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			pv.ClearFilter()
			return nil
		case tcell.KeyEnter:
			pv.stopFilter()
			return nil
		}
		return event
	})

	pv.filterBar.SetChangedFunc(func(text string) {
		pv.model.SetFilter(text)
		pv.refreshFromFilter()
	})

	pv.cmdBar.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			pv.stopCmd()
			return nil
		case tcell.KeyEnter:
			cmd := strings.TrimSpace(pv.cmdText)
			pv.stopCmd()
			if pv.onCommand != nil && cmd != "" {
				pv.onCommand(cmd)
			}
			return nil
		case tcell.KeyTab:
			if suffix := cmdHintSuffix(pv.cmdText); suffix != "" {
				pv.cmdText += suffix
				pv.updateCmdBar()
			}
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(pv.cmdText) > 0 {
				pv.cmdText = pv.cmdText[:len(pv.cmdText)-1]
				pv.updateCmdBar()
			}
			return nil
		case tcell.KeyRune:
			pv.cmdText += string(event.Rune())
			pv.updateCmdBar()
			return nil
		}
		return event
	})
}

func (pv *ProcessView) startFilter() {
	if pv.Filtering {
		return
	}
	pv.Filtering = true
	pv.layout.ResizeItem(pv.filterContainer, 3, 0) // 3 rows: border + 1 content + border
	pv.filterBar.SetText(pv.model.Filter())
	pv.app.SetFocus(pv.filterContainer)
}

func (pv *ProcessView) stopFilter() {
	pv.Filtering = false
	// Keep the bar visible while a filter is active; hide it when cleared.
	if pv.model.Filter() == "" {
		pv.layout.ResizeItem(pv.filterContainer, 0, 0)
	}
	pv.app.SetFocus(pv.table)
}

// knownCommands is the list of valid command-mode commands for hint/completion.
var knownCommands = []string{
	"flush",
	"q!",
	"reload all",
	"restart all",
	"save",
	"stop all",
}

// cmdHintSuffix returns the untyped suffix of the best matching command, or "".
func cmdHintSuffix(text string) string {
	if text == "" {
		return ""
	}
	for _, cmd := range knownCommands {
		if strings.HasPrefix(cmd, text) && cmd != text {
			return cmd[len(text):]
		}
	}
	return ""
}

// SetOnCommand registers a callback invoked when the user executes a command.
func (pv *ProcessView) SetOnCommand(fn func(string)) {
	pv.onCommand = fn
}

// updateCmdBar refreshes the command bar with current text and hint
func (pv *ProcessView) updateCmdBar() {
	pv.cmdBar.Clear()
	suffix := cmdHintSuffix(pv.cmdText)
	display := " : " + pv.cmdText
	if suffix != "" {
		display += fmt.Sprintf("[gray]%s[-]", suffix)
	}
	fmt.Fprint(pv.cmdBar, display)
}

func (pv *ProcessView) startCmd() {
	if pv.Commanding {
		return
	}
	pv.Commanding = true
	pv.cmdText = ""
	pv.updateCmdBar()
	pv.layout.ResizeItem(pv.cmdContainer, 3, 0)
	pv.app.SetFocus(pv.cmdContainer)
}

func (pv *ProcessView) stopCmd() {
	pv.Commanding = false
	pv.cmdText = ""
	pv.cmdBar.Clear()
	pv.layout.ResizeItem(pv.cmdContainer, 0, 0)
	pv.app.SetFocus(pv.table)
}

func statusToColor(status string) string {
	switch status {
	case pm2.StatusOnline:
		return "green"
	case pm2.StatusErrored:
		return "red"
	case pm2.StatusStopped:
		return "gray"
	case pm2.StatusStopping:
		return "yellow"
	case pm2.StatusLaunching:
		return "blue"
	default:
		return "white"
	}
}
