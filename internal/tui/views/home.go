package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/tui"
)

// MenuItem represents a menu item on the home screen.
type MenuItem struct {
	Title       string
	Description string
	Action      string
}

// HomeView renders the home screen.
type HomeView struct {
	styles tui.Styles
	items  []MenuItem
	cursor int
	width  int
	height int
}

// NewHomeView creates a new home view.
func NewHomeView(styles tui.Styles) *HomeView {
	return &HomeView{
		styles: styles,
		items: []MenuItem{
			{
				Title:       "Scan for Junk",
				Description: "Find caches, logs, and other cleanable files",
				Action:      "scan",
			},
			{
				Title:       "Deep Scan",
				Description: "Include app leftovers (slower, more cautious)",
				Action:      "deep-scan",
			},
			{
				Title:       "Maintenance",
				Description: "Run system maintenance commands",
				Action:      "maintenance",
			},
			{
				Title:       "Quit",
				Description: "Exit the application",
				Action:      "quit",
			},
		},
	}
}

// SetSize sets the view dimensions.
func (v *HomeView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetCursor sets the cursor position.
func (v *HomeView) SetCursor(pos int) {
	if pos >= 0 && pos < len(v.items) {
		v.cursor = pos
	}
}

// MoveUp moves the cursor up.
func (v *HomeView) MoveUp() {
	if v.cursor > 0 {
		v.cursor--
	}
}

// MoveDown moves the cursor down.
func (v *HomeView) MoveDown() {
	if v.cursor < len(v.items)-1 {
		v.cursor++
	}
}

// SelectedAction returns the currently selected action.
func (v *HomeView) SelectedAction() string {
	return v.items[v.cursor].Action
}

// Cursor returns the current cursor position.
func (v *HomeView) Cursor() int {
	return v.cursor
}

// Render renders the home view.
func (v *HomeView) Render() string {
	var b strings.Builder

	// Logo/Title
	logo := `
               __
  ____ _  ___ / /_
 / __ '/ / __/ __ \
/ /_/ / _\ \/ / / /
\__,_/ /___/_/ /_/
`
	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fafafa")).
		Bold(true)

	b.WriteString(logoStyle.Render(logo))
	b.WriteString("\n")

	tagline := v.styles.Subtitle.Render("macOS cleanup utility")
	b.WriteString(tagline)
	b.WriteString("\n\n")

	// Menu items
	for i, item := range v.items {
		cursor := "  "
		titleStyle := v.styles.FileName
		descStyle := v.styles.Subtitle

		if i == v.cursor {
			cursor = "> "
			titleStyle = titleStyle.
				Background(lipgloss.Color("#262626")).
				Bold(true)
			descStyle = descStyle.
				Background(lipgloss.Color("#262626"))
		}

		b.WriteString(cursor)
		b.WriteString(titleStyle.Render(item.Title))
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(descStyle.Render(item.Description))
		b.WriteString("\n\n")
	}

	// Help
	help := v.styles.StatusBar.Render("j/k:navigate | enter:select | q:quit")
	b.WriteString("\n")
	b.WriteString(help)

	return b.String()
}
