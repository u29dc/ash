package app

import (
	"ash/internal/cleaner"
	"ash/internal/maintenance"
	"ash/internal/safety"
	"ash/internal/scanner"
)

// ScanStatus indicates whether a scan completed fully or partially.
type ScanStatus string

const (
	ScanStatusComplete ScanStatus = "complete"
	ScanStatusPartial  ScanStatus = "partial"
	ScanStatusFailed   ScanStatus = "failed"
)

// ScanIssue describes a warning or module failure encountered during scanning.
type ScanIssue struct {
	Source  string
	Message string
}

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
	Entries        []scanner.Entry
	TotalSize      int64
	Duration       float64
	Status         ScanStatus
	Issues         []ScanIssue
	FullDiskAccess safety.PermissionStatus
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

// AuthSuccessMsg indicates authorization succeeded.
type AuthSuccessMsg struct{}

// AuthErrorMsg indicates authorization failed.
type AuthErrorMsg struct {
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
