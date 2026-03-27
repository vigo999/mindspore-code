package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uirender "github.com/vigo999/mindspore-code/ui/render"
	"github.com/vigo999/mindspore-code/ui/slash"
)

var (
	sugCmdStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	sugDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	sugSelCmdStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	sugSelDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	separatorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	composerStyle   = lipgloss.NewStyle().PaddingLeft(2)
)

const (
	maxVisibleSuggestions = 8
	minComposerRows       = 1
	composerPrompt        = "❯ "
	composerContinue      = "  "
)

// TextInput wraps a multiline textarea for the chat composer.
type TextInput struct {
	Model            textarea.Model
	slashRegistry    *slash.Registry
	showSuggestions  bool
	slashMode        bool // true once suggestions have been shown, until submit/esc
	suggestions      []string
	selectedIdx      int
	suggestionOffset int
	history          []string
	historyIndex     int
	historyDraft     string
	maskedPasteRaw   string
	maskedPasteLabel string
	width            int
}

// NewTextInput creates a focused multiline composer with a prompt.
func NewTextInput() TextInput {
	ti := textarea.New()
	ti.ShowLineNumbers = false
	ti.CharLimit = 0
	ti.MaxHeight = 0
	ti.Prompt = composerPrompt
	ti.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("shift+enter", "ctrl+j"),
		key.WithHelp("shift+enter", "insert newline"),
	)
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ti.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ti.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	ti.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ti.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ti.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ti.SetPromptFunc(lipgloss.Width(composerPrompt), func(line int) string {
		if line == 0 {
			return composerPrompt
		}
		return composerContinue
	})
	ti.Focus()

	input := TextInput{
		Model:         ti,
		slashRegistry: slash.DefaultRegistry,
		historyIndex:  -1,
	}
	input.syncHeight()
	return input
}

// Value returns the current input text.
func (t TextInput) Value() string {
	value := t.Model.Value()
	if t.maskedPasteLabel == "" {
		return value
	}
	return strings.Replace(value, t.maskedPasteLabel, t.maskedPasteRaw, 1)
}

// Reset clears the input.
func (t TextInput) Reset() TextInput {
	t.Model.Reset()
	t.syncHeight()
	t.showSuggestions = false
	// Keep slashMode — it gets cleared when the command result arrives.
	t.suggestions = nil
	t.selectedIdx = 0
	t.suggestionOffset = 0
	t.historyIndex = -1
	t.historyDraft = ""
	t.maskedPasteRaw = ""
	t.maskedPasteLabel = ""
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
	t.Model.SetWidth(width)
	t.width = width
	t.syncHeight()
	return t
}

// Update handles key events.
func (t TextInput) Update(msg tea.Msg) (TextInput, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Paste {
			if summary, ok := uirender.SummarizeLargePaste(string(msg.Runes)); ok {
				t.Model.InsertString(summary)
				t.maskedPasteRaw = string(msg.Runes)
				t.maskedPasteLabel = summary
				t.syncHeight()
				t.updateSuggestions()
				return t, nil
			}
		}
		if isExplicitNewlineKey(msg) {
			t.Model.SetHeight(t.editorHeight() + 1)
		}

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
				t.syncSuggestionWindow()
				return t, nil
			case "down":
				if t.selectedIdx < len(t.suggestions)-1 {
					t.selectedIdx++
				} else {
					// Wrap to first
					t.selectedIdx = 0
				}
				t.syncSuggestionWindow()
				return t, nil
			case "tab", "enter":
				// Accept selected suggestion
				if t.selectedIdx < len(t.suggestions) {
					val := t.suggestions[t.selectedIdx] + " "
					t.Model.SetValue(val)
					t.Model.SetCursor(len(val))
					t.syncHeight()
					t.maskedPasteRaw = ""
					t.maskedPasteLabel = ""
					t.showSuggestions = false
					t.suggestions = nil
					t.suggestionOffset = 0
				}
				return t, nil
			case "esc":
				// Cancel suggestions
				t.showSuggestions = false
				t.slashMode = false
				t.suggestions = nil
				t.suggestionOffset = 0
				return t, nil
			}
		}
	}

	m, cmd := t.Model.Update(msg)
	t.Model = m
	t.syncHeight()
	if t.maskedPasteLabel != "" && !strings.Contains(t.Model.Value(), t.maskedPasteLabel) {
		t.maskedPasteRaw = ""
		t.maskedPasteLabel = ""
	}

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
	t.syncHeight()
	t.maskedPasteRaw = ""
	t.maskedPasteLabel = ""
	t.showSuggestions = false
	t.slashMode = false
	t.suggestions = nil
	t.suggestionOffset = 0
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
		t.syncHeight()
		t.maskedPasteRaw = ""
		t.maskedPasteLabel = ""
		t.showSuggestions = false
		t.slashMode = false
		t.suggestions = nil
		t.suggestionOffset = 0
		return t
	}
	t.historyIndex = -1
	t.Model.SetValue(t.historyDraft)
	t.syncHeight()
	t.maskedPasteRaw = ""
	t.maskedPasteLabel = ""
	t.historyDraft = ""
	t.showSuggestions = false
	t.slashMode = false
	t.suggestions = nil
	t.suggestionOffset = 0
	return t
}

