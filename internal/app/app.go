package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ash/internal/cleaner"
	"ash/internal/maintenance"
	"ash/internal/scanner"
	"ash/internal/tui"
)

// View represents the current view state.
type View int

const (
	ViewHome View = iota
	ViewScanning
	ViewResults
	ViewConfirm
	ViewCleaning
	ViewMaintenance
)

// KeyMap defines the key bindings.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Select    key.Binding
	SelectAll key.Binding
	Confirm   key.Binding
	Back      key.Binding
	Quit      key.Binding
	Help      key.Binding
	Tab       key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys(" ", "x"),
			key.WithHelp("space/x", "select"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "b"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch"),
		),
	}
}

// Model represents the application state.
type Model struct {
	// View state
	currentView View
	width       int
	height      int

	// Theme and styles
	theme  tui.Theme
	styles tui.Styles

	// Data
	entries       []scanner.Entry
	selected      map[string]bool
	totalSize     int64
	selectedSize  int64
	selectedCount int

	// List state
	cursor   int
	offset   int
	pageSize int

	// Scan state
	scanning     bool
	scanProgress float64
	scanMessage  string

	// Clean state
	cleaning   bool
	cleanStats *cleaner.CleanStats

	// Maintenance state
	maintenanceCommands []*maintenance.Command
	maintenanceCursor   int

	// Components
	spinner  spinner.Model
	progress progress.Model

	// Key bindings
	keys KeyMap

	// Error state
	lastError error

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new application model.
func New() Model {
	theme := tui.DefaultTheme
	styles := tui.NewStyles(theme)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.Spinner

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		currentView:         ViewHome,
		theme:               theme,
		styles:              styles,
		selected:            make(map[string]bool),
		pageSize:            20,
		spinner:             s,
		progress:            p,
		keys:                DefaultKeyMap(),
		maintenanceCommands: maintenance.GetCommands(),
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.pageSize = msg.Height - 10
		if m.pageSize < 5 {
			m.pageSize = 5
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case ScanStartedMsg:
		m.scanning = true
		m.currentView = ViewScanning
		return m, m.spinner.Tick

	case ScanProgressMsg:
		m.scanProgress = msg.Progress
		m.scanMessage = msg.Message
		return m, nil

	case ScanCompleteMsg:
		m.scanning = false
		m.entries = msg.Entries
		m.totalSize = msg.TotalSize
		m.currentView = ViewResults
		m.sortEntries()
		return m, nil

	case ScanErrorMsg:
		m.scanning = false
		m.lastError = msg.Err
		m.currentView = ViewHome
		return m, nil

	case CleanStartedMsg:
		m.cleaning = true
		m.currentView = ViewCleaning
		return m, m.spinner.Tick

	case CleanCompleteMsg:
		m.cleaning = false
		m.cleanStats = msg.Stats
		m.currentView = ViewResults
		// Remove cleaned entries
		m.removeCleanedEntries()
		return m, nil

	case CleanErrorMsg:
		m.cleaning = false
		m.lastError = msg.Err
		return m, nil

	case MaintenanceCompleteMsg:
		// Handle maintenance result
		return m, nil

	case ErrorMsg:
		m.lastError = msg.Err
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.cancel()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		return m.handleBack()

	case key.Matches(msg, m.keys.Up):
		return m.handleUp()

	case key.Matches(msg, m.keys.Down):
		return m.handleDown()

	case key.Matches(msg, m.keys.Select):
		return m.handleSelect()

	case key.Matches(msg, m.keys.SelectAll):
		return m.handleSelectAll()

	case key.Matches(msg, m.keys.Confirm):
		return m.handleConfirm()

	case key.Matches(msg, m.keys.Tab):
		return m.handleTab()
	}

	return m, nil
}

func (m *Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewResults, ViewMaintenance:
		m.currentView = ViewHome
		m.cursor = 0
	case ViewConfirm:
		m.currentView = ViewResults
	default:
		// No action for other views
	}
	return m, nil
}

