package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/slash"
)

var (
	sugCmdStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	sugDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	sugSelCmdStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	sugSelDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
)

// TextInput wraps the bubbles text input for the chat prompt.
type TextInput struct {
	Model           textinput.Model
	slashRegistry   *slash.Registry
	showSuggestions bool
	suggestions     []string
	selectedIdx     int
	history         []string
	historyIndex    int
	historyDraft    string
}

// NewTextInput creates a focused text input with "> " prompt.
func NewTextInput() TextInput {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 2000
	return TextInput{
		Model:         ti,
		slashRegistry: slash.DefaultRegistry,
		historyIndex:  -1,
	}
}

// Value returns the current input text.
func (t TextInput) Value() string {
	return t.Model.Value()
}

// Reset clears the input.
func (t TextInput) Reset() TextInput {
	t.Model.Reset()
	t.showSuggestions = false
	t.suggestions = nil
	t.selectedIdx = 0
	t.historyIndex = -1
	t.historyDraft = ""
	return t
}

// Focus gives the input focus.
func (t TextInput) Focus() (TextInput, tea.Cmd) {
	cmd := t.Model.Focus()
	return t, cmd
}

// Blur removes focus from the input.
func (t TextInput) Blur() TextInput {
	t.Model.Blur()
	return t
}

// SetWidth updates the rendered input width.
func (t TextInput) SetWidth(width int) TextInput {
	if width < 1 {
		width = 1
	}
	t.Model.Width = width
	return t
}

// Update handles key events.
func (t TextInput) Update(msg tea.Msg) (TextInput, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle slash command suggestions navigation
		if t.showSuggestions && len(t.suggestions) > 0 {
			switch msg.String() {
			case "up":
				if t.selectedIdx > 0 {
					t.selectedIdx--
				} else {
					// Wrap to last
					t.selectedIdx = len(t.suggestions) - 1
				}
				return t, nil
			case "down":
				if t.selectedIdx < len(t.suggestions)-1 {
					t.selectedIdx++
				} else {
					// Wrap to first
					t.selectedIdx = 0
				}
				return t, nil
			case "tab", "enter":
				// Accept selected suggestion
				if t.selectedIdx < len(t.suggestions) {
					val := t.suggestions[t.selectedIdx] + " "
					t.Model.SetValue(val)
					t.Model.SetCursor(len(val))
					t.showSuggestions = false
					t.suggestions = nil
				}
				return t, nil
			case "esc":
				// Cancel suggestions
				t.showSuggestions = false
				t.suggestions = nil
				return t, nil
			}
		}
	}

	m, cmd := t.Model.Update(msg)
	t.Model = m

	// Update suggestions based on current input
	t.updateSuggestions()

	return t, cmd
}

// PushHistory stores a submitted input line for later up/down recall.
func (t TextInput) PushHistory(value string) TextInput {
	value = strings.TrimSpace(value)
	if value == "" {
		return t
	}
	if n := len(t.history); n > 0 && t.history[n-1] == value {
		t.historyIndex = -1
		t.historyDraft = ""
		return t
	}
	t.history = append(t.history, value)
	t.historyIndex = -1
	t.historyDraft = ""
	return t
}

// PrevHistory recalls the previous submitted line.
func (t TextInput) PrevHistory() TextInput {
	if len(t.history) == 0 {
		return t
	}
	if t.historyIndex == -1 {
		t.historyDraft = t.Model.Value()
		t.historyIndex = len(t.history) - 1
	} else if t.historyIndex > 0 {
		t.historyIndex--
	}
	t.Model.SetValue(t.history[t.historyIndex])
	t.Model.SetCursor(len(t.history[t.historyIndex]))
	t.updateSuggestions()
	return t
}

// NextHistory moves forward in submitted-line history, restoring the draft at the end.
func (t TextInput) NextHistory() TextInput {
	if len(t.history) == 0 || t.historyIndex == -1 {
		return t
	}
	if t.historyIndex < len(t.history)-1 {
		t.historyIndex++
		t.Model.SetValue(t.history[t.historyIndex])
		t.Model.SetCursor(len(t.history[t.historyIndex]))
		t.updateSuggestions()
		return t
	}
	t.historyIndex = -1
	t.Model.SetValue(t.historyDraft)
	t.Model.SetCursor(len(t.historyDraft))
	t.historyDraft = ""
	t.updateSuggestions()
	return t
}

// updateSuggestions updates the slash command suggestions based on current input.
func (t *TextInput) updateSuggestions() {
	val := t.Model.Value()
	val = strings.TrimSpace(val)

	// Only show suggestions if input starts with "/"
	if !strings.HasPrefix(val, "/") {
		t.showSuggestions = false
		t.suggestions = nil
		t.selectedIdx = 0
		return
	}

	// Get suggestions
	t.suggestions = t.slashRegistry.Suggestions(val)
	t.showSuggestions = len(t.suggestions) > 0

	// Reset selection if it's out of bounds
	if t.selectedIdx >= len(t.suggestions) {
		t.selectedIdx = 0
	}
}

// View renders the input with optional suggestions.
func (t TextInput) View() string {
	inputView := t.Model.View()

	if !t.showSuggestions || len(t.suggestions) == 0 {
		return inputView
	}

	// Render suggestions below input
	var sb strings.Builder
	sb.WriteString(inputView)
	sb.WriteString("\n")

	for i, sug := range t.suggestions {
		if i >= 8 { // Limit to 8 suggestions
			break
		}

		// Get command description
		cmd, ok := t.slashRegistry.Get(sug)
		if !ok {
			continue
		}

		if i == t.selectedIdx {
			sb.WriteString("    ")
			sb.WriteString(sugSelCmdStyle.Render(sug))
			sb.WriteString("  ")
			sb.WriteString(sugSelDescStyle.Render(cmd.Description))
		} else {
			sb.WriteString("    ")
			sb.WriteString(sugCmdStyle.Render(sug))
			sb.WriteString("  ")
			sb.WriteString(sugDescStyle.Render(cmd.Description))
		}

		if i < len(t.suggestions)-1 && i < 7 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Height returns the total height including suggestions.
func (t TextInput) Height() int {
	if !t.showSuggestions || len(t.suggestions) == 0 {
		return 1
	}

	height := 1 // Input line
	if len(t.suggestions) > 8 {
		height += 8
	} else {
		height += len(t.suggestions)
	}
	return height
}

// IsSlashMode returns true if showing slash suggestions.
func (t TextInput) IsSlashMode() bool {
	return t.showSuggestions
}
