package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/maintenance"
	"ash/internal/tui"
)

// MaintenanceView renders the maintenance commands screen.
type MaintenanceView struct {
	styles   tui.Styles
	commands []*maintenance.Command
	results  map[string]*maintenance.CommandResult
	cursor   int
	running  bool
	width    int
	height   int
}

// NewMaintenanceView creates a new maintenance view.
func NewMaintenanceView(styles tui.Styles) *MaintenanceView {
	return &MaintenanceView{
		styles:   styles,
		commands: maintenance.GetCommands(),
		results:  make(map[string]*maintenance.CommandResult),
	}
}

// SetSize sets the view dimensions.
func (v *MaintenanceView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// MoveUp moves the cursor up.
func (v *MaintenanceView) MoveUp() {
	if v.cursor > 0 {
		v.cursor--
	}
}

// MoveDown moves the cursor down.
func (v *MaintenanceView) MoveDown() {
	if v.cursor < len(v.commands)-1 {
		v.cursor++
	}
}

// SelectedCommand returns the currently selected command.
func (v *MaintenanceView) SelectedCommand() *maintenance.Command {
	if v.cursor >= 0 && v.cursor < len(v.commands) {
		return v.commands[v.cursor]
	}
	return nil
}

// Cursor returns the current cursor position.
func (v *MaintenanceView) Cursor() int {
	return v.cursor
}

// SetRunning sets whether a command is currently running.
func (v *MaintenanceView) SetRunning(running bool) {
	v.running = running
}

// SetResult sets the result for a command.
func (v *MaintenanceView) SetResult(cmd *maintenance.Command, result *maintenance.CommandResult) {
	v.results[cmd.Name] = result
}

// IsRunning returns whether a command is running.
func (v *MaintenanceView) IsRunning() bool {
	return v.running
}

// Render renders the maintenance view.
func (v *MaintenanceView) Render() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(v.styles.Title.Render("Maintenance"))
	b.WriteString("\n")
	b.WriteString(v.styles.Subtitle.Render("System maintenance commands"))
	b.WriteString("\n\n")

	for i, cmd := range v.commands {
		line := v.renderCommand(cmd, i == v.cursor)
		b.WriteString(line)
		b.WriteString("\n")

		// Show result if available
		if result, ok := v.results[cmd.Name]; ok {
			b.WriteString(v.renderResult(result))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(v.renderHelp())

	return b.String()
}

func (v *MaintenanceView) renderCommand(cmd *maintenance.Command, isCursor bool) string {
	cursor := "  "
	if isCursor {
		cursor = "> "
	}

	titleStyle := v.styles.FileName
	descStyle := v.styles.Subtitle

	if isCursor {
		bg := lipgloss.Color("#262626")
		titleStyle = titleStyle.Copy().Background(bg).Bold(true)
		descStyle = descStyle.Copy().Background(bg)
	}

	// Sudo indicator
	sudoIndicator := ""
	if cmd.RequiresSudo {
		sudoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f59e0b")).
			Bold(true)
		sudoIndicator = sudoStyle.Render(" [sudo]")
	}

	// Useful indicator
	usefulIndicator := ""
	if !cmd.Useful {
		usefulStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#525252"))
		usefulIndicator = usefulStyle.Render(" (limited benefit)")
	}

	var b strings.Builder
	b.WriteString(cursor)
	b.WriteString(titleStyle.Render(cmd.Name))
	b.WriteString(sudoIndicator)
	b.WriteString(usefulIndicator)
	b.WriteString("\n")
	b.WriteString("    ")
	b.WriteString(descStyle.Render(cmd.Description))

	return b.String()
}

func (v *MaintenanceView) renderResult(result *maintenance.CommandResult) string {
	var status string
	var style lipgloss.Style

	if result.Success {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
		status = fmt.Sprintf("    + Completed in %s", result.Duration.Round(1))
	} else {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Error()
		}
		status = fmt.Sprintf("    x Failed: %s", errMsg)
	}

	return style.Render(status)
}

func (v *MaintenanceView) renderHelp() string {
	help := "j/k:navigate | enter:run | esc:back | q:quit"
	return v.styles.StatusBar.Render(help)
}
