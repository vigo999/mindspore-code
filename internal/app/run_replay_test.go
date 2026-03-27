package app

import (
	"context"
	"testing"
	"time"

	agentctx "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/session"
	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestReplayHistoryEmitsUsageSnapshotAfterBacklog(t *testing.T) {
	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow: 4096,
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
	if second.CtxMax != expected.ContextWindow {
		t.Fatalf("token update ctx max = %d, want %d", second.CtxMax, expected.ContextWindow)
	}
}

func TestReplayHistoryTimelinePreservesRecordedDelays(t *testing.T) {
	previousWait := waitReplayDelay
	t.Cleanup(func() {
		waitReplayDelay = previousWait
	})

	var waits []time.Duration
	waitReplayDelay = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow: 4096,
		ReserveTokens: 512,
	})
	ctxManager.SetSystemPrompt("system prompt")
	if err := ctxManager.AddMessage(llm.NewUserMessage("hello")); err != nil {
		t.Fatalf("add user context: %v", err)
	}
	if err := ctxManager.AddMessage(llm.NewAssistantMessage("hi")); err != nil {
		t.Fatalf("add assistant context: %v", err)
	}

	t0 := time.Date(2026, time.March, 27, 12, 0, 0, 0, time.UTC)
	eventCh := make(chan model.Event, 3)
	app := &Application{
		EventCh:    eventCh,
		ctxManager: ctxManager,
		replayOnly: true,
		replayTimeline: []session.ReplayFrame{
			{
				Timestamp: t0,
				Event:     model.Event{Type: model.UserInput, Message: "hello"},
			},
			{
				Timestamp: t0.Add(200 * time.Millisecond),
				Event:     model.Event{Type: model.AgentReply, Message: "hi"},
			},
		},
	}

	app.replayHistory()

	first := <-eventCh
	if first.Type != model.UserInput {
		t.Fatalf("first event type = %q, want %q", first.Type, model.UserInput)
	}
	second := <-eventCh
	if second.Type != model.AgentReply {
		t.Fatalf("second event type = %q, want %q", second.Type, model.AgentReply)
	}
	third := <-eventCh
	if third.Type != model.TokenUpdate {
		t.Fatalf("third event type = %q, want %q", third.Type, model.TokenUpdate)
	}
	if len(waits) != 1 {
		t.Fatalf("wait count = %d, want 1", len(waits))
	}
	if waits[0] != 200*time.Millisecond {
		t.Fatalf("wait duration = %v, want %v", waits[0], 200*time.Millisecond)
	}
}

func TestReplayHistoryTimelineShowsThinkingDuringLLMWait(t *testing.T) {
	previousWait := waitReplayDelay
	t.Cleanup(func() {
		waitReplayDelay = previousWait
	})

	var waits []time.Duration
	waitReplayDelay = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	eventCh := make(chan model.Event, 3)
	t0 := time.Date(2026, time.March, 27, 12, 0, 0, 0, time.UTC)
	app := &Application{
		EventCh:    eventCh,
		replayOnly: true,
		replayTimeline: []session.ReplayFrame{
			{
				Timestamp: t0,
				Event:     model.Event{Type: model.UserInput, Message: "hello"},
			},
			{
				Timestamp: t0,
				Event:     model.Event{Type: model.AgentThinking},
			},
			{
				Timestamp: t0.Add(200 * time.Millisecond),
				Event:     model.Event{Type: model.AgentReply, Message: "hi"},
			},
		},
	}

	app.replayHistory()

	first := <-eventCh
	if first.Type != model.UserInput {
		t.Fatalf("first event type = %q, want %q", first.Type, model.UserInput)
	}
	second := <-eventCh
	if second.Type != model.AgentThinking {
		t.Fatalf("second event type = %q, want %q", second.Type, model.AgentThinking)
	}
	third := <-eventCh
	if third.Type != model.AgentReply {
		t.Fatalf("third event type = %q, want %q", third.Type, model.AgentReply)
	}
	if len(waits) != 1 {
		t.Fatalf("wait count = %d, want 1", len(waits))
	}
	if waits[0] != 200*time.Millisecond {
		t.Fatalf("wait duration = %v, want %v", waits[0], 200*time.Millisecond)
	}
}

func TestParseBootstrapConfigReplay(t *testing.T) {
	cfg, err := parseBootstrapConfig([]string{"replay", "sess_123"})
	if err != nil {
		t.Fatalf("parse replay config: %v", err)
	}
	if !cfg.Replay {
		t.Fatal("expected replay mode")
	}
	if cfg.ReplaySessionID != "sess_123" {
		t.Fatalf("replay session id = %q, want %q", cfg.ReplaySessionID, "sess_123")
	}
}

func TestLooksLikeTrajectoryPath(t *testing.T) {
	tests := []struct {
		target string
		want   bool
	}{
		{target: "sess_123", want: false},
		{target: "trajectory.json", want: true},
		{target: "trajectory.jsonl", want: true},
		{target: "./trajectory.json", want: true},
		{target: "logs/trajectory.json", want: true},
	}

	for _, tt := range tests {
		if got := looksLikeTrajectoryPath(tt.target); got != tt.want {
			t.Fatalf("looksLikeTrajectoryPath(%q) = %v, want %v", tt.target, got, tt.want)
		}
	}
}