// updateSuggestions updates the slash command suggestions based on current input.
func (t *TextInput) updateSuggestions() {
	val := t.Model.Value()
	val = strings.TrimSpace(val)

	// Only show suggestions if input starts with "/"
	if !strings.HasPrefix(val, "/") {
		t.showSuggestions = false
		t.slashMode = false
		t.suggestions = nil
		t.selectedIdx = 0
		t.suggestionOffset = 0
		return
	}

	// Get suggestions
	t.suggestions = t.slashRegistry.Suggestions(val)
	t.showSuggestions = len(t.suggestions) > 0
	if t.showSuggestions {
		t.slashMode = true
	}

	// Reset selection if it's out of bounds
	if t.selectedIdx >= len(t.suggestions) {
		t.selectedIdx = 0
	}
	if len(t.suggestions) == 0 {
		t.suggestionOffset = 0
		return
	}
	t.syncSuggestionWindow()
}

func (t TextInput) separator() string {
	width := t.width
	if width < 1 {
		width = 1
	}
	return separatorStyle.Render(strings.Repeat("─", width+4))
}

// View renders the input with optional suggestions.
func (t TextInput) View() string {
	sep := t.separator()
	inputView := composerStyle.Render(t.Model.View())

	if !t.showSuggestions || len(t.suggestions) == 0 {
		if t.slashMode {
			return sep + "\n" + inputView + strings.Repeat("\n", maxVisibleSuggestions) + "\n" + sep
		}
		return sep + "\n" + inputView + "\n" + sep
	}

	// Render suggestions below input
	var sb strings.Builder
	sb.WriteString(sep)
	sb.WriteString("\n")
	sb.WriteString(inputView)
	sb.WriteString("\n")

	start := t.suggestionOffset
	if start < 0 {
		start = 0
	}
	end := start + maxVisibleSuggestions
	if end > len(t.suggestions) {
		end = len(t.suggestions)
	}

	for i := start; i < end; i++ {
		sug := t.suggestions[i]

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

		sb.WriteString("\n")
	}
	// Pad remaining rows to fill the fixed slash suggestion area.
	rendered := end - start
	for i := rendered; i < maxVisibleSuggestions; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString(sep)

	return sb.String()
}

// Height returns the total height including suggestions area.
func (t TextInput) Height() int {
	height := t.editorHeight() + 2
	if t.slashMode {
		return height + maxVisibleSuggestions
	}
	return height
}

// IsSlashMode returns true if showing slash suggestions.
func (t TextInput) IsSlashMode() bool {
	return t.showSuggestions
}

// ClearSlashMode exits the slash suggestion reserved area.
func (t TextInput) ClearSlashMode() TextInput {
	t.slashMode = false
	t.showSuggestions = false
	t.suggestions = nil
	t.suggestionOffset = 0
	return t
}

// HasSuggestions returns true if there are visible suggestion candidates.
func (t TextInput) HasSuggestions() bool {
	return t.showSuggestions && len(t.suggestions) > 0
}

// HasPasteSummary returns true when the composer is showing a collapsed paste preview.
func (t TextInput) HasPasteSummary() bool {
	return t.maskedPasteLabel != ""
}

func isExplicitNewlineKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+j", "shift+enter":
		return true
	default:
		return false
	}
}

// CanNavigateHistory reports whether up/down should switch prompt history
// instead of moving inside the editor.
func (t TextInput) CanNavigateHistory(direction string) bool {
	row, _, lines := t.cursorPosition()
	switch direction {
	case "up":
		return row == 0
	case "down":
		return len(lines) == 0 || row == len(lines)-1
	default:
		return false
	}
}

func (t *TextInput) syncSuggestionWindow() {
	if len(t.suggestions) == 0 {
		t.suggestionOffset = 0
		return
	}

	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}
	if t.selectedIdx >= len(t.suggestions) {
		t.selectedIdx = len(t.suggestions) - 1
	}

	if t.selectedIdx < t.suggestionOffset {
		t.suggestionOffset = t.selectedIdx
	}
	if t.selectedIdx >= t.suggestionOffset+maxVisibleSuggestions {
		t.suggestionOffset = t.selectedIdx - maxVisibleSuggestions + 1
	}

	maxOffset := len(t.suggestions) - maxVisibleSuggestions
	if maxOffset < 0 {
		maxOffset = 0
	}
	if t.suggestionOffset > maxOffset {
		t.suggestionOffset = maxOffset
	}
	if t.suggestionOffset < 0 {
		t.suggestionOffset = 0
	}
}

func (t *TextInput) syncHeight() {
	t.Model.SetHeight(t.editorHeight())
}

func (t TextInput) editorHeight() int {
	lines := t.Model.LineCount()
	if lines < minComposerRows {
		return minComposerRows
	}
	return lines
}

func (t TextInput) atInputStart() bool {
	row, col, lines := t.cursorPosition()
	return row == 0 && col == 0 && len(lines) > 0
}

func (t TextInput) atInputEnd() bool {
	row, col, lines := t.cursorPosition()
	if len(lines) == 0 {
		return true
	}
	return row == len(lines)-1 && col == len([]rune(lines[row]))
}

func (t TextInput) cursorPosition() (int, int, []string) {
	lines := t.lines()
	row := t.Model.Line()
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}

	info := t.Model.LineInfo()
	col := info.StartColumn + info.ColumnOffset
	if col < 0 {
		col = 0
	}
	maxCol := len([]rune(lines[row]))
	if col > maxCol {
		col = maxCol
	}
	return row, col, lines
}

func (t TextInput) lines() []string {
	value := t.Model.Value()
	if value == "" {
		return []string{""}
	}
	return strings.Split(value, "\n")
}
