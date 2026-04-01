package app

import (
	"strings"
	"testing"

	"github.com/vigo999/mindspore-code/agent/session"
)

func TestExitResumeHintShowsSessionID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runtimeSession, err := session.Create(t.TempDir(), "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeSession.Close()
	})
	if err := runtimeSession.NoteRuntimeLLM(); err != nil {
		t.Fatalf("note runtime llm: %v", err)
	}

	app := &Application{session: runtimeSession}
	got := app.exitResumeHint()
	if !strings.Contains(got, "mscode resume "+runtimeSession.ID()) {
		t.Fatalf("expected resume hint with session id, got %q", got)
	}
}

func TestExitResumeHintEmptyWithoutSession(t *testing.T) {
	app := &Application{}
	if got := app.exitResumeHint(); got != "" {
		t.Fatalf("expected no resume hint without session, got %q", got)
	}
}

func TestExitResumeHintSkippedForReplay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runtimeSession, err := session.Create(t.TempDir(), "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeSession.Close()
	})
	if err := runtimeSession.NoteRuntimeLLM(); err != nil {
		t.Fatalf("note runtime llm: %v", err)
	}

	app := &Application{session: runtimeSession, replayOnly: true}
	if got := app.exitResumeHint(); got != "" {
		t.Fatalf("expected no resume hint for replay, got %q", got)
	}
}

func TestExitResumeHintSkippedWithoutRuntimeLLMOnResumedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workDir := t.TempDir()
	runtimeSession, err := session.Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := runtimeSession.AppendUserInput("hello"); err != nil {
		t.Fatalf("append user input: %v", err)
	}
	if err := runtimeSession.AppendAssistant("hi"); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	if err := runtimeSession.Activate(); err != nil {
		t.Fatalf("activate session: %v", err)
	}
	if err := runtimeSession.Close(); err != nil {
		t.Fatalf("close session: %v", err)
	}

	resumed, err := session.LoadByID(workDir, runtimeSession.ID())
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	t.Cleanup(func() {
		_ = resumed.Close()
	})

	app := &Application{session: resumed}
	if got := app.exitResumeHint(); got != "" {
		t.Fatalf("expected no resume hint without new runtime llm, got %q", got)
	}
}
