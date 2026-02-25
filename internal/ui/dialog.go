package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ConfirmDialog creates a modal confirmation dialog.
func ConfirmDialog(message string, onConfirm, onCancel func()) *tview.Modal {
	modal := tview.NewModal()
	modal.SetText(message)
	modal.AddButtons([]string{"Cancel", "OK"})
	modal.SetBackgroundColor(tcell.ColorDefault)
	modal.SetButtonBackgroundColor(tcell.ColorDarkCyan)
	modal.SetButtonTextColor(tcell.ColorWhite)
	modal.SetTextColor(tcell.ColorWhite)

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "OK" && onConfirm != nil {
			onConfirm()
		}
		if onCancel != nil {
			onCancel()
		}
	})

	return modal
}
