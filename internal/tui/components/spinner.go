package components

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ash/internal/tui"
)

// SpinnerType defines the type of spinner animation.
type SpinnerType int

const (
	SpinnerDot SpinnerType = iota
	SpinnerLine
	SpinnerPulse
	SpinnerPoints
)

// Spinner wraps the bubbles spinner with custom styling.
type Spinner struct {
	styles  tui.Styles
	spinner spinner.Model
	label   string
}

// NewSpinner creates a new spinner component.
func NewSpinner(styles tui.Styles, spinnerType SpinnerType) *Spinner {
	s := spinner.New()

	switch spinnerType {
	case SpinnerDot:
		s.Spinner = spinner.Dot
	case SpinnerLine:
		s.Spinner = spinner.Line
	case SpinnerPulse:
		s.Spinner = spinner.Pulse
	case SpinnerPoints:
		s.Spinner = spinner.Points
	default:
		s.Spinner = spinner.Dot
	}

	s.Style = styles.Spinner

	return &Spinner{
		styles:  styles,
		spinner: s,
	}
}

// SetLabel sets the spinner label.
func (s *Spinner) SetLabel(label string) {
	s.label = label
}

// Tick returns the spinner tick command.
func (s *Spinner) Tick() tea.Cmd {
	return s.spinner.Tick
}

// Update updates the spinner state.
func (s *Spinner) Update(msg spinner.TickMsg) {
	s.spinner, _ = s.spinner.Update(msg)
}

// View returns the spinner view.
func (s *Spinner) View() string {
	if s.label == "" {
		return s.spinner.View()
	}
	return s.spinner.View() + " " + s.label
}

// Model returns the underlying spinner model.
func (s *Spinner) Model() spinner.Model {
	return s.spinner
}

// CustomSpinner creates a spinner with custom frames.
func CustomSpinner(frames []string, fps time.Duration) spinner.Spinner {
	return spinner.Spinner{
		Frames: frames,
		FPS:    fps,
	}
}

// GrayscaleSpinner returns a grayscale-themed spinner.
func GrayscaleSpinner() spinner.Spinner {
	return spinner.Spinner{
		Frames: []string{"○", "◔", "◑", "◕", "●", "◕", "◑", "◔"},
		FPS:    time.Second / 10,
	}
}

// BlockSpinner returns a block-based spinner.
func BlockSpinner() spinner.Spinner {
	return spinner.Spinner{
		Frames: []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉", "█", "▉", "▊", "▋", "▌", "▍", "▎"},
		FPS:    time.Second / 10,
	}
}

// SpinnerWithStyle applies a style to a spinner.
func SpinnerWithStyle(s spinner.Model, style lipgloss.Style) spinner.Model {
	s.Style = style
	return s
}
