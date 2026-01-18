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
	ViewAuthorizing
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

	// Auth state
	authGranted        bool
	authInProgress     bool
	pendingClean       []scanner.Entry
	pendingMaintenance *maintenance.Command

	// Clean state
	cleaning   bool
	cleanStats *cleaner.CleanStats

	// Maintenance state
	maintenanceCommands []*maintenance.Command
	maintenanceCursor   int
	maintenanceRunning  bool
	maintenanceResult   *maintenance.CommandResult
	maintenanceActive   *maintenance.Command

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
		m.scanMessage = ""
		m.entries = msg.Entries
		m.totalSize = msg.TotalSize
		m.selected = make(map[string]bool)
		m.selectedSize = 0
		m.selectedCount = 0
		m.cursor = 0
		m.offset = 0
		m.lastError = nil
		m.currentView = ViewResults
		m.sortEntries()
		return m, nil

	case ScanErrorMsg:
		m.scanning = false
		m.scanMessage = ""
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
		m.removeCleanedEntries(msg.Stats)
		return m, nil

	case CleanErrorMsg:
		m.cleaning = false
		m.lastError = msg.Err
		m.currentView = ViewResults
		return m, nil

	case AuthSuccessMsg:
		m.authGranted = true
		m.authInProgress = false
		m.lastError = nil

		if len(m.pendingClean) > 0 {
			toClean := m.pendingClean
			m.pendingClean = nil
			m.cleaning = true
			m.currentView = ViewCleaning
			return m, tea.Batch(m.spinner.Tick, StartClean(m.ctx, toClean, false))
		}

		if m.pendingMaintenance != nil {
			cmd := m.pendingMaintenance
			m.pendingMaintenance = nil
			m.maintenanceRunning = true
			m.currentView = ViewMaintenance
			m.maintenanceActive = cmd
			return m, tea.Batch(m.spinner.Tick, RunMaintenanceCommand(m.ctx, cmd))
		}

		return m, nil

	case AuthErrorMsg:
		m.authInProgress = false
		m.lastError = msg.Err
		if len(m.pendingClean) > 0 {
			m.pendingClean = nil
			m.currentView = ViewConfirm
		} else if m.pendingMaintenance != nil {
			m.pendingMaintenance = nil
			m.currentView = ViewMaintenance
		}
		return m, nil

	case MaintenanceCompleteMsg:
		// Handle maintenance result
		m.maintenanceRunning = false
		m.maintenanceActive = nil
		m.maintenanceResult = msg.Result
		if msg.Result != nil && msg.Result.Error != nil {
			m.lastError = msg.Result.Error
		} else {
			m.lastError = nil
		}
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
		if m.cursor < 3 { // 4 options: Scan, Deep Scan, Maintenance, Quit
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
		for i := range m.entries {
			m.selected[m.entries[i].Path] = true
			m.selectedSize += m.entries[i].Size
			m.selectedCount++
		}
	}

	return m, nil
}

func (m *Model) handleConfirm() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewHome:
		return m.handleConfirmHome()
	case ViewResults:
		return m.handleConfirmResults()
	case ViewConfirm:
		return m.handleConfirmCleanup()
	case ViewMaintenance:
		return m.handleConfirmMaintenance()
	default:
		// No action for other views
	}

	return m, nil
}

func (m *Model) handleConfirmHome() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0:
		return m.startScan(false, "Scanning standard locations...")
	case 1:
		return m.startScan(true, "Scanning with app leftovers...")
	case 2:
		m.currentView = ViewMaintenance
		return m, nil
	case 3:
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m *Model) handleConfirmResults() (tea.Model, tea.Cmd) {
	if m.selectedCount > 0 {
		m.currentView = ViewConfirm
	}
	return m, nil
}

func (m *Model) handleConfirmCleanup() (tea.Model, tea.Cmd) {
	toClean := m.selectedEntries()
	if len(toClean) == 0 {
		return m, nil
	}
	return m.beginCleanup(toClean)
}

func (m *Model) handleConfirmMaintenance() (tea.Model, tea.Cmd) {
	cmd := m.maintenanceCommands[m.maintenanceCursor]
	return m.beginMaintenance(cmd)
}

func (m *Model) startScan(includeAppData bool, message string) (tea.Model, tea.Cmd) {
	m.currentView = ViewScanning
	m.scanning = true
	m.scanMessage = message
	return m, tea.Batch(m.spinner.Tick, StartModuleScan(m.ctx, includeAppData))
}

func (m *Model) beginCleanup(entries []scanner.Entry) (tea.Model, tea.Cmd) {
	if !m.authGranted {
		return m.requestAuthForClean(entries)
	}
	m.lastError = nil
	m.cleanStats = nil
	m.cleaning = true
	m.currentView = ViewCleaning
	return m, tea.Batch(m.spinner.Tick, StartClean(m.ctx, entries, false))
}

func (m *Model) requestAuthForClean(entries []scanner.Entry) (tea.Model, tea.Cmd) {
	m.lastError = nil
	m.authInProgress = true
	m.pendingClean = entries
	m.currentView = ViewAuthorizing
	return m, tea.Batch(m.spinner.Tick, RequestAuth(m.ctx))
}

func (m *Model) beginMaintenance(cmd *maintenance.Command) (tea.Model, tea.Cmd) {
	if cmd.RequiresSudo && !m.authGranted {
		return m.requestAuthForMaintenance(cmd)
	}
	m.lastError = nil
	m.maintenanceResult = nil
	m.maintenanceRunning = true
	m.maintenanceActive = cmd
	return m, tea.Batch(m.spinner.Tick, RunMaintenanceCommand(m.ctx, cmd))
}

