package components

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/mindspore-code/ui/slash"
)

const largePastedBlock = "line 01\nline 02\nline 03\nline 04\nline 05\nline 06\nline 07\nline 08\n"

func TestTextInputHistoryRecall(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("first prompt")
	input = input.PushHistory("second prompt")
	input.Model.SetValue("draft")
	input.Model.SetCursor(len("draft"))

	input = input.PrevHistory()
	if got := input.Value(); got != "second prompt" {
		t.Fatalf("expected latest history entry, got %q", got)
	}

	input = input.PrevHistory()
	if got := input.Value(); got != "first prompt" {
		t.Fatalf("expected previous history entry, got %q", got)
	}

	input = input.NextHistory()
	if got := input.Value(); got != "second prompt" {
		t.Fatalf("expected forward history entry, got %q", got)
	}

	input = input.NextHistory()
	if got := input.Value(); got != "draft" {
		t.Fatalf("expected draft restoration after leaving history, got %q", got)
	}
}

func TestTextInputHistoryDoesNotBreakSlashSuggestions(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("/project")
	var cmd tea.Cmd
	input, cmd = input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	_ = cmd
	if !input.IsSlashMode() {
		t.Fatal("expected slash suggestions after typing slash")
	}
}

func TestTextInputCtrlJInsertsNewline(t *testing.T) {
	input := NewTextInput()
	if got := input.Height(); got != 3 {
		t.Fatalf("expected single-row composer block height 3, got %d", got)
	}

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if got := input.Value(); got != "\n" {
		t.Fatalf("expected ctrl+j to insert newline, got %q", got)
	}
	if got := input.Height(); got != 4 {
		t.Fatalf("expected two-row composer block height 4, got %d", got)
	}

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})

	if got := input.Value(); got != "\n\n" {
		t.Fatalf("expected second ctrl+j to insert another newline, got %q", got)
	}
	if got := input.Height(); got != 5 {
		t.Fatalf("expected three-row composer block height 5, got %d", got)
	}
}

func TestTextInputUsesSinglePromptWithContinuationLines(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(40)
	input.Model.SetValue("one\ntwo\nthree")
	input.syncHeight()

	view := input.View()
	if got := strings.Count(view, composerPrompt); got != 1 {
		t.Fatalf("expected one primary prompt, got %d in view %q", got, view)
	}
	if got := strings.Count(view, composerContinue+"two"); got != 1 {
		t.Fatalf("expected continuation prompt for second line, got view %q", view)
	}
	if got := strings.Count(view, composerContinue+"three"); got != 1 {
		t.Fatalf("expected continuation prompt for third line, got view %q", view)
	}
}

func TestTextInputKeepsFirstLineVisibleAfterExplicitNewlineGrowth(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(40)
	input.Model.SetValue("alpha")

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("beta")})

	view := input.View()
	if !strings.Contains(view, composerPrompt+"alpha") {
		t.Fatalf("expected first line to remain visible after newline growth, got %q", view)
	}
	if !strings.Contains(view, composerContinue+"beta") {
		t.Fatalf("expected second line to remain visible after newline growth, got %q", view)
	}
}

func TestTextInputHeightGrowsForSoftWrappedLine(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(11)
	input.Model.SetValue("alpha beta")
	input.syncHeight()

	if got := input.Height(); got != 4 {
		t.Fatalf("expected wrapped two-row composer block height 4, got %d", got)
	}

	view := input.View()
	if !strings.Contains(view, composerPrompt+"alpha ") {
		t.Fatalf("expected first wrapped row in view, got %q", view)
	}
	if !strings.Contains(view, composerContinue+"beta") {
		t.Fatalf("expected second wrapped row in view, got %q", view)
	}
}

func TestTextInputKeepsFirstWrappedLineVisibleAfterSoftWrapGrowth(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(11)

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alpha beta")})

	view := input.View()
	if !strings.Contains(view, composerPrompt+"alpha ") {
		t.Fatalf("expected first wrapped row to remain visible after typing growth, got %q", view)
	}
	if !strings.Contains(view, composerContinue+"beta") {
		t.Fatalf("expected second wrapped row after typing growth, got %q", view)
	}
}

func TestTextInputPasteShowsFullContent(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(60)

	input, _ = input.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(largePastedBlock),
		Paste: true,
	})

	if got := input.Value(); got != largePastedBlock {
		t.Fatalf("expected pasted content to be stored verbatim, got %q", got)
	}

	view := input.View()
	if !strings.Contains(view, "line 01") {
		t.Fatalf("expected pasted content to stay visible in view, got %q", view)
	}
	if !strings.Contains(view, "line 07") {
		t.Fatalf("expected later pasted lines to stay visible in view, got %q", view)
	}
}

