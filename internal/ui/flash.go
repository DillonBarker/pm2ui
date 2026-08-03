package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// FlashLevel indicates the type of flash message.
type FlashLevel int

const (
	FlashInfo FlashLevel = iota
	FlashWarn
	FlashError
)

// FlashWidget displays temporary messages at the bottom of the screen.
type FlashWidget struct {
	*tview.TextView
	app *tview.Application
	// gen invalidates pending clear timers when a newer message arrives.
	// Only touched on the event loop.
	gen int
}

// NewFlashWidget creates a new flash message widget.
func NewFlashWidget() *FlashWidget {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetTextColor(tcell.ColorDefault)

	return &FlashWidget{TextView: tv}
}

// SetApp sets the tview application for QueueUpdateDraw.
func (f *FlashWidget) SetApp(app *tview.Application) {
	f.app = app
}

// Show displays a flash message that auto-clears after 3 seconds. Must be
// called from the event loop.
func (f *FlashWidget) Show(level FlashLevel, msg string) {
	color := "green"
	switch level {
	case FlashWarn:
		color = "yellow"
	case FlashError:
		color = "red"
	}
	f.SetText(fmt.Sprintf("[%s]%s[-]", color, msg))

	f.gen++
	gen := f.gen
	go func() {
		time.Sleep(3 * time.Second)
		if f.app != nil {
			f.app.QueueUpdateDraw(func() {
				if gen == f.gen { // don't clear a newer message
					f.SetText("")
				}
			})
		}
	}()
}
