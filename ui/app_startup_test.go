package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/mindspore-code/ui/model"
)

func TestStartupToolMessageIsTopAlignedInViewport(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type:     model.ToolSkill,
		ToolName: "mindspore-skills",
		Summary:  "shared skills repo update available: 25002f2 -> 1bef901. enter y to update or n to skip.",
	})
	app = next.(App)

	view := app.viewport.View()
	if strings.HasPrefix(view, "\n") {
		t.Fatalf("expected startup tool message to stay top-aligned, got viewport:\n%q", view)
	}
	if !strings.Contains(view, "mindspore-skills") {
		t.Fatalf("expected startup tool message in viewport, got:\n%s", view)
	}
}
