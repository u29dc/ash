package components

import (
	"github.com/charmbracelet/lipgloss"

	"ash/internal/tui"
)

// Header renders the application header.
type Header struct {
	styles tui.Styles
	title  string
}

// NewHeader creates a new header component.
func NewHeader(styles tui.Styles, title string) *Header {
	return &Header{
		styles: styles,
		title:  title,
	}
}

// SetTitle sets the header title.
func (h *Header) SetTitle(title string) {
	h.title = title
}

// Render renders the header.
func (h *Header) Render(width int) string {
	title := h.styles.Title.Render(h.title)

	return lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#404040")).
		Render(title)
}

// RenderWithSubtitle renders the header with a subtitle.
func (h *Header) RenderWithSubtitle(width int, subtitle string) string {
	title := h.styles.Title.Render(h.title)
	sub := h.styles.Subtitle.Render(subtitle)

	content := lipgloss.JoinVertical(lipgloss.Left, title, sub)

	return lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#404040")).
		Render(content)
}
