package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

var manualMode bool

func main() {
	// Parse flags
	flag.BoolVar(&manualMode, "m", false, "Manual mode - ask for confirmation at each step")
	flag.BoolVar(&manualMode, "manual", false, "Manual mode - ask for confirmation at each step")
	flag.Parse()

	// Check if we're in a git repo
	if !IsGitRepo() {
		fmt.Println("❌ Not a git repository. Please run this from inside a git repo.")
		os.Exit(1)
	}

	// Run the TUI
	p := tea.NewProgram(InitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