func (m *Model) handleUp() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewResults:
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case ViewMaintenance:
		if m.maintenanceCursor > 0 {
			m.maintenanceCursor--
		}
	case ViewHome:
		if m.cursor > 0 {
			m.cursor--
		}
	default:
		// No action for other views
	}
	return m, nil
}

func (m *Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewResults:
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			if m.cursor >= m.offset+m.pageSize {
				m.offset = m.cursor - m.pageSize + 1
			}
		}
	case ViewMaintenance:
		if m.maintenanceCursor < len(m.maintenanceCommands)-1 {
			m.maintenanceCursor++
		}
	case ViewHome:
		if m.cursor < 2 { // 3 options: Scan, Maintenance, Quit
			m.cursor++
		}
	default:
		// No action for other views
	}
	return m, nil
}

func (m *Model) handleSelect() (tea.Model, tea.Cmd) {
	if m.currentView != ViewResults || len(m.entries) == 0 {
		return m, nil
	}

	entry := &m.entries[m.cursor]
	if m.selected[entry.Path] {
		delete(m.selected, entry.Path)
		m.selectedSize -= entry.Size
		m.selectedCount--
	} else {
		m.selected[entry.Path] = true
		m.selectedSize += entry.Size
		m.selectedCount++
	}

	return m, nil
}

func (m *Model) handleSelectAll() (tea.Model, tea.Cmd) {
	if m.currentView != ViewResults {
		return m, nil
	}

	if m.selectedCount == len(m.entries) {
		// Deselect all
		m.selected = make(map[string]bool)
		m.selectedSize = 0
		m.selectedCount = 0
	} else {
		// Select all
		m.selected = make(map[string]bool)
		m.selectedSize = 0
		m.selectedCount = 0
		for _, entry := range m.entries {
			m.selected[entry.Path] = true
			m.selectedSize += entry.Size
			m.selectedCount++
		}
	}

	return m, nil
}

func (m *Model) handleConfirm() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewHome:
		switch m.cursor {
		case 0: // Scan
			m.currentView = ViewScanning
			m.scanning = true
			return m, tea.Batch(m.spinner.Tick, StartModuleScan(m.ctx))
		case 1: // Maintenance
			m.currentView = ViewMaintenance
		case 2: // Quit
			return m, tea.Quit
		}

	case ViewResults:
		if m.selectedCount > 0 {
			m.currentView = ViewConfirm
		}

	case ViewConfirm:
		// Start cleaning
		var toClean []scanner.Entry
		for _, entry := range m.entries {
			if m.selected[entry.Path] {
				toClean = append(toClean, entry)
			}
		}
		return m, tea.Batch(m.spinner.Tick, StartClean(m.ctx, toClean, false))

	case ViewMaintenance:
		cmd := m.maintenanceCommands[m.maintenanceCursor]
		return m, RunMaintenanceCommand(m.ctx, cmd)

	default:
		// No action for other views
	}

	return m, nil
}

func (m *Model) handleTab() (tea.Model, tea.Cmd) {
	// Switch between views or tabs
	return m, nil
}

func (m *Model) sortEntries() {
	sort.Slice(m.entries, func(i, j int) bool {
		return m.entries[i].Size > m.entries[j].Size
	})
}

func (m *Model) removeCleanedEntries() {
	var remaining []scanner.Entry
	for _, entry := range m.entries {
		if !m.selected[entry.Path] {
			remaining = append(remaining, entry)
		}
	}
	m.entries = remaining
	m.selected = make(map[string]bool)
	m.selectedSize = 0
	m.selectedCount = 0
	m.cursor = 0
	m.offset = 0
}

// View renders the current view.
func (m Model) View() string {
	switch m.currentView {
	case ViewHome:
		return m.renderHome()
	case ViewScanning:
		return m.renderScanning()
	case ViewResults:
		return m.renderResults()
	case ViewConfirm:
		return m.renderConfirm()
	case ViewCleaning:
		return m.renderCleaning()
	case ViewMaintenance:
		return m.renderMaintenance()
	default:
		return ""
	}
}

