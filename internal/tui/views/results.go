package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/scanner"
	"ash/internal/tui"
)

// ResultsView renders the scan results screen.
type ResultsView struct {
	styles        tui.Styles
	entries       []scanner.Entry
	selected      map[string]bool
	cursor        int
	offset        int
	pageSize      int
	width         int
	height        int
	totalSize     int64
	selectedSize  int64
	selectedCount int
}

// NewResultsView creates a new results view.
func NewResultsView(styles tui.Styles) *ResultsView {
	return &ResultsView{
		styles:   styles,
		selected: make(map[string]bool),
		pageSize: 15,
	}
}

// SetSize sets the view dimensions.
func (v *ResultsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.pageSize = height - 10
	if v.pageSize < 5 {
		v.pageSize = 5
	}
}

// SetEntries sets the entries to display.
func (v *ResultsView) SetEntries(entries []scanner.Entry, totalSize int64) {
	v.entries = entries
	v.totalSize = totalSize
	v.cursor = 0
	v.offset = 0
}

// SetSelected sets the selection state.
func (v *ResultsView) SetSelected(selected map[string]bool) {
	v.selected = selected
	v.updateSelectedStats()
}

// MoveUp moves the cursor up.
func (v *ResultsView) MoveUp() {
	if v.cursor > 0 {
		v.cursor--
		if v.cursor < v.offset {
			v.offset = v.cursor
		}
	}
}

// MoveDown moves the cursor down.
func (v *ResultsView) MoveDown() {
	if v.cursor < len(v.entries)-1 {
		v.cursor++
		if v.cursor >= v.offset+v.pageSize {
			v.offset = v.cursor - v.pageSize + 1
		}
	}
}

// Toggle toggles selection of the current item.
func (v *ResultsView) Toggle() {
	if len(v.entries) == 0 {
		return
	}
	entry := v.entries[v.cursor]
	if v.selected[entry.Path] {
		delete(v.selected, entry.Path)
	} else {
		v.selected[entry.Path] = true
	}
	v.updateSelectedStats()
}

// SelectAll selects all entries.
func (v *ResultsView) SelectAll() {
	for _, entry := range v.entries {
		v.selected[entry.Path] = true
	}
	v.updateSelectedStats()
}

// DeselectAll deselects all entries.
func (v *ResultsView) DeselectAll() {
	v.selected = make(map[string]bool)
	v.updateSelectedStats()
}

// ToggleAll toggles all selections.
func (v *ResultsView) ToggleAll() {
	if v.selectedCount == len(v.entries) {
		v.DeselectAll()
	} else {
		v.SelectAll()
	}
}

func (v *ResultsView) updateSelectedStats() {
	v.selectedSize = 0
	v.selectedCount = 0
	for _, entry := range v.entries {
		if v.selected[entry.Path] {
			v.selectedSize += entry.Size
			v.selectedCount++
		}
	}
}

// Selected returns the selection map.
func (v *ResultsView) Selected() map[string]bool {
	return v.selected
}

// SelectedEntries returns the selected entries.
func (v *ResultsView) SelectedEntries() []scanner.Entry {
	var selected []scanner.Entry
	for _, entry := range v.entries {
		if v.selected[entry.Path] {
			selected = append(selected, entry)
		}
	}
	return selected
}

// SelectedCount returns the number of selected items.
func (v *ResultsView) SelectedCount() int {
	return v.selectedCount
}

// SelectedSize returns the total size of selected items.
func (v *ResultsView) SelectedSize() int64 {
	return v.selectedSize
}

// HasSelection returns whether any items are selected.
func (v *ResultsView) HasSelection() bool {
	return v.selectedCount > 0
}

// Cursor returns the current cursor position.
func (v *ResultsView) Cursor() int {
	return v.cursor
}

// Render renders the results view.
func (v *ResultsView) Render() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(v.styles.Title.Render("Results"))
	b.WriteString("\n")

	// Summary
	summary := fmt.Sprintf("Found %d items (%s) | Selected %d items (%s)",
		len(v.entries),
		scanner.FormatSize(v.totalSize),
		v.selectedCount,
		scanner.FormatSize(v.selectedSize),
	)
	b.WriteString(v.styles.Subtitle.Render(summary))
	b.WriteString("\n\n")

	if len(v.entries) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#525252")).
			Render("No items found"))
		b.WriteString("\n")
	} else {
		// Render entries
		b.WriteString(v.renderEntries())
	}

	b.WriteString("\n")
	b.WriteString(v.renderHelp())

	return b.String()
}

func (v *ResultsView) renderEntries() string {
	var b strings.Builder

	end := v.offset + v.pageSize
	if end > len(v.entries) {
		end = len(v.entries)
	}

	nameWidth := 45
	if v.width > 0 {
		nameWidth = v.width - 25
		if nameWidth < 25 {
			nameWidth = 25
		}
	}

	for i := v.offset; i < end; i++ {
		entry := v.entries[i]
		line := v.renderEntry(entry, i == v.cursor, nameWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(v.entries) > v.pageSize {
		indicator := fmt.Sprintf("\n[%d-%d of %d]", v.offset+1, end, len(v.entries))
		b.WriteString(v.styles.Subtitle.Render(indicator))
	}

	return b.String()
}

func (v *ResultsView) renderEntry(entry scanner.Entry, isCursor bool, nameWidth int) string {
	cursor := "  "
	if isCursor {
		cursor = "> "
	}

	checkbox := "[ ]"
	checkStyle := v.styles.Checkbox
	if v.selected[entry.Path] {
		checkbox = "[x]"
		checkStyle = v.styles.CheckboxSelected
	}

	name := entry.Name
	if len(name) > nameWidth {
		name = name[:nameWidth-3] + "..."
	}

	size := scanner.FormatSize(entry.Size)

	// Category indicator
	catStyle := v.styles.CategorySafe
	switch entry.Risk {
	case scanner.RiskSafe:
		catStyle = v.styles.CategorySafe
	case scanner.RiskCaution:
		catStyle = v.styles.CategoryCaution
	case scanner.RiskDangerous:
		catStyle = v.styles.CategoryDanger
	}

	catIndicator := catStyle.Render("●")

	nameStyle := v.styles.FileName
	sizeStyle := v.styles.FileSize

	if isCursor {
		bg := lipgloss.Color("#262626")
		nameStyle = nameStyle.Background(bg)
		sizeStyle = sizeStyle.Background(bg)
	}

	return fmt.Sprintf("%s%s %s %s %s",
		cursor,
		checkStyle.Render(checkbox),
		catIndicator,
		nameStyle.Width(nameWidth).Render(name),
		sizeStyle.Render(size),
	)
}

func (v *ResultsView) renderHelp() string {
	help := "j/k:navigate | space:toggle | a:all | enter:clean | esc:back | q:quit"
	return v.styles.StatusBar.Render(help)
}
