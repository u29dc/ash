package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"ash/internal/tui"
)

// ScanView renders the scanning progress screen.
type ScanView struct {
	styles   tui.Styles
	spinner  spinner.Model
	message  string
	current  string
	progress float64
	width    int
	height   int
}

// NewScanView creates a new scan view.
func NewScanView(styles tui.Styles) *ScanView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.Spinner

	return &ScanView{
		styles:  styles,
		spinner: s,
		message: "Scanning...",
	}
}

// SetSize sets the view dimensions.
func (v *ScanView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetMessage sets the status message.
func (v *ScanView) SetMessage(message string) {
	v.message = message
}

// SetCurrent sets the currently scanned path.
func (v *ScanView) SetCurrent(path string) {
	v.current = path
}

// SetProgress sets the progress percentage.
func (v *ScanView) SetProgress(progress float64) {
	v.progress = progress
}

// UpdateSpinner updates the spinner state.
func (v *ScanView) UpdateSpinner(msg spinner.TickMsg) {
	v.spinner, _ = v.spinner.Update(msg)
}

// SpinnerTick returns the spinner tick command.
func (v *ScanView) SpinnerTick() tea.Cmd {
	return v.spinner.Tick
}

// Spinner returns the spinner model.
func (v *ScanView) Spinner() spinner.Model {
	return v.spinner
}

// Render renders the scan view.
func (v *ScanView) Render() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(v.styles.Title.Render("Scanning"))
	b.WriteString("\n\n")

	// Spinner and message
	b.WriteString(v.spinner.View())
	b.WriteString(" ")
	b.WriteString(v.message)
	b.WriteString("\n\n")

	// Current path being scanned
	if v.current != "" {
		current := v.current
		maxLen := 60
		if len(current) > maxLen {
			current = "..." + current[len(current)-maxLen+3:]
		}
		b.WriteString(v.styles.FilePath.Render(current))
		b.WriteString("\n")
	}

	// Progress bar (if we have progress)
	if v.progress > 0 {
		b.WriteString("\n")
		b.WriteString(v.renderProgressBar())
	}

	return b.String()
}

func (v *ScanView) renderProgressBar() string {
	width := 40
	filled := int(float64(width) * v.progress)
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	percentage := fmt.Sprintf("%3.0f%%", v.progress*100)

	return fmt.Sprintf("%s %s", bar, percentage)
}
