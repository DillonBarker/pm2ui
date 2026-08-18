package ui

import (
	"github.com/rivo/tview"
)

// App wraps tview.Application with the root layout.
type App struct {
	*tview.Application
	Pages    *tview.Pages
	Root     *tview.Flex
	tabTitle *TabTitle
}

// NewApp creates a new TUI application with a Pages container.
func NewApp(enableMouse bool) *App {
	app := tview.NewApplication()
	pages := tview.NewPages()

	a := &App{
		Application: app,
		Pages:       pages,
		tabTitle:    NewTabTitle(),
	}

	app.SetRoot(pages, true)
	app.EnableMouse(enableMouse)

	return a
}

// SetTerminalTitle sets both titles a terminal can show: the window title
// (via tcell) and the tab/icon title. Event-loop only.
func (a *App) SetTerminalTitle(title string) {
	a.SetTitle(title)
	a.tabTitle.Set(title)
}

// RestoreTerminalTitle hands the tab title back to the terminal. Call once on
// shutdown, after the screen is finalized.
func (a *App) RestoreTerminalTitle() {
	a.tabTitle.Restore()
}
