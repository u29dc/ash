package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/scanner"
	"ash/internal/tui"
)

// ConfirmView renders the deletion confirmation screen.
type ConfirmView struct {
	styles    tui.Styles
	entries   []scanner.Entry
	totalSize int64
	cursor    int // 0 = Cancel, 1 = Confirm
	width     int
	height    int
}

// NewConfirmView creates a new confirm view.
func NewConfirmView(styles tui.Styles) *ConfirmView {
	return &ConfirmView{
		styles: styles,
		cursor: 0,
	}
}

// SetSize sets the view dimensions.
func (v *ConfirmView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetEntries sets the entries to be cleaned.
func (v *ConfirmView) SetEntries(entries []scanner.Entry) {
	v.entries = entries
	v.totalSize = 0
	for i := range entries {
		v.totalSize += entries[i].Size
	}
}

// MoveLeft moves cursor to Cancel.
func (v *ConfirmView) MoveLeft() {
	v.cursor = 0
}

// MoveRight moves cursor to Confirm.
func (v *ConfirmView) MoveRight() {
	v.cursor = 1
}

// Toggle toggles between Cancel and Confirm.
func (v *ConfirmView) Toggle() {
	if v.cursor == 0 {
		v.cursor = 1
	} else {
		v.cursor = 0
	}
}

// IsConfirmed returns whether Confirm is selected.
func (v *ConfirmView) IsConfirmed() bool {
	return v.cursor == 1
}

// Cursor returns the current cursor position.
func (v *ConfirmView) Cursor() int {
	return v.cursor
}

// Render renders the confirm view.
func (v *ConfirmView) Render() string {
	var b strings.Builder

	// Dialog container
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f59e0b")).
		Padding(1, 3).
		Width(60)

	// Title
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f59e0b")).
		Bold(true).
		Render("Confirm Cleanup")

	b.WriteString(title)
	b.WriteString("\n\n")

	// Warning message
	msg := fmt.Sprintf("Move %d items (%s) to Trash?",
		len(v.entries),
		scanner.FormatSize(v.totalSize),
	)
	b.WriteString(msg)
	b.WriteString("\n\n")

	// Note about recovery
	note := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a3a3a3")).
		Render("Items can be recovered from Trash.")
	b.WriteString(note)
	b.WriteString("\n\n")

	// Buttons
	b.WriteString(v.renderButtons())

	return dialogStyle.Render(b.String())
}

func (v *ConfirmView) renderButtons() string {
	cancelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fafafa")).
		Background(lipgloss.Color("#404040")).
		Padding(0, 3)

	confirmStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fafafa")).
		Background(lipgloss.Color("#ef4444")).
		Padding(0, 3)

	// Highlight selected button
	if v.cursor == 0 {
		cancelStyle = cancelStyle.
			Background(lipgloss.Color("#737373")).
			Bold(true)
	} else {
		confirmStyle = confirmStyle.
			Background(lipgloss.Color("#dc2626")).
			Bold(true)
	}

	cancel := cancelStyle.Render("Cancel")
	confirm := confirmStyle.Render("Clean")

	return cancel + "  " + confirm
}

// RenderCentered renders the confirm view centered on screen.
func (v *ConfirmView) RenderCentered() string {
	dialog := v.Render()

	return lipgloss.Place(
		v.width, v.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}
