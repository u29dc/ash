package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ash/internal/cleaner"
	"ash/internal/cleaner/modules"
	"ash/internal/maintenance"
	"ash/internal/scanner"
)

// StartScan returns a command that starts the scanning process.
func StartScan(ctx context.Context, categories []scanner.Category) tea.Cmd {
	return func() tea.Msg {
		s, err := scanner.New()
		if err != nil {
			return ScanErrorMsg{Err: err}
		}

		opts := scanner.ScanOptions{
			Categories:    categories,
			IncludeHidden: false,
			Parallelism:   4,
		}

		result, err := s.ScanAll(ctx, opts)
		if err != nil {
			return ScanErrorMsg{Err: err}
		}

		return ScanCompleteMsg{
			Entries:   result.Entries,
			TotalSize: result.TotalSize,
			Duration:  result.Duration.Seconds(),
		}
	}
}

// StartModuleScan returns a command that scans using cleanup modules.
func StartModuleScan(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		registry, err := modules.NewRegistry()
		if err != nil {
			return ScanErrorMsg{Err: err}
		}

		registry.EnableAll()

		var allEntries []scanner.Entry
		var totalSize int64

		for _, mod := range registry.EnabledModules() {
			entries, err := mod.Scan(ctx)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				allEntries = append(allEntries, entry)
				totalSize += entry.Size
			}
		}

		return ScanCompleteMsg{
			Entries:   allEntries,
			TotalSize: totalSize,
		}
	}
}

// StartClean returns a command that starts the cleaning process.
func StartClean(ctx context.Context, entries []scanner.Entry, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		c := cleaner.New(
			cleaner.WithDryRun(dryRun),
			cleaner.WithTrash(true),
		)

		stats, err := c.Clean(ctx, entries)
		if err != nil {
			return CleanErrorMsg{Err: err}
		}

		return CleanCompleteMsg{Stats: stats}
	}
}

// RunMaintenanceCommand returns a command that runs a maintenance operation.
func RunMaintenanceCommand(ctx context.Context, cmd *maintenance.Command) tea.Cmd {
	return func() tea.Msg {
		result := maintenance.Run(ctx, cmd)
		return MaintenanceCompleteMsg{Result: result}
	}
}

// Tick returns a command that sends a tick message after the given duration.
func Tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

// DoNothing returns an empty command.
func DoNothing() tea.Cmd {
	return nil
}