func (m *Model) requestAuthForMaintenance(cmd *maintenance.Command) (tea.Model, tea.Cmd) {
	m.lastError = nil
	m.authInProgress = true
	m.pendingMaintenance = cmd
	m.currentView = ViewAuthorizing
	return m, tea.Batch(m.spinner.Tick, RequestAuth(m.ctx))
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

func (m *Model) selectedEntries() []scanner.Entry {
	var selected []scanner.Entry
	for i := range m.entries {
		if m.selected[m.entries[i].Path] {
			selected = append(selected, m.entries[i])
		}
	}
	return selected
}

func (m *Model) removeCleanedEntries(stats *cleaner.CleanStats) {
	failed := make(map[string]bool)
	if stats != nil {
		for _, result := range stats.Errors {
			failed[result.Path] = true
		}
	}

	var remaining []scanner.Entry
	var totalSize int64
	for i := range m.entries {
		if m.selected[m.entries[i].Path] && !failed[m.entries[i].Path] {
			continue
		}
		remaining = append(remaining, m.entries[i])
		totalSize += m.entries[i].Size
	}
	m.entries = remaining
	m.totalSize = totalSize
	m.selected = make(map[string]bool)
	m.selectedSize = 0
	m.selectedCount = 0
	for i := range remaining {
		if failed[remaining[i].Path] {
			m.selected[remaining[i].Path] = true
			m.selectedSize += remaining[i].Size
			m.selectedCount++
		}
	}
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
	case ViewAuthorizing:
		return m.renderAuthorizing()
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
	if errLine := m.renderError(); errLine != "" {
		b.WriteString(errLine)
		b.WriteString("\n\n")
	}

	options := []string{"Scan for junk", "Deep scan (includes app leftovers)", "Maintenance", "Quit"}

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
	b.WriteString(m.renderResultsSummary())
	b.WriteString("\n\n")
	if errLine := m.renderError(); errLine != "" {
		b.WriteString(errLine)
		b.WriteString("\n\n")
	}

	b.WriteString(m.renderResultsBody())

	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderResultsSummary() string {
	summary := fmt.Sprintf("Found %d items (%s) | Selected %d items (%s)",
		len(m.entries),
		scanner.FormatSize(m.totalSize),
		m.selectedCount,
		scanner.FormatSize(m.selectedSize),
	)
	return m.styles.Subtitle.Render(summary)
}

func (m Model) renderResultsBody() string {
	if len(m.entries) == 0 {
		return m.renderResultsEmpty()
	}
	return m.renderResultsEntries()
}

func (m Model) renderResultsEmpty() string {
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#525252"))
	return mutedStyle.Render("No items found") + "\n"
}

func (m Model) renderResultsEntries() string {
	var b strings.Builder

	end := m.offset + m.pageSize
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderResultsEntry(i))
		b.WriteString("\n")
	}

	if len(m.entries) > m.pageSize {
		b.WriteString(fmt.Sprintf("\n[%d-%d of %d]", m.offset+1, end, len(m.entries)))
	}

	return b.String()
}

func (m Model) renderResultsEntry(index int) string {
	entry := m.entries[index]
	cursor := "  "
	if index == m.cursor {
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

	return fmt.Sprintf("%s%s %s %s",
		cursor,
		checkbox,
		lipgloss.NewStyle().Width(42).Render(name),
		m.styles.FileSize.Render(size),
	)
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

	if !m.authGranted {
		b.WriteString("Authorization required before cleanup.\n\n")
	}

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

func (m Model) renderAuthorizing() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.styles.Title.Render("Authorizing..."))
	b.WriteString("\n\n")
	b.WriteString(m.spinner.View())
	b.WriteString(" Waiting for Touch ID or password")
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
	if errLine := m.renderError(); errLine != "" {
		b.WriteString(errLine)
		b.WriteString("\n\n")
	}

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

	if m.maintenanceRunning && m.maintenanceActive != nil {
		b.WriteString("\n")
		b.WriteString(m.spinner.View())
		b.WriteString(" Running ")
		b.WriteString(m.maintenanceActive.Name)
		b.WriteString("\n")
	}

	if m.maintenanceResult != nil {
		b.WriteString("\n")
		b.WriteString(m.renderMaintenanceResult())
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
	case ViewAuthorizing:
		keys = []string{"authorizing"}
	default:
		keys = []string{"esc:back", "q:quit"}
	}

	return m.styles.StatusBar.Render(strings.Join(keys, " | "))
}

func (m Model) renderError() string {
	if m.lastError == nil {
		return ""
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
	return style.Render("Error: " + m.lastError.Error())
}

func (m Model) renderMaintenanceResult() string {
	if m.maintenanceResult == nil {
		return ""
	}

	result := m.maintenanceResult
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	status := "Success"
	if !result.Success {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
		status = "Failed"
	}

	header := statusStyle.Render(fmt.Sprintf("%s: %s (%s)", status, result.Command.Name, result.Duration.String()))
	output := strings.TrimSpace(result.Output)
	if output == "" {
		return header
	}

	output = truncateLines(output, 3, 200)
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("#a3a3a3")).Render(output)
	return header + "\n" + body
}

func truncateLines(input string, maxLines int, maxLen int) string {
	if input == "" {
		return input
	}
	lines := strings.Split(input, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	joined := strings.Join(lines, "\n")
	if len(joined) > maxLen {
		joined = joined[:maxLen-3] + "..."
	}
	return joined
}
