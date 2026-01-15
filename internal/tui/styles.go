package tui

import "github.com/charmbracelet/lipgloss"

// Styles contains all the lipgloss styles for the application.
type Styles struct {
	// Layout
	App     lipgloss.Style
	Header  lipgloss.Style
	Content lipgloss.Style
	Footer  lipgloss.Style

	// Components
	Title    lipgloss.Style
	Subtitle lipgloss.Style

	// List items
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style

	// File entries
	FileName lipgloss.Style
	FileSize lipgloss.Style
	FilePath lipgloss.Style

	// Status
	StatusBar   lipgloss.Style
	KeyBind     lipgloss.Style
	KeyBindDesc lipgloss.Style

	// Indicators
	Spinner          lipgloss.Style
	Progress         lipgloss.Style
	Checkbox         lipgloss.Style
	CheckboxSelected lipgloss.Style

	// Categories
	CategorySafe    lipgloss.Style
	CategoryCaution lipgloss.Style
	CategoryDanger  lipgloss.Style

	// Dialogs
	Dialog       lipgloss.Style
	DialogTitle  lipgloss.Style
	DialogButton lipgloss.Style
}

// NewStyles creates a new Styles instance with the given theme.
func NewStyles(t Theme) Styles {
	return Styles{
		App: lipgloss.NewStyle().
			Background(t.Background).
			Foreground(t.TextPrimary),

		Header: lipgloss.NewStyle().
			Padding(1, 2).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(t.Border),

		Content: lipgloss.NewStyle().
			Padding(1, 2),

		Footer: lipgloss.NewStyle().
			Padding(0, 2).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(t.Border),

		Title: lipgloss.NewStyle().
			Foreground(t.TextPrimary).
			Bold(true),

		Subtitle: lipgloss.NewStyle().
			Foreground(t.TextSecondary),

		ListItem: lipgloss.NewStyle().
			Padding(0, 2),

		ListItemSelected: lipgloss.NewStyle().
			Padding(0, 2).
			Background(t.Selected).
			Foreground(t.TextPrimary),

		FileName: lipgloss.NewStyle().
			Foreground(t.TextPrimary),

		FileSize: lipgloss.NewStyle().
			Foreground(t.TextSecondary).
			Width(10).
			Align(lipgloss.Right),

		FilePath: lipgloss.NewStyle().
			Foreground(t.TextMuted),

		StatusBar: lipgloss.NewStyle().
			Foreground(t.TextSecondary).
			Padding(0, 1),

		KeyBind: lipgloss.NewStyle().
			Foreground(t.TextPrimary).
			Background(t.Surface).
			Padding(0, 1),

		KeyBindDesc: lipgloss.NewStyle().
			Foreground(t.TextMuted),

		Spinner: lipgloss.NewStyle().
			Foreground(t.TextPrimary),

		Progress: lipgloss.NewStyle().
			Foreground(t.TextPrimary),

		Checkbox: lipgloss.NewStyle().
			Foreground(t.TextMuted),

		CheckboxSelected: lipgloss.NewStyle().
			Foreground(t.Success),

		CategorySafe: lipgloss.NewStyle().
			Foreground(t.Success),

		CategoryCaution: lipgloss.NewStyle().
			Foreground(t.Warning),

		CategoryDanger: lipgloss.NewStyle().
			Foreground(t.Danger),

		Dialog: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(1, 2),

		DialogTitle: lipgloss.NewStyle().
			Foreground(t.TextPrimary).
			Bold(true).
			MarginBottom(1),

		DialogButton: lipgloss.NewStyle().
			Foreground(t.TextPrimary).
			Background(t.Surface).
			Padding(0, 2).
			MarginRight(1),
	}
}
