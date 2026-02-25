package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// LogPanel is a scrollable log viewer with autoscroll support.
type LogPanel struct {
	*tview.TextView
	autoScroll bool
	wordWrap   bool
	lineCount  int
	indicator  *tview.TextView
}

// NewLogPanel creates a new log panel widget.
func NewLogPanel() *LogPanel {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)
	tv.SetWordWrap(true)
	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetTextColor(tcell.ColorDefault)

	indicator := tview.NewTextView()
	indicator.SetDynamicColors(true)
	indicator.SetBackgroundColor(tcell.ColorDefault)
	indicator.SetTextAlign(tview.AlignLeft)

	lp := &LogPanel{
		TextView:   tv,
		autoScroll: true,
		wordWrap:   true,
		indicator:  indicator,
	}

	tv.SetChangedFunc(func() {
		if lp.autoScroll {
			tv.ScrollToEnd()
		}
		lp.updateIndicator()
	})

	lp.updateIndicator()
	return lp
}

// Indicator returns the status indicator widget.
func (lp *LogPanel) Indicator() *tview.TextView {
	return lp.indicator
}

// AppendLine adds a line to the log panel.
func (lp *LogPanel) AppendLine(text string) {
	lp.lineCount++
	fmt.Fprintln(lp.TextView, text)
}

// ToggleAutoScroll toggles auto-scroll.
func (lp *LogPanel) ToggleAutoScroll() {
	lp.autoScroll = !lp.autoScroll
	if lp.autoScroll {
		lp.ScrollToEnd()
	}
	lp.updateIndicator()
}

// ToggleWordWrap toggles word wrap.
func (lp *LogPanel) ToggleWordWrap() {
	lp.wordWrap = !lp.wordWrap
	lp.SetWrap(lp.wordWrap)
	lp.SetWordWrap(lp.wordWrap)
	lp.updateIndicator()
}

// AutoScroll returns whether auto-scroll is enabled.
func (lp *LogPanel) AutoScroll() bool {
	return lp.autoScroll
}

// WordWrap returns whether word wrap is enabled.
func (lp *LogPanel) WordWrap() bool {
	return lp.wordWrap
}

func (lp *LogPanel) updateIndicator() {
	on := "[green::b]on[-::-]"
	off := "[red::b]off[-::-]"

	autoStr := off
	if lp.autoScroll {
		autoStr = on
	}
	wrapStr := off
	if lp.wordWrap {
		wrapStr = on
	}

	lp.indicator.SetText(fmt.Sprintf(
		"  [white::b]scroll[-::-] %s    [white::b]wrap[-::-] %s    [darkgray]lines: %d[-]",
		autoStr, wrapStr, lp.lineCount,
	))
}

// Reset clears the log panel.
func (lp *LogPanel) Reset() {
	lp.Clear()
	lp.lineCount = 0
	lp.autoScroll = true
	lp.wordWrap = true
	lp.SetWrap(true)
	lp.SetWordWrap(true)
	lp.updateIndicator()
}
