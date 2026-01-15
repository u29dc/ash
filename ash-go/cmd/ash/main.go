package main

import (
	"fmt"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"ash/internal/app"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	// Check if running on macOS
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "ash is designed for macOS only")
		os.Exit(1)
	}

	// Handle flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			printVersion()
			return
		case "-h", "--help", "help":
			printHelp()
			return
		}
	}

	// Start the TUI
	model := app.New()
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("ash %s\n", version)
	fmt.Printf("commit: %s\n", commit)
	fmt.Printf("built: %s\n", buildTime)
}

func printHelp() {
	help := `ash - macOS cleanup utility

Usage:
  ash              Start the interactive TUI
  ash --version    Show version information
  ash --help       Show this help message

Description:
  ash is a fast, safe macOS cleanup tool that helps you remove:
  - Application caches
  - Log files
  - Xcode derived data and device support
  - Homebrew cache
  - Browser caches

  All deletions are moved to Trash for safety.

Controls:
  j/k or arrows    Navigate
  space            Toggle selection
  a                Select all
  enter            Confirm action
  esc              Go back
  q                Quit

For more information, visit: https://github.com/u29dc/ash
`
	fmt.Print(help)
}
