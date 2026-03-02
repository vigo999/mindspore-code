package main

import (
	"testing"

	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestParseModelRef(t *testing.T) {
	tests := []struct {
		in       string
		ok       bool
		provider string
		model    string
	}{
		{in: "openai/gpt-4o-mini", ok: true, provider: "openai", model: "gpt-4o-mini"},
		{in: "openrouter/deepseek/deepseek-r1", ok: true, provider: "openrouter", model: "deepseek/deepseek-r1"},
		{in: "openai", ok: false},
		{in: "/gpt-4o-mini", ok: false},
	}

	for _, tc := range tests {
		p, m, ok := parseModelRef(tc.in)
		if ok != tc.ok {
			t.Fatalf("parseModelRef(%q) ok=%v want %v", tc.in, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if p != tc.provider || m != tc.model {
			t.Fatalf("parseModelRef(%q) = (%q,%q), want (%q,%q)", tc.in, p, m, tc.provider, tc.model)
		}
	}
}

func TestUtilityCommandsEmitEvents(t *testing.T) {
	app := &Application{
		EventCh:    make(chan model.Event, 16),
		Permission: loop.NewPermissionManager(false, nil),
	}

	app.handleCommand("/clear")
	ev := <-app.EventCh
	if ev.Type != model.ClearChat {
		t.Fatalf("expected ClearChat, got %s", ev.Type)
	}

	app.handleCommand("/compact 5")
	ev = <-app.EventCh
	if ev.Type != model.CompactChat || ev.KeepMessages != 5 {
		t.Fatalf("expected CompactChat with keep=5, got type=%s keep=%d", ev.Type, ev.KeepMessages)
	}
	_ = <-app.EventCh // follow-up agent reply

	app.handleCommand("/exit")
	ev = <-app.EventCh
	if ev.Type != model.Done {
		t.Fatalf("expected Done, got %s", ev.Type)
	}
}

func TestPermissionCommands(t *testing.T) {
	app := &Application{
		EventCh:    make(chan model.Event, 16),
		Permission: loop.NewPermissionManager(false, nil),
	}

	// Create a pending approval.
	ok, err := app.Permission.Request("shell", "rm -rf /tmp/mscli-test", "")
	if err == nil || ok {
		t.Fatalf("expected approval required for shell")
	}

	app.handleCommand("/approve once")
	ev := <-app.EventCh
	if ev.Type != model.AgentReply || ev.Message == "" {
		t.Fatalf("unexpected approve output: %+v", ev)
	}

	app.handleCommand("/perm yolo on")
	ev = <-app.EventCh
	if ev.Type != model.AgentReply {
		t.Fatalf("unexpected yolo output: %+v", ev)
	}
}
