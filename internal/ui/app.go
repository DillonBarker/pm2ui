package ui

import (
	"github.com/rivo/tview"
)

// App wraps tview.Application with the root layout.
type App struct {
	*tview.Application
	Pages *tview.Pages
	Root  *tview.Flex
}

// NewApp creates a new TUI application with a Pages container.
func NewApp() *App {
	app := tview.NewApplication()
	pages := tview.NewPages()

	a := &App{
		Application: app,
		Pages:       pages,
	}

	app.SetRoot(pages, true)
	app.EnableMouse(false)

	return a
}
