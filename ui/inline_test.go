package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestPrintMessage_ToolMessagesGetLeadingBlankLine(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	cmd := app.printMessage(model.Message{
		Kind:     model.MsgTool,
		ToolName: "Bash",
		ToolArgs: "$ which uv",
		Display:  model.DisplayCollapsed,
		Content:  "completed\n/Users/townwish/.local/bin/uv",
	})
	if cmd == nil {
		t.Fatal("expected non-nil print command")
	}

	msg := cmd()
	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Slice || value.Len() != 2 {
		t.Fatalf("expected sequence with blank line + tool block, got %#v", msg)
	}

	first, ok := value.Index(0).Interface().(tea.Cmd)
	if !ok {
		t.Fatalf("expected first sequence element to be tea.Cmd, got %#v", value.Index(0))
	}
	second, ok := value.Index(1).Interface().(tea.Cmd)
	if !ok {
		t.Fatalf("expected second sequence element to be tea.Cmd, got %#v", value.Index(1))
	}

	firstMsg := first()
	firstValue := reflect.ValueOf(firstMsg)
	if got := firstValue.FieldByName("messageBody").String(); got != "" {
		t.Fatalf("expected first print line to be blank, got %q", got)
	}

	secondMsg := second()
	secondValue := reflect.ValueOf(secondMsg)
	secondBody := secondValue.FieldByName("messageBody").String()
	if !strings.Contains(secondBody, "✓ Bash($ which uv)") {
		t.Fatalf("expected second print line to contain tool render, got %q", secondBody)
	}
}

func TestEnsureWaitForEvent_SequencesLocalCommandBeforeBackendEvent(t *testing.T) {
	eventCh := make(chan model.Event, 1)
	eventCh <- model.Event{Type: model.AgentReply, Message: "backend reply"}

	app := New(eventCh, make(chan string, 1), "test", ".", "", "demo-model", 4096)
	app.bootActive = false
	app.input.Model.SetValue("run tests")

	next, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	if cmd == nil {
		t.Fatal("expected non-nil command after enter")
	}

	msg := cmd()
	if got, want := reflect.TypeOf(msg).String(), "tea.sequenceMsg"; got != want {
		t.Fatalf("top-level cmd type = %s, want %s", got, want)
	}

	value := reflect.ValueOf(msg)
	if got, want := value.Len(), 2; got != want {
		t.Fatalf("top-level sequence len = %d, want %d", got, want)
	}

	first, ok := value.Index(0).Interface().(tea.Cmd)
	if !ok {
		t.Fatalf("expected first sequence element to be tea.Cmd, got %#v", value.Index(0))
	}
	second, ok := value.Index(1).Interface().(tea.Cmd)
	if !ok {
		t.Fatalf("expected second sequence element to be tea.Cmd, got %#v", value.Index(1))
	}

	firstMsg := first()
	if got, want := reflect.TypeOf(firstMsg).String(), "tea.sequenceMsg"; got != want {
		t.Fatalf("first command type = %s, want %s", got, want)
	}

	secondMsg := second()
	ev, ok := secondMsg.(model.Event)
	if !ok {
		t.Fatalf("expected second command to deliver backend event, got %#v", secondMsg)
	}
	if got, want := ev.Type, model.AgentReply; got != want {
		t.Fatalf("backend event type = %v, want %v", got, want)
	}
}
