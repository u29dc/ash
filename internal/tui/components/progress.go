package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/tui"
)

// ProgressBar renders a progress bar.
type ProgressBar struct {
	styles   tui.Styles
	progress float64
	width    int
	label    string
}

// NewProgressBar creates a new progress bar component.
func NewProgressBar(styles tui.Styles) *ProgressBar {
	return &ProgressBar{
		styles: styles,
		width:  40,
	}
}

// SetProgress sets the progress value (0.0 to 1.0).
func (p *ProgressBar) SetProgress(progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	p.progress = progress
}

// SetWidth sets the bar width.
func (p *ProgressBar) SetWidth(width int) {
	p.width = width
}

// SetLabel sets the progress label.
func (p *ProgressBar) SetLabel(label string) {
	p.label = label
}

// Render renders the progress bar.
func (p *ProgressBar) Render() string {
	filled := int(float64(p.width) * p.progress)
	empty := p.width - filled

	percentage := fmt.Sprintf("%3.0f%%", p.progress*100)

	progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fafafa"))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#404040"))

	renderedBar := progressStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))

	result := fmt.Sprintf("%s %s", renderedBar, percentage)

	if p.label != "" {
		result = fmt.Sprintf("%s\n%s", p.label, result)
	}

	return result
}

// RenderIndeterminate renders an indeterminate progress indicator.
func (p *ProgressBar) RenderIndeterminate(frame int) string {
	chars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := chars[frame%len(chars)]

	result := spinner
	if p.label != "" {
		result = fmt.Sprintf("%s %s", spinner, p.label)
	}

	return result
}
