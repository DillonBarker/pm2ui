package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DefaultMaxLogLines is the default cap on buffered log lines.
const DefaultMaxLogLines = 5000

// LogEntry is a single log line in display (tview-tagged) and plain form.
// Process is the originating service name ("" in single-service mode).
// Mark entries are user-inserted separators that bypass all filters.
type LogEntry struct {
	Display string
	Plain   string
	Process string
	Mark    bool
}

// LogPanel is a scrollable log viewer with autoscroll support and a bounded
// ring buffer of the lines it has shown.
type LogPanel struct {
	*tview.TextView
	autoScroll bool
	wordWrap   bool
	paused     bool
	lineCount  int
	maxLines   int
	buffer     []LogEntry
	search     *regexp.Regexp
	matchCount int
	// svcFilter limits rendering to these services (nil = all). Lines keep
	// buffering for every service so changing the filter never loses data.
	svcFilter map[string]bool
	indicator *tview.TextView
}

// NewLogPanel creates a new log panel widget.
func NewLogPanel() *LogPanel {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)
	tv.SetWordWrap(true)
	tv.SetMaxLines(DefaultMaxLogLines)
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
		maxLines:   DefaultMaxLogLines,
		indicator:  indicator,
	}

	// No SetChangedFunc here: tview invokes it on a fresh goroutine, racing
	// the event loop. All writes go through AppendLines/rerender (event loop
	// only), which call afterContentChange synchronously instead.

	// Wheel-up pauses tailing, same rule as keyboard scrolling.
	tv.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseScrollUp {
			lp.setAutoScroll(false)
		}
		return action, event
	})

	// Scrolling up pauses tailing; jumping to the end resumes it.
	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyPgUp, tcell.KeyHome, tcell.KeyCtrlB:
			lp.setAutoScroll(false)
		case tcell.KeyEnd:
			lp.setAutoScroll(true)
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k', 'g':
				lp.setAutoScroll(false)
			case 'G':
				lp.setAutoScroll(true)
			}
		}
		return event
	})

	lp.updateIndicator()
	return lp
}

// Indicator returns the status indicator widget.
func (lp *LogPanel) Indicator() *tview.TextView {
	return lp.indicator
}

// SetMaxLines bounds both the ring buffer and the rendered text.
func (lp *LogPanel) SetMaxLines(n int) {
	if n <= 0 {
		n = DefaultMaxLogLines
	}
	lp.maxLines = n
	lp.TextView.SetMaxLines(n)
}

// AppendLines adds a batch of lines in a single text write. While a search
// is active only matching lines are rendered.
func (lp *LogPanel) AppendLines(entries []LogEntry) {
	if len(entries) == 0 {
		return
	}
	lp.buffer = append(lp.buffer, entries...)
	lp.enforceQuota()
	lp.lineCount += len(entries)

	var b strings.Builder
	for _, e := range entries {
		lp.renderEntry(&b, e)
	}
	if b.Len() > 0 {
		fmt.Fprint(lp.TextView, b.String())
	}
	lp.afterContentChange()
}

// enforceQuota trims the buffer to maxLines using a per-service fair share:
// only services holding more than maxLines/services lose their oldest lines,
// so a noisy neighbor can't evict a quiet service's history. With a single
// service this degrades to plain keep-newest.
func (lp *LogPanel) enforceQuota() {
	over := len(lp.buffer) - lp.maxLines
	if over <= 0 {
		return
	}
	counts := make(map[string]int, 8)
	for _, e := range lp.buffer {
		counts[e.Process]++
	}
	quota := max(1, lp.maxLines/len(counts))
	kept := make([]LogEntry, 0, lp.maxLines)
	for _, e := range lp.buffer {
		if over > 0 && counts[e.Process] > quota {
			counts[e.Process]--
			over--
			continue
		}
		kept = append(kept, e)
	}
	lp.buffer = kept
}

// afterContentChange applies autoscroll and refreshes the indicator. Must be
// called on the event loop after any content write.
func (lp *LogPanel) afterContentChange() {
	if lp.autoScroll {
		lp.ScrollToEnd()
	}
	lp.updateIndicator()
}

// SetServiceFilter re-renders showing only the given services; nil or empty
// restores all. The buffer is untouched, so this is instant and lossless.
func (lp *LogPanel) SetServiceFilter(names []string) {
	if len(names) == 0 {
		if lp.svcFilter == nil {
			return
		}
		lp.svcFilter = nil
	} else {
		lp.svcFilter = make(map[string]bool, len(names))
		for _, n := range names {
			lp.svcFilter[n] = true
		}
	}
	lp.rerender()
}

