package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/DillonBarker/pm2ui/internal/pm2"
)

// StatusBar displays the k9s-style header with logo, info, and key hints.
type StatusBar struct {
	*tview.Flex
	info     *tview.TextView
	hints    *tview.TextView
	hintList []KeyHint
	dimmed   bool
}

// NewStatusBar creates a new status bar header.
func NewStatusBar() *StatusBar {
	info := tview.NewTextView()
	info.SetDynamicColors(true)
	info.SetBackgroundColor(tcell.ColorDefault)
	info.SetTextColor(tcell.ColorDefault)
	info.SetTextAlign(tview.AlignLeft)

	hints := tview.NewTextView()
	hints.SetDynamicColors(true)
	hints.SetBackgroundColor(tcell.ColorDefault)
	hints.SetTextColor(tcell.ColorDefault)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(info, 1, 0, false).
		AddItem(hints, 1, 0, false)

	return &StatusBar{
		Flex:  flex,
		info:  info,
		hints: hints,
	}
}

// Update refreshes the status bar with current process counts.
// filtered is the currently visible (post-filter) list; total is the
// unfiltered count; ns is the active namespace scope ("" = all).
func (s *StatusBar) Update(filtered []pm2.Process, total int, filter, ns string) {
	var online, errored, stopped int
	for _, p := range filtered {
		switch p.PM2Env.Status {
		case pm2.StatusOnline:
			online++
		case pm2.StatusErrored:
			errored++
		case pm2.StatusStopped:
			stopped++
		}
	}

	var processCount string
	if filter != "" {
		processCount = fmt.Sprintf("[::b]%d/%d[-::-]", len(filtered), total)
	} else {
		processCount = fmt.Sprintf("[::b]%d[-::-]", total)
	}
	text := fmt.Sprintf("Processes: %s  [green::b]Online: %d[-::-]  [red::b]Errored: %d[-::-]  [gray]Stopped: %d[-]",
		processCount, online, errored, stopped)

	if ns != "" {
		text += fmt.Sprintf("  ns: [aqua]%s[-]", ns)
	}

	if filter != "" {
		text += fmt.Sprintf("  Filter: [aqua]%s[-]", filter)
	}

	s.info.SetText(text)
}

// SetKeyHints sets the key hints displayed in the header.
func (s *StatusBar) SetKeyHints(hintList []KeyHint) {
	s.hintList = hintList
	s.renderHints()
}

// SetDimmed grays out the hints when the pane doesn't own the keyboard.
func (s *StatusBar) SetDimmed(dimmed bool) {
	if s.dimmed == dimmed {
		return
	}
	s.dimmed = dimmed
	s.renderHints()
}

func (s *StatusBar) renderHints() {
	keyColor, actionColor := "aqua", "white"
	if s.dimmed {
		keyColor, actionColor = "gray", "gray"
	}
	var parts []string
	for _, h := range s.hintList {
		parts = append(parts, fmt.Sprintf("[%s::b]<%s>[-::-] [%s]%s[-]", keyColor, h.Key, actionColor, h.Action))
	}
	s.hints.SetText(" " + strings.Join(parts, "    "))
}
