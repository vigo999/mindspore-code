package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestModelPickerOpenAndEnterDispatchesModelCommand(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false
	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.ModelPickerOpen,
		Popup: &model.SelectionPopup{
			Title:    "Model Selection\nProvider: openai-completion\nURL: https://api.openai.com/v1\nModel: gpt-4o-mini\nKey: not set",
			ActionID: "model_picker",
			Options: []model.SelectionOption{
				{ID: "kimi-k2.5-free", Label: "kimi-k2.5 [free]", Desc: "anthropic · kimi-k2.5"},
			},
		},
	})
	app = next.(App)

	view := app.View()
	if !strings.Contains(view, "Model Selection") || !strings.Contains(view, "kimi-k2.5 [free]") {
		t.Fatalf("expected model picker popup in view, got:\n%s", view)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	select {
	case got := <-userCh:
		if want := "/model kimi-k2.5-free"; got != want {
			t.Fatalf("selected command = %q, want %q", got, want)
		}
	default:
		t.Fatal("expected enter to dispatch selected /model command")
	}

	if app.modelPicker != nil {
		t.Fatal("expected model picker popup to close after enter")
	}
}
