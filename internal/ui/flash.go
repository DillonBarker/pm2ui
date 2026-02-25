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
	FlashError
)

// FlashWidget displays temporary messages at the bottom of the screen.
type FlashWidget struct {
	*tview.TextView
	app *tview.Application
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

// Show displays a flash message that auto-clears after 3 seconds.
func (f *FlashWidget) Show(level FlashLevel, msg string) {
	color := "green"
	if level == FlashError {
		color = "red"
	}
	f.SetText(fmt.Sprintf("[%s]%s[-]", color, msg))

	go func() {
		time.Sleep(3 * time.Second)
		if f.app != nil {
			f.app.QueueUpdateDraw(func() {
				f.SetText("")
			})
		}
	}()
}
