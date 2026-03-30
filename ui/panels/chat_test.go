package panels

import (
	"strings"
	"testing"
	"time"

	"github.com/vigo999/mindspore-code/ui/model"
)

func TestRenderMessages_ToolPendingShowsOneCallLine(t *testing.T) {
	state := model.State{
		Messages: []model.Message{
			{
				Kind:     model.MsgTool,
				ToolName: "Write",
				ToolArgs: "none-1.md",
				Display:  model.DisplayCollapsed,
				Pending:  true,
			},
		},
	}

	view := RenderMessages(state, "", "", 80, true)
	if !strings.Contains(view, "⏺ Write(none-1.md)") {
		t.Fatalf("expected pending write call line, got:\n%s", view)
	}
	if strings.Contains(view, "⎿") {
		t.Fatalf("expected pending tool to not render result summary, got:\n%s", view)
	}
}

func TestRenderMessages_ToolSuccessShowsSummaryAndDetails(t *testing.T) {
	state := model.State{
		Messages: []model.Message{
			{
				Kind:     model.MsgTool,
				ToolName: "Write",
				ToolArgs: "none.md",
				Display:  model.DisplayExpanded,
				Content:  "Wrote 1 lines to none.md\n1 (No content)",
			},
		},
	}

	view := RenderMessages(state, "", "", 80, true)
	if !strings.Contains(view, "⏺ Write(none.md)") {
		t.Fatalf("expected success call line, got:\n%s", view)
	}
	if !strings.Contains(view, "⎿  Wrote 1 lines to none.md") {
		t.Fatalf("expected success summary line, got:\n%s", view)
	}
	if !strings.Contains(view, "1 (No content)") {
		t.Fatalf("expected success detail line, got:\n%s", view)
	}
}

func TestRenderMessages_ToolFailureShowsErrorSummaryAndDetails(t *testing.T) {
	state := model.State{
		Messages: []model.Message{
			{
				Kind:     model.MsgTool,
				ToolName: "Write",
				ToolArgs: "none.md",
				Display:  model.DisplayError,
				Content:  "User rejected write to none.md\n1 (No content)",
			},
		},
	}

	view := RenderMessages(state, "", "", 80, true)
	if !strings.Contains(view, "⏺ Write(none.md)") {
		t.Fatalf("expected failure call line, got:\n%s", view)
	}
	if !strings.Contains(view, "⎿  User rejected write to none.md") {
		t.Fatalf("expected failure summary line, got:\n%s", view)
	}
	if !strings.Contains(view, "1 (No content)") {
		t.Fatalf("expected failure detail line, got:\n%s", view)
	}
}

func TestRenderMessages_ToolSummaryDedupesLeadingDetailLine(t *testing.T) {
	state := model.State{
		Messages: []model.Message{
			{
				Kind:     model.MsgTool,
				ToolName: "Grep",
				ToolArgs: "needle",
				Display:  model.DisplayCollapsed,
				Summary:  "showing 2-2 of 3 matches",
				Content:  "showing 2-2 of 3 matches\na.txt:2:needle two",
			},
		},
	}

	view := RenderMessages(state, "", "", 80, true)
	if got, want := strings.Count(view, "showing 2-2 of 3 matches"), 1; got != want {
		t.Fatalf("expected deduped summary count %d, got %d in view:\n%s", want, got, view)
	}
	if !strings.Contains(view, "a.txt:2:needle two") {
		t.Fatalf("expected detail line after dedupe, got:\n%s", view)
	}
}

func TestRenderMessages_ToolPendingShowsSpinnerAndTimer(t *testing.T) {
	state := model.State{
		Messages: []model.Message{{
			Kind:     model.MsgTool,
			ToolName: "Shell",
			ToolArgs: "$ go test ./ui",
			Summary:  "running command...",
			Pending:  true,
		}},
		WaitKind:      model.WaitTool,
		WaitStartedAt: time.Now().Add(-2 * time.Second),
	}

	view := RenderMessages(state, "", "⣷", 80, true)
	if !strings.Contains(view, "⣷ Shell($ go test ./ui)") {
		t.Fatalf("expected pending spinner in tool line, got:\n%s", view)
	}
	if !strings.Contains(view, "running command... 00:0") {
		t.Fatalf("expected pending timer suffix, got:\n%s", view)
	}
}

func TestRenderMessages_ToolWarningUsesWarningSummaryStyle(t *testing.T) {
	state := model.State{
		Messages: []model.Message{{
			Kind:     model.MsgTool,
			ToolName: "Engine",
			ToolArgs: "timeout",
			Display:  model.DisplayWarning,
			Content:  "request timeout\nTry /compact",
		}},
	}

	view := RenderMessages(state, "", "", 80, true)
	if !strings.Contains(view, "⏺ Engine(timeout)") {
		t.Fatalf("expected warning call line, got:\n%s", view)
	}
	if !strings.Contains(view, "⎿  request timeout") {
		t.Fatalf("expected warning summary, got:\n%s", view)
	}
}
