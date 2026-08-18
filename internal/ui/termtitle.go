package ui

import (
	"fmt"
	"io"
	"os"
)

// TabTitle writes the terminal's *icon* title (OSC 1). tcell's Screen.SetTitle
// only writes the window title (OSC 2), and terminals that show a tab bar —
// iTerm2, Ghostty, tmux — label tabs from the icon title, so the window title
// alone leaves the tab showing the foreground job name ("./pm2ui").
//
// Writes go straight to the terminal rather than through tcell's buffer. That
// is safe as long as Set is called from the tview event loop (between draws),
// never from a poll goroutine.
type TabTitle struct {
	w       io.Writer
	enabled bool
	saved   bool
}

// NewTabTitle returns a writer targeting stdout. It is inert on terminals that
// can't be expected to understand OSC 1 ($TERM unset or "dumb").
func NewTabTitle() *TabTitle {
	term := os.Getenv("TERM")
	return &TabTitle{
		w:       os.Stdout,
		enabled: term != "" && term != "dumb",
	}
}

// Set writes title as the tab/icon title, saving the terminal's original one
// on first use so Restore can put it back.
func (t *TabTitle) Set(title string) {
	if !t.enabled {
		return
	}
	if !t.saved {
		t.saved = true
		fmt.Fprint(t.w, "\x1b[22;1t") // push icon title onto the terminal's stack
	}
	fmt.Fprintf(t.w, "\x1b]1;%s\x07", title)
}

// Restore returns the tab title to the terminal's control: it clears our title
// (terminals fall back to the job name) and pops the saved one.
func (t *TabTitle) Restore() {
	if !t.enabled || !t.saved {
		return
	}
	t.saved = false
	fmt.Fprint(t.w, "\x1b]1;\x07") // empty icon title: revert to default
	fmt.Fprint(t.w, "\x1b[23;1t")  // pop the saved icon title, if supported
}
