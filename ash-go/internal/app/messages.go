package app

import (
	"ash/internal/cleaner"
	"ash/internal/maintenance"
	"ash/internal/scanner"
)

// ScanStartedMsg indicates scanning has started.
type ScanStartedMsg struct{}

// ScanProgressMsg reports scanning progress.
type ScanProgressMsg struct {
	Progress float64
	Message  string
	Current  string
}

// ScanCompleteMsg indicates scanning has completed.
type ScanCompleteMsg struct {
	Entries   []scanner.Entry
	TotalSize int64
	Duration  float64
}

// ScanErrorMsg indicates a scan error.
type ScanErrorMsg struct {
	Err error
}

// CleanStartedMsg indicates cleaning has started.
type CleanStartedMsg struct {
	Count int
	Size  int64
}

// CleanProgressMsg reports cleaning progress.
type CleanProgressMsg struct {
	Current int
	Total   int
	Path    string
}

// CleanCompleteMsg indicates cleaning has completed.
type CleanCompleteMsg struct {
	Stats *cleaner.CleanStats
}

// CleanErrorMsg indicates a clean error.
type CleanErrorMsg struct {
	Err error
}

// MaintenanceStartedMsg indicates a maintenance command has started.
type MaintenanceStartedMsg struct {
	Command *maintenance.Command
}

// MaintenanceCompleteMsg indicates a maintenance command has completed.
type MaintenanceCompleteMsg struct {
	Result *maintenance.CommandResult
}

// ErrorMsg represents a general error message.
type ErrorMsg struct {
	Err error
}

// TickMsg is sent periodically for animations.
type TickMsg struct{}

// SelectionChangedMsg indicates the selection has changed.
type SelectionChangedMsg struct {
	Selected map[string]bool
	Count    int
	Size     int64
}

// ViewChangedMsg indicates the view has changed.
type ViewChangedMsg struct {
	View View
}

// QuitMsg indicates the application should quit.
type QuitMsg struct{}
