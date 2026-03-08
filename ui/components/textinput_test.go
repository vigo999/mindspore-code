package components

import "testing"

func TestTextInputShowsSuggestionsForPartialSlashCommand(t *testing.T) {
	input := NewTextInput()
	input.Model.SetValue("/cl")
	input.updateSuggestions()

	if !input.IsSlashMode() {
		t.Fatalf("expected partial slash command to show suggestions")
	}
	if len(input.suggestions) == 0 {
		t.Fatalf("expected at least one slash suggestion")
	}
}

func TestTextInputHidesSuggestionsForExactSlashCommand(t *testing.T) {
	input := NewTextInput()
	input.Model.SetValue("/clear")
	input.updateSuggestions()

	if input.IsSlashMode() {
		t.Fatalf("expected exact slash command to bypass suggestions and submit on enter")
	}
}

func TestTextInputHidesSuggestionsAfterSlashArgsBegin(t *testing.T) {
	input := NewTextInput()
	input.Model.SetValue("/model ")
	input.updateSuggestions()

	if input.IsSlashMode() {
		t.Fatalf("expected slash suggestions to hide once argument entry begins")
	}
}
