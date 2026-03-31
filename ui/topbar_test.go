package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/mindspore-code/ui/model"
)

func TestViewShowsTopBarTitle(t *testing.T) {
	app := New(nil, nil, "MindSpore CLI. test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	app = next.(App)

	view := app.View()
	if !strings.Contains(view, "MindSpore CLI. test") {
		t.Fatalf("expected top bar title in view, got:\n%s", view)
	}
}

func TestInlineModeOmitsPersistentTopBarAndViewportFill(t *testing.T) {
	app := New(nil, nil, "MindSpore CLI. test", ".", "", "demo-model", 4096).WithInlineMode()
	app.bootActive = false
	app.state = app.state.WithMessage(model.Message{Kind: model.MsgAgent, Content: "history line"})

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	app = next.(App)

	view := app.View()
	if strings.Contains(view, "MindSpore CLI. test") {
		t.Fatalf("expected inline mode to omit persistent top bar, got:\n%s", view)
	}
	if strings.Contains(view, "history line") {
		t.Fatalf("expected inline mode history to stay out of the live frame, got:\n%s", view)
	}
	if got := strings.Count(view, "\n") + 1; got >= 24 {
		t.Fatalf("expected inline mode view to avoid filling terminal height, got %d lines", got)
	}
}

func TestRenderInlineBannerIncludesMetadata(t *testing.T) {
	banner := RenderInlineBanner("MindSpore Code. test", "/tmp/project", "github.com/vigo999/mindspore-code", "demo-model", 4096)
	for _, want := range []string{
		"MindSpore Code",
		"inline mode",
		"demo-model",
		"/tmp/project",
		"4096 tokens",
	} {
		if !strings.Contains(banner, want) {
			t.Fatalf("expected banner to include %q, got:\n%s", want, banner)
		}
	}
	if !strings.Contains(banner, "Model: demo-model") {
		t.Fatalf("expected banner rows to stay left aligned, got:\n%s", banner)
	}
	if strings.Contains(banner, "Version: MindSpore Code. test\n│                                           │\n│  Model: demo-model") {
		t.Fatalf("expected banner rows to render without per-row blank spacing, got:\n%s", banner)
	}
}
