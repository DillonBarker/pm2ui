package cmd

import (
	"fmt"
	"os"

	"github.com/DillonBarker/pm2ui/internal/view"
)

var Version = "dev"

// Execute is the entry point for the application.
func Execute() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" || arg == "-version" {
			fmt.Printf("pm2ui %s\n", Version)
			return
		}
	}

	layout := view.NewLayout()
	if err := layout.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
