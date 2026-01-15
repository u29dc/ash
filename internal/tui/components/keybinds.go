package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ash/internal/tui"
)

// KeyBind represents a single key binding.
type KeyBind struct {
	Key         string
	Description string
}

// KeyBinds renders a help footer with key bindings.
type KeyBinds struct {
	styles   tui.Styles
	bindings []KeyBind
	width    int
}

// NewKeyBinds creates a new key bindings component.
func NewKeyBinds(styles tui.Styles) *KeyBinds {
	return &KeyBinds{
		styles: styles,
	}
}

// SetBindings sets the key bindings to display.
func (k *KeyBinds) SetBindings(bindings []KeyBind) {
	k.bindings = bindings
}

// SetWidth sets the component width.
func (k *KeyBinds) SetWidth(width int) {
	k.width = width
}

// Render renders the key bindings.
func (k *KeyBinds) Render() string {
	if len(k.bindings) == 0 {
		return ""
	}

	var parts []string
	for _, bind := range k.bindings {
		key := k.styles.KeyBind.Render(bind.Key)
		desc := k.styles.KeyBindDesc.Render(bind.Description)
		parts = append(parts, key+" "+desc)
	}

	return strings.Join(parts, "  ")
}

// RenderCompact renders key bindings in a compact format.
func (k *KeyBinds) RenderCompact() string {
	if len(k.bindings) == 0 {
		return ""
	}

	var parts []string
	for _, bind := range k.bindings {
		parts = append(parts, bind.Key+":"+bind.Description)
	}

	return k.styles.StatusBar.Render(strings.Join(parts, " | "))
}

// RenderVertical renders key bindings vertically.
func (k *KeyBinds) RenderVertical() string {
	if len(k.bindings) == 0 {
		return ""
	}

	var b strings.Builder
	for _, bind := range k.bindings {
		key := k.styles.KeyBind.Render(bind.Key)
		desc := k.styles.KeyBindDesc.Render(bind.Description)
		b.WriteString(key + " " + desc + "\n")
	}

	return b.String()
}

// CommonBindings returns common key bindings.
func CommonBindings() []KeyBind {
	return []KeyBind{
		{Key: "q", Description: "quit"},
		{Key: "?", Description: "help"},
	}
}

// NavigationBindings returns navigation key bindings.
func NavigationBindings() []KeyBind {
	return []KeyBind{
		{Key: "j/k", Description: "navigate"},
		{Key: "enter", Description: "select"},
		{Key: "esc", Description: "back"},
	}
}

// SelectionBindings returns selection key bindings.
func SelectionBindings() []KeyBind {
	return []KeyBind{
		{Key: "space", Description: "toggle"},
		{Key: "a", Description: "select all"},
	}
}

// RenderFooter renders a complete footer with bindings.
func RenderFooter(styles tui.Styles, bindings []KeyBind, width int) string {
	kb := NewKeyBinds(styles)
	kb.SetBindings(bindings)
	kb.SetWidth(width)

	content := kb.RenderCompact()

	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color("#404040")).
		Render(content)
}
