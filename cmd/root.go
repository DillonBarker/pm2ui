package cmd

import (
	"fmt"
	"os"

	"github.com/DillonBarker/pm2ui/internal/view"
)

// Execute is the entry point for the application.
func Execute() {
	layout := view.NewLayout()
	if err := layout.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
