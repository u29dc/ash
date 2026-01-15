package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the grayscale color palette for the application.
type Theme struct {
	// Base colors - grayscale only
	Background   lipgloss.Color // #0a0a0a (near black)
	Surface      lipgloss.Color // #171717 (elevated surface)
	SurfaceHover lipgloss.Color // #262626 (hover state)
	Border       lipgloss.Color // #404040 (subtle borders)
	BorderFocus  lipgloss.Color // #737373 (focused borders)

	// Text hierarchy
	TextPrimary   lipgloss.Color // #fafafa (primary content)
	TextSecondary lipgloss.Color // #a3a3a3 (secondary content)
	TextMuted     lipgloss.Color // #525252 (disabled/hints)

	// Semantic colors (only accents)
	Success lipgloss.Color // #22c55e (green - only accent)
	Warning lipgloss.Color // #f59e0b (amber - only accent)
	Danger  lipgloss.Color // #ef4444 (red - only accent)

	// Selection
	Selected lipgloss.Color // #262626 (selected row bg)
	Cursor   lipgloss.Color // #fafafa (cursor indicator)
}

// DefaultTheme is the default grayscale theme for ash.
var DefaultTheme = Theme{
	Background:   lipgloss.Color("#0a0a0a"),
	Surface:      lipgloss.Color("#171717"),
	SurfaceHover: lipgloss.Color("#262626"),
	Border:       lipgloss.Color("#404040"),
	BorderFocus:  lipgloss.Color("#737373"),

	TextPrimary:   lipgloss.Color("#fafafa"),
	TextSecondary: lipgloss.Color("#a3a3a3"),
	TextMuted:     lipgloss.Color("#525252"),

	Success: lipgloss.Color("#22c55e"),
	Warning: lipgloss.Color("#f59e0b"),
	Danger:  lipgloss.Color("#ef4444"),

	Selected: lipgloss.Color("#262626"),
	Cursor:   lipgloss.Color("#fafafa"),
}