func (m Model) renderHome() string {
	var b strings.Builder

	title := m.styles.Title.Render("ash")
	subtitle := m.styles.Subtitle.Render("macOS cleanup utility")

	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(subtitle)
	b.WriteString("\n\n")

	options := []string{"Scan for junk", "Maintenance", "Quit"}

	for i, opt := range options {
		cursor := "  "
		style := m.styles.ListItem
		if i == m.cursor {
			cursor = "> "
			style = m.styles.ListItemSelected
		}
		b.WriteString(cursor)
		b.WriteString(style.Render(opt))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderScanning() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.styles.Title.Render("Scanning..."))
	b.WriteString("\n\n")
	b.WriteString(m.spinner.View())
	b.WriteString(" ")
	b.WriteString(m.scanMessage)
	b.WriteString("\n")

	return b.String()
}

func (m Model) renderResults() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.styles.Title.Render("Results"))
	b.WriteString("\n")

	summary := fmt.Sprintf("Found %d items (%s) | Selected %d items (%s)",
		len(m.entries),
		scanner.FormatSize(m.totalSize),
		m.selectedCount,
		scanner.FormatSize(m.selectedSize),
	)
	b.WriteString(m.styles.Subtitle.Render(summary))
	b.WriteString("\n\n")

	if len(m.entries) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#525252"))
		b.WriteString(mutedStyle.Render("No items found"))
		b.WriteString("\n")
	} else {
		// Render visible entries
		end := m.offset + m.pageSize
		if end > len(m.entries) {
			end = len(m.entries)
		}

		for i := m.offset; i < end; i++ {
			entry := m.entries[i]
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			checkbox := "[ ]"
			if m.selected[entry.Path] {
				checkbox = "[x]"
			}

			name := entry.Name
			if len(name) > 40 {
				name = name[:37] + "..."
			}

			size := scanner.FormatSize(entry.Size)

			line := fmt.Sprintf("%s%s %s %s", cursor, checkbox,
				lipgloss.NewStyle().Width(42).Render(name),
				m.styles.FileSize.Render(size))
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Scrollbar indicator
		if len(m.entries) > m.pageSize {
			b.WriteString(fmt.Sprintf("\n[%d-%d of %d]", m.offset+1, end, len(m.entries)))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderConfirm() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.styles.DialogTitle.Render("Confirm Cleanup"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Move %d items (%s) to Trash?\n\n",
		m.selectedCount,
		scanner.FormatSize(m.selectedSize),
	))

	b.WriteString("Press ENTER to confirm, ESC to cancel")

	return b.String()
}

func (m Model) renderCleaning() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.styles.Title.Render("Cleaning..."))
	b.WriteString("\n\n")
	b.WriteString(m.spinner.View())
	b.WriteString(" Moving items to Trash")
	b.WriteString("\n")

	return b.String()
}

func (m Model) renderMaintenance() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.styles.Title.Render("Maintenance"))
	b.WriteString("\n")
	b.WriteString(m.styles.Subtitle.Render("System maintenance commands"))
	b.WriteString("\n\n")

	for i, cmd := range m.maintenanceCommands {
		cursor := "  "
		style := m.styles.ListItem
		if i == m.maintenanceCursor {
			cursor = "> "
			style = m.styles.ListItemSelected
		}

		sudoIndicator := ""
		if cmd.RequiresSudo {
			sudoIndicator = " [sudo]"
		}

		b.WriteString(cursor)
		b.WriteString(style.Render(cmd.Name + sudoIndicator))
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(m.styles.Subtitle.Render(cmd.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderHelp() string {
	var keys []string

	switch m.currentView {
	case ViewHome:
		keys = []string{"j/k:navigate", "enter:select", "q:quit"}
	case ViewResults:
		keys = []string{"j/k:navigate", "space:toggle", "a:all", "enter:clean", "esc:back", "q:quit"}
	case ViewMaintenance:
		keys = []string{"j/k:navigate", "enter:run", "esc:back", "q:quit"}
	default:
		keys = []string{"esc:back", "q:quit"}
	}

	return m.styles.StatusBar.Render(strings.Join(keys, " | "))
}
