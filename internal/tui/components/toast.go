package components

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/tui"
)

// ToastType defines the type of toast message.
type ToastType int

const (
	ToastInfo ToastType = iota
	ToastSuccess
	ToastWarning
	ToastError
)

// Toast represents a temporary status message.
type Toast struct {
	styles    tui.Styles
	message   string
	toastType ToastType
	visible   bool
	expiresAt time.Time
	duration  time.Duration
}

// NewToast creates a new toast component.
func NewToast(styles tui.Styles) *Toast {
	return &Toast{
		styles:   styles,
		duration: 3 * time.Second,
	}
}

// Show displays a toast message.
func (t *Toast) Show(message string, toastType ToastType) {
	t.message = message
	t.toastType = toastType
	t.visible = true
	t.expiresAt = time.Now().Add(t.duration)
}

// ShowInfo displays an info toast.
func (t *Toast) ShowInfo(message string) {
	t.Show(message, ToastInfo)
}

// ShowSuccess displays a success toast.
func (t *Toast) ShowSuccess(message string) {
	t.Show(message, ToastSuccess)
}

// ShowWarning displays a warning toast.
func (t *Toast) ShowWarning(message string) {
	t.Show(message, ToastWarning)
}

// ShowError displays an error toast.
func (t *Toast) ShowError(message string) {
	t.Show(message, ToastError)
}

// Hide hides the toast.
func (t *Toast) Hide() {
	t.visible = false
}

// SetDuration sets the toast duration.
func (t *Toast) SetDuration(d time.Duration) {
	t.duration = d
}

// IsVisible returns whether the toast is visible.
func (t *Toast) IsVisible() bool {
	if !t.visible {
		return false
	}

	// Check if toast has expired
	if time.Now().After(t.expiresAt) {
		t.visible = false
		return false
	}

	return true
}

// Render renders the toast.
func (t *Toast) Render() string {
	if !t.IsVisible() {
		return ""
	}

	var style lipgloss.Style
	var icon string

	switch t.toastType {
	case ToastInfo:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fafafa")).
			Background(lipgloss.Color("#404040"))
		icon = "i"
	case ToastSuccess:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fafafa")).
			Background(lipgloss.Color("#22c55e"))
		icon = "+"
	case ToastWarning:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0a0a0a")).
			Background(lipgloss.Color("#f59e0b"))
		icon = "!"
	case ToastError:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fafafa")).
			Background(lipgloss.Color("#ef4444"))
		icon = "x"
	}

	style = style.Padding(0, 2)

	return style.Render(icon + " " + t.message)
}

// RenderWithWidth renders the toast with a specific width.
func (t *Toast) RenderWithWidth(width int) string {
	if !t.IsVisible() {
		return ""
	}

	rendered := t.Render()

	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(rendered)
}
