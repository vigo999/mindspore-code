package app

import (
	"testing"

	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestConvertLoopEventMapsUserInput(t *testing.T) {
	ev := convertLoopEvent(loop.Event{
		Type:    loop.EventUserInput,
		Message: "hello",
	})
	if ev == nil {
		t.Fatal("expected UserInput event to be converted")
	}
	if ev.Type != model.UserInput {
		t.Fatalf("converted event type = %s, want %s", ev.Type, model.UserInput)
	}
	if ev.Message != "hello" {
		t.Fatalf("converted event message = %q, want %q", ev.Message, "hello")
	}
}

func TestConvertLoopEventSkipsTaskStarted(t *testing.T) {
	if ev := convertLoopEvent(loop.Event{
		Type:    loop.EventTaskStarted,
		Message: "Task: hello",
	}); ev != nil {
		t.Fatalf("expected TaskStarted to be hidden from UI, got %+v", *ev)
	}
}