// renderEntry writes one entry to b, honoring the service filter and the
// active search. Mark separators always render.
func (lp *LogPanel) renderEntry(b *strings.Builder, e LogEntry) {
	if e.Mark {
		b.WriteString(e.Display)
		b.WriteByte('\n')
		return
	}
	if lp.svcFilter != nil && !lp.svcFilter[e.Process] {
		return
	}
	if lp.search != nil {
		if !lp.search.MatchString(e.Plain) {
			return
		}
		lp.matchCount++
		b.WriteString(highlightMatches(e.Plain, lp.search))
	} else {
		b.WriteString(e.Display)
	}
	b.WriteByte('\n')
}

// SetSearch filters the panel to lines matching re, highlighting matches.
func (lp *LogPanel) SetSearch(re *regexp.Regexp) {
	lp.search = re
	lp.rerender()
}

// ClearSearch removes the search filter and restores the full buffer.
func (lp *LogPanel) ClearSearch() {
	if lp.search == nil {
		return
	}
	lp.search = nil
	lp.rerender()
}

// HasSearch reports whether a search filter is active.
func (lp *LogPanel) HasSearch() bool {
	return lp.search != nil
}

// rerender rebuilds the text view from the buffer.
func (lp *LogPanel) rerender() {
	lp.TextView.Clear()
	lp.matchCount = 0
	var b strings.Builder
	for _, e := range lp.buffer {
		lp.renderEntry(&b, e)
	}
	if b.Len() > 0 {
		fmt.Fprint(lp.TextView, b.String())
	}
	lp.afterContentChange()
}

// highlightMatches renders plain with match regions highlighted, escaping
// everything else so tview doesn't interpret log content as color tags.
func highlightMatches(plain string, re *regexp.Regexp) string {
	var b strings.Builder
	last := 0
	for _, loc := range re.FindAllStringIndex(plain, -1) {
		b.WriteString(tview.Escape(plain[last:loc[0]]))
		b.WriteString("[black:yellow]")
		b.WriteString(tview.Escape(plain[loc[0]:loc[1]]))
		b.WriteString("[-:-]")
		last = loc[1]
	}
	b.WriteString(tview.Escape(plain[last:]))
	return b.String()
}

// AppendLine adds a single line to the log panel.
func (lp *LogPanel) AppendLine(text string) {
	lp.AppendLines([]LogEntry{{Display: text, Plain: text}})
}

// Buffer returns the buffered entries (shared slice; treat as read-only).
func (lp *LogPanel) Buffer() []LogEntry {
	return lp.buffer
}

// ToggleAutoScroll toggles auto-scroll.
func (lp *LogPanel) ToggleAutoScroll() {
	lp.setAutoScroll(!lp.autoScroll)
}

func (lp *LogPanel) setAutoScroll(on bool) {
	lp.autoScroll = on
	if on {
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

// SetPausedIndicator shows or hides the PAUSED marker.
func (lp *LogPanel) SetPausedIndicator(on bool) {
	lp.paused = on
	lp.updateIndicator()
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

	pausedStr := ""
	if lp.paused {
		pausedStr = "[black:yellow:b] PAUSED [-:-:-]   "
	}

	searchStr := ""
	if lp.search != nil {
		searchStr = fmt.Sprintf("[white::b]search[-::-] [yellow]/%s/[-] %d hits    ", tview.Escape(lp.search.String()), lp.matchCount)
	}

	lp.indicator.SetText(fmt.Sprintf(
		"  %s%s[white::b]scroll[-::-] %s    [white::b]wrap[-::-] %s    [darkgray]lines: %d[-]",
		pausedStr, searchStr, autoStr, wrapStr, lp.lineCount,
	))
}

// ClearContent empties the log content but keeps search, service filter and
// scroll/wrap preferences — the k9s `c` behavior.
func (lp *LogPanel) ClearContent() {
	lp.Clear()
	lp.buffer = nil
	lp.lineCount = 0
	lp.matchCount = 0
	lp.updateIndicator()
}

// Reset clears the log content, search and service filter but preserves
// scroll/wrap preferences.
func (lp *LogPanel) Reset() {
	lp.Clear()
	lp.buffer = nil
	lp.lineCount = 0
	lp.search = nil
	lp.matchCount = 0
	lp.svcFilter = nil
	lp.updateIndicator()
}