func TestTextInputPasteKeepsAllRowsVisibleAfterGrowth(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(24)

	input, _ = input.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("line 01\nline 02\nline 03\nline 04"),
		Paste: true,
	})

	view := input.View()
	for _, want := range []string{"line 01", "line 02", "line 03", "line 04"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected pasted line %q to stay visible after paste growth, got %q", want, view)
		}
	}
}

func TestTextInputHistoryRecallOfSlashCommandDoesNotReopenSuggestions(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("/project")
	input = input.PushHistory("hello")

	input = input.PrevHistory()
	if got := input.Value(); got != "hello" {
		t.Fatalf("expected latest history entry, got %q", got)
	}

	input = input.PrevHistory()
	if got := input.Value(); got != "/project" {
		t.Fatalf("expected slash command from history, got %q", got)
	}
	if input.IsSlashMode() {
		t.Fatal("expected slash suggestions to stay closed while browsing history")
	}

	input = input.NextHistory()
	if got := input.Value(); got != "hello" {
		t.Fatalf("expected down to continue history recall even in slash mode, got %q", got)
	}
}

func TestTextInputHistoryOnlyTriggersAtEditorBoundaries(t *testing.T) {
	input := NewTextInput()
	input.Model.SetValue("first line\nsecond line")
	input.syncHeight()

	if input.CanNavigateHistory("up") {
		t.Fatal("expected up to stay inside multiline editor while on the last line")
	}

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyUp})

	if !input.CanNavigateHistory("up") {
		t.Fatal("expected up history at the top boundary of the editor")
	}
	if input.CanNavigateHistory("down") {
		t.Fatal("expected down history to stay disabled away from the bottom boundary")
	}

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyDown})

	if !input.CanNavigateHistory("down") {
		t.Fatal("expected down history at the bottom boundary of multiline input")
	}
}

func TestTextInputSuggestionsScrollDownToKeepSelectionVisible(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 7
	input.suggestionOffset = 0

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyDown})

	if input.selectedIdx != 8 {
		t.Fatalf("expected selected index 8, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 1 {
		t.Fatalf("expected suggestion offset 1, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd08") {
		t.Fatalf("expected view to include newly selected command, got %q", view)
	}
	if strings.Contains(view, "/cmd00") {
		t.Fatalf("expected first command to scroll out of view, got %q", view)
	}
}

func TestTextInputSuggestionsScrollUpToKeepSelectionVisible(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 2
	input.suggestionOffset = 2

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyUp})

	if input.selectedIdx != 1 {
		t.Fatalf("expected selected index 1, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 1 {
		t.Fatalf("expected suggestion offset 1, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd01") {
		t.Fatalf("expected view to include newly selected command, got %q", view)
	}
	if strings.Contains(view, "/cmd09") {
		t.Fatalf("expected last command to scroll out of view, got %q", view)
	}
}

func TestTextInputSuggestionsWrapDownToTopPage(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 9
	input.suggestionOffset = 2

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyDown})

	if input.selectedIdx != 0 {
		t.Fatalf("expected selected index 0 after wrap, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 0 {
		t.Fatalf("expected suggestion offset 0 after wrap, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd00") {
		t.Fatalf("expected wrapped view to include first command, got %q", view)
	}
	if strings.Contains(view, "/cmd09") {
		t.Fatalf("expected last command to leave view after wrapping to top, got %q", view)
	}
}

func TestTextInputSuggestionsWrapUpToLastPage(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 0
	input.suggestionOffset = 0

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyUp})

	if input.selectedIdx != 9 {
		t.Fatalf("expected selected index 9 after wrap, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 2 {
		t.Fatalf("expected suggestion offset 2 after wrap, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd09") {
		t.Fatalf("expected wrapped view to include last command, got %q", view)
	}
	if strings.Contains(view, "/cmd00") {
		t.Fatalf("expected first command to leave view after wrapping to bottom, got %q", view)
	}
}

func newSlashSuggestionInput(count int) TextInput {
	input := NewTextInput()
	registry := slash.NewRegistry()
	input.slashRegistry = registry
	input.showSuggestions = true
	input.slashMode = true
	input.suggestions = make([]string, 0, count)

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("/cmd%02d", i)
		registry.Register(slash.Command{
			Name:        name,
			Description: fmt.Sprintf("Command %02d", i),
			Usage:       name,
		})
		input.suggestions = append(input.suggestions, name)
	}

	return input
}
