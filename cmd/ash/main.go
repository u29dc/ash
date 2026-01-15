package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"

	"ash/internal/app"
	"ash/internal/cleaner/modules"
	"ash/internal/scanner"
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
		case "-n", "--dry-run", "dry-run":
			runDryRun()
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
  ash --dry-run    Scan and print report (no TTY required)
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

func runDryRun() {
	ctx := context.Background()

	// Create module registry
	registry, err := modules.NewRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	registry.EnableAll()

	// Scan all modules
	var allEntries []scanner.Entry
	for _, mod := range registry.EnabledModules() {
		entries, scanErr := mod.Scan(ctx)
		if scanErr != nil {
			continue // Skip modules with errors
		}
		allEntries = append(allEntries, entries...)
	}

	// Group entries by category
	byCategory := make(map[scanner.Category][]scanner.Entry)
	for _, entry := range allEntries {
		byCategory[entry.Category] = append(byCategory[entry.Category], entry)
	}

	// Get home directory for path shortening
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	// Print report header
	fmt.Println("ash - dry run report")
	fmt.Println()

	// Category display order
	categoryOrder := []scanner.Category{
		scanner.CategoryCaches,
		scanner.CategoryLogs,
		scanner.CategoryXcode,
		scanner.CategoryHomebrew,
		scanner.CategoryBrowsers,
		scanner.CategoryAppData,
		scanner.CategoryOther,
	}

	var totalSize int64
	var totalCount int

	for _, cat := range categoryOrder {
		entries, ok := byCategory[cat]
		if !ok || len(entries) == 0 {
			continue
		}
		size, count := printCategoryReport(entries, cat, homeDir)
		totalSize += size
		totalCount += count
	}

	// Print summary
	fmt.Printf("Summary: %d items, %s total\n", totalCount, humanize.IBytes(uint64(totalSize)))
}

func printCategoryReport(entries []scanner.Entry, cat scanner.Category, homeDir string) (int64, int) {
	// Sort entries by size (largest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	// Calculate category totals
	var catSize int64
	for _, e := range entries {
		catSize += e.Size
	}

	// Print category header
	catName := formatCategoryName(cat)
	fmt.Printf("%s (%d items, %s)\n", catName, len(entries), humanize.IBytes(uint64(catSize)))

	// Print entries (limit to top 20 per category)
	displayCount := len(entries)
	if displayCount > 20 {
		displayCount = 20
	}

	for i := 0; i < displayCount; i++ {
		e := entries[i]
		path := e.Path
		if homeDir != "" && strings.HasPrefix(path, homeDir) {
			path = "~" + path[len(homeDir):]
		}
		fmt.Printf("  %-60s %10s\n", truncatePath(path, 60), humanize.IBytes(uint64(e.Size)))
	}

	if len(entries) > 20 {
		fmt.Printf("  ... and %d more items\n", len(entries)-20)
	}
	fmt.Println()

	return catSize, len(entries)
}

func formatCategoryName(cat scanner.Category) string {
	switch cat {
	case scanner.CategoryCaches:
		return "Caches"
	case scanner.CategoryLogs:
		return "Logs"
	case scanner.CategoryXcode:
		return "Xcode"
	case scanner.CategoryHomebrew:
		return "Homebrew"
	case scanner.CategoryBrowsers:
		return "Browsers"
	case scanner.CategoryAppData:
		return "App Data"
	case scanner.CategoryOther:
		return "Other"
	default:
		return string(cat)
	}
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return path[:maxLen-3] + "..."
}
