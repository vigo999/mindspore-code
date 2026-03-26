package app

import (
	"testing"

	agentctx "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestReplayHistoryEmitsUsageSnapshotAfterBacklog(t *testing.T) {
	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		MaxTokens:     4096,
		ReserveTokens: 512,
	})
	ctxManager.SetSystemPrompt("system prompt")
	if err := ctxManager.AddMessage(llm.NewUserMessage("hello")); err != nil {
		t.Fatalf("add context message: %v", err)
	}

	expected := ctxManager.TokenUsage()
	eventCh := make(chan model.Event, 2)
	app := &Application{
		EventCh:       eventCh,
		ctxManager:    ctxManager,
		replayBacklog: []model.Event{{Type: model.UserInput, Message: "hello"}},
	}

	app.replayHistory()

	first := <-eventCh
	if first.Type != model.UserInput {
		t.Fatalf("first replay event type = %q, want %q", first.Type, model.UserInput)
	}

	second := <-eventCh
	if second.Type != model.TokenUpdate {
		t.Fatalf("second replay event type = %q, want %q", second.Type, model.TokenUpdate)
	}
	if second.CtxUsed != expected.Current {
		t.Fatalf("token update ctx used = %d, want %d", second.CtxUsed, expected.Current)
	}
	if second.CtxMax != expected.Max {
		t.Fatalf("token update ctx max = %d, want %d", second.CtxMax, expected.Max)
	}
}
