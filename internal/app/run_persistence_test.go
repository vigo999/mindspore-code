package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentctx "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/session"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestRunTaskWithoutLLMPersistsSessionBeforeReply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workDir := t.TempDir()
	runtimeSession, err := session.Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeSession.Close()
	})

	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		MaxTokens:     4096,
		ReserveTokens: 512,
	})
	ctxManager.SetSystemPrompt("system prompt")

	app := &Application{
		EventCh:    make(chan model.Event),
		llmReady:   false,
		session:    runtimeSession,
		ctxManager: ctxManager,
	}

	done := make(chan struct{})
	go func() {
		app.runTask("hello")
		close(done)
	}()

	ev := <-app.EventCh
	if ev.Type != model.AgentReply {
		t.Fatalf("event type = %q, want %q", ev.Type, model.AgentReply)
	}
	if ev.Message != provideAPIKeyFirstMsg {
		t.Fatalf("event message = %q, want %q", ev.Message, provideAPIKeyFirstMsg)
	}

	if _, err := os.Stat(runtimeSession.Path()); err != nil {
		t.Fatalf("expected trajectory to exist before reply, got %v", err)
	}
	snapshotPath := filepath.Join(filepath.Dir(runtimeSession.Path()), "snapshot.json")
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("expected snapshot to exist before reply, got %v", err)
	}

	trajectory, err := os.ReadFile(runtimeSession.Path())
	if err != nil {
		t.Fatalf("read trajectory: %v", err)
	}
	if !strings.Contains(string(trajectory), `"type":"user"`) {
		t.Fatalf("expected trajectory to contain user record, got %s", string(trajectory))
	}
	if !strings.Contains(string(trajectory), `"type":"assistant"`) {
		t.Fatalf("expected trajectory to contain assistant record, got %s", string(trajectory))
	}

	<-done
}
