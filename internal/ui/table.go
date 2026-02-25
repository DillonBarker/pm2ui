package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Column defines a table column.
type Column struct {
	Title     string
	Width     int
	Expansion int
	AlignRight bool
}

// Table is a reusable table widget with j/k navigation and sort indicators.
type Table struct {
	*tview.Table
	columns     []Column
	sortCol     int
	sortAsc     bool
	onSelect    func(row int)
}

// NewTable creates a new table widget.
func NewTable(columns []Column) *Table {
	t := &Table{
		Table:   tview.NewTable(),
		columns: columns,
		sortCol: -1,
		sortAsc: true,
	}

	t.SetSelectable(true, false)
	t.SetFixed(1, 0)
	t.SetBorders(false)
	t.SetSeparator(' ')
	t.SetBackgroundColor(tcell.ColorDefault)
	t.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorDarkCyan).
		Bold(true))

	t.renderHeaders()

	return t
}

// SetSortIndicator updates the sort indicator on headers.
func (t *Table) SetSortIndicator(col int, asc bool) {
	t.sortCol = col
	t.sortAsc = asc
	t.renderHeaders()
}

// SetOnSelect sets the callback for row selection (on Enter).
func (t *Table) SetOnSelect(fn func(row int)) {
	t.onSelect = fn
	t.SetSelectedFunc(func(row, _ int) {
		if row > 0 && fn != nil {
			fn(row - 1) // adjust for header
		}
	})
}

func (t *Table) renderHeaders() {
	for i, col := range t.columns {
		title := col.Title
		if i == t.sortCol {
			if t.sortAsc {
				title += " ▲"
			} else {
				title += " ▼"
			}
		}

		align := tview.AlignLeft
		if col.AlignRight {
			align = tview.AlignRight
		}

		cell := tview.NewTableCell(fmt.Sprintf("[::b]%s", title)).
			SetSelectable(false).
			SetAlign(align).
			SetExpansion(col.Expansion).
			SetMaxWidth(col.Width).
			SetTextColor(tcell.ColorYellow).
			SetBackgroundColor(tcell.ColorDefault)
		t.SetCell(0, i, cell)
	}
}

// SetRow sets the content of a data row (0-indexed, maps to table row+1).
func (t *Table) SetRow(row int, cells ...string) {
	tableRow := row + 1 // account for header
	for i, text := range cells {
		align := tview.AlignLeft
		if i < len(t.columns) && t.columns[i].AlignRight {
			align = tview.AlignRight
		}

		cell := tview.NewTableCell(text).
			SetAlign(align).
			SetExpansion(t.columns[i].Expansion).
			SetMaxWidth(0).
			SetBackgroundColor(tcell.ColorDefault)
		t.SetCell(tableRow, i, cell)
	}
}

// ClearRows removes all data rows (keeps headers).
func (t *Table) ClearRows() {
	rowCount := t.GetRowCount()
	for r := rowCount - 1; r >= 1; r-- {
		t.RemoveRow(r)
	}
}

// SelectedDataRow returns the currently selected data row index (0-based),
// or -1 if no data row is selected.
func (t *Table) SelectedDataRow() int {
	row, _ := t.GetSelection()
	if row < 1 {
		return -1
	}
	return row - 1
}
