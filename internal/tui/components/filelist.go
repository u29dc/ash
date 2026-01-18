package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/scanner"
	"ash/internal/tui"
)

// FileList renders a scrollable list of file entries.
type FileList struct {
	styles   tui.Styles
	entries  []scanner.Entry
	selected map[string]bool
	cursor   int
	offset   int
	pageSize int
	width    int
}

// NewFileList creates a new file list component.
func NewFileList(styles tui.Styles) *FileList {
	return &FileList{
		styles:   styles,
		selected: make(map[string]bool),
		pageSize: 20,
	}
}

// SetEntries sets the entries to display.
func (f *FileList) SetEntries(entries []scanner.Entry) {
	f.entries = entries
	f.cursor = 0
	f.offset = 0
}

// SetSelected sets the selection state.
func (f *FileList) SetSelected(selected map[string]bool) {
	f.selected = selected
}

// SetPageSize sets the number of visible items.
func (f *FileList) SetPageSize(size int) {
	f.pageSize = size
}

// SetWidth sets the component width.
func (f *FileList) SetWidth(width int) {
	f.width = width
}

// SetCursor sets the cursor position.
func (f *FileList) SetCursor(pos int) {
	f.cursor = pos
	f.updateOffset()
}

// MoveUp moves the cursor up.
func (f *FileList) MoveUp() {
	if f.cursor > 0 {
		f.cursor--
		f.updateOffset()
	}
}

// MoveDown moves the cursor down.
func (f *FileList) MoveDown() {
	if f.cursor < len(f.entries)-1 {
		f.cursor++
		f.updateOffset()
	}
}

// Toggle toggles selection of the current item.
func (f *FileList) Toggle() {
	if len(f.entries) == 0 {
		return
	}
	path := f.entries[f.cursor].Path
	if f.selected[path] {
		delete(f.selected, path)
	} else {
		f.selected[path] = true
	}
}

// SelectAll selects all entries.
func (f *FileList) SelectAll() {
	for i := range f.entries {
		f.selected[f.entries[i].Path] = true
	}
}

// DeselectAll deselects all entries.
func (f *FileList) DeselectAll() {
	f.selected = make(map[string]bool)
}

// Cursor returns the current cursor position.
func (f *FileList) Cursor() int {
	return f.cursor
}

// SelectedCount returns the number of selected items.
func (f *FileList) SelectedCount() int {
	return len(f.selected)
}

// SelectedSize returns the total size of selected items.
func (f *FileList) SelectedSize() int64 {
	var size int64
	for i := range f.entries {
		if f.selected[f.entries[i].Path] {
			size += f.entries[i].Size
		}
	}
	return size
}

func (f *FileList) updateOffset() {
	if f.cursor < f.offset {
		f.offset = f.cursor
	}
	if f.cursor >= f.offset+f.pageSize {
		f.offset = f.cursor - f.pageSize + 1
	}
}

// Render renders the file list.
func (f *FileList) Render() string {
	if len(f.entries) == 0 {
		return f.styles.ListItem.Render("No items found")
	}

	var b strings.Builder

	end := f.offset + f.pageSize
	if end > len(f.entries) {
		end = len(f.entries)
	}

	nameWidth := 40
	if f.width > 0 {
		nameWidth = f.width - 20 // Account for checkbox, size, padding
		if nameWidth < 20 {
			nameWidth = 20
		}
	}

	for i := f.offset; i < end; i++ {
		entry := f.entries[i]
		line := f.renderEntry(entry, i == f.cursor, nameWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(f.entries) > f.pageSize {
		indicator := fmt.Sprintf("[%d-%d of %d]", f.offset+1, end, len(f.entries))
		b.WriteString("\n")
		b.WriteString(f.styles.Subtitle.Render(indicator))
	}

	return b.String()
}

func (f *FileList) renderEntry(entry scanner.Entry, isCursor bool, nameWidth int) string {
	cursor := "  "
	if isCursor {
		cursor = "> "
	}

	checkbox := "[ ]"
	checkStyle := f.styles.Checkbox
	if f.selected[entry.Path] {
		checkbox = "[x]"
		checkStyle = f.styles.CheckboxSelected
	}

	name := entry.Name
	if len(name) > nameWidth {
		name = name[:nameWidth-3] + "..."
	}

	size := scanner.FormatSize(entry.Size)

	// Apply styles
	cursorStyle := lipgloss.NewStyle()
	nameStyle := f.styles.FileName
	sizeStyle := f.styles.FileSize

	if isCursor {
		cursorStyle = f.styles.ListItemSelected
		nameStyle = nameStyle.Background(lipgloss.Color("#262626"))
		sizeStyle = sizeStyle.Background(lipgloss.Color("#262626"))
	}

	return fmt.Sprintf("%s%s %s %s",
		cursorStyle.Render(cursor),
		checkStyle.Render(checkbox),
		nameStyle.Width(nameWidth).Render(name),
		sizeStyle.Render(size),
	)
}
