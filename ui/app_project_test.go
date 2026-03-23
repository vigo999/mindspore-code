package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestProjectHUDUsesSharedChatSurface(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 26})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.ProjectModeOpen,
		Project: &model.ProjectStatusView{
			Name:      "ms-cli",
			Root:      "/repo/ms-cli",
			Branch:    "refactor-arch-3",
			Summary:   "1 staged · 2 modified · ahead 3",
			Dirty:     true,
			Modified:  2,
			Staged:    1,
			Untracked: 0,
			Ahead:     3,
		},
	})
	app = next.(App)

	view := app.View()
	if !strings.Contains(view, "project status") {
		t.Fatalf("expected project HUD in view, got:\n%s", view)
	}
	if strings.Contains(view, "train workspace") {
		t.Fatalf("expected project HUD to replace train HUD, got:\n%s", view)
	}
	if !strings.Contains(view, "> ") {
		t.Fatalf("expected global composer to stay visible, got:\n%s", view)
	}
	if !strings.Contains(view, "working tree") {
		t.Fatalf("expected expanded project HUD details, got:\n%s", view)
	}
	if !strings.Contains(view, "summary") {
		t.Fatalf("expected project summary block, got:\n%s", view)
	}
}

func TestProjectAndTrainHUDCanReplaceEachOther(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 26})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.TrainModeOpen,
		Train: &model.TrainEventData{
			RunID:    "primary",
			RawInput: "qwen3 lora",
			Model:    "qwen3",
			Method:   "lora",
		},
	})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.ProjectModeOpen,
		Project: &model.ProjectStatusView{
			Name:    "ms-cli",
			Root:    "/repo/ms-cli",
			Branch:  "refactor-arch-3",
			Summary: "clean working tree",
		},
	})
	app = next.(App)

	projectView := app.View()
	if !strings.Contains(projectView, "project status") {
		t.Fatalf("expected project HUD after project open, got:\n%s", projectView)
	}

	next, _ = app.handleEvent(model.Event{
		Type: model.TrainModeOpen,
		Train: &model.TrainEventData{
			RunID:    "primary",
			Model:    "qwen3",
			Method:   "lora",
			RawInput: "analyze",
		},
	})
	app = next.(App)

	trainView := app.View()
	if !strings.Contains(trainView, "train job") {
		t.Fatalf("expected train HUD after switching back, got:\n%s", trainView)
	}
	if strings.Contains(trainView, "project status") {
		t.Fatalf("expected project HUD to close when train HUD opens, got:\n%s", trainView)
	}
	if !strings.Contains(trainView, "> ") {
		t.Fatalf("expected shared composer to remain visible, got:\n%s", trainView)
	}
}
