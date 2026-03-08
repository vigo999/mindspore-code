package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/ui/components"
	"github.com/vigo999/ms-cli/ui/model"
	"github.com/vigo999/ms-cli/ui/slash"
)

func TestHandleKeyEnterSubmitsExactSlashCommand(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(make(chan model.Event), userCh, "v", "/tmp", "", "test-model", 1024)
	app.width = 120
	app.height = 30
	app.input.Model.SetValue("/clear")
	app.syncInputMode()
	app.updateViewport()

	if app.input.IsSlashMode() {
		t.Fatalf("expected exact slash command not to stay in suggestion mode")
	}

	_, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	select {
	case input := <-userCh:
		if input != "/clear" {
			t.Fatalf("expected /clear to be submitted, got %q", input)
		}
	default:
		t.Fatalf("expected exact slash command to be submitted on enter")
	}
}

func TestHandleTrainKeyEnterSubmitsExactNonTrainSlashCommand(t *testing.T) {
	userCh := make(chan string, 1)
	train := model.NewTrainDashboard()
	train.Status = "success"
	train.CurrentStage = "dashboard"
	train.StageStatus["dashboard"] = model.TrainStageSuccess

	app := App{
		viewMode:      viewModeTrain,
		train:         train,
		userCh:        userCh,
		trainSlash:    slash.DefaultRegistry.Without("/train"),
		width:         120,
		height:        36,
		viewport:      components.NewViewport(1, 1),
		trainViewport: components.NewViewport(1, 1),
		input:         components.NewTextInput(),
	}
	app.input.Model.SetValue("/clear")
	app.syncInputMode()
	app.updateViewport()

	if app.input.IsSlashMode() {
		t.Fatalf("expected exact slash command not to stay in suggestion mode")
	}

	next, _ := app.handleTrainKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	select {
	case input := <-userCh:
		if input != "/clear" {
			t.Fatalf("expected /clear to be submitted from train chat, got %q", input)
		}
	default:
		t.Fatalf("expected exact slash command to be submitted from train chat")
	}
}
