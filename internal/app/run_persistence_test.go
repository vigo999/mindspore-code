package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	agentctx "github.com/vigo999/mindspore-code/agent/context"
	"github.com/vigo999/mindspore-code/agent/loop"
	"github.com/vigo999/mindspore-code/agent/session"
	"github.com/vigo999/mindspore-code/integrations/llm"
	"github.com/vigo999/mindspore-code/tools"
	"github.com/vigo999/mindspore-code/ui/model"
)

type singleReplyProvider struct {
	content string
	done    bool
}

func (p *singleReplyProvider) Name() string {
	return "single-reply"
}

func (p *singleReplyProvider) Complete(context.Context, *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, io.EOF
}

func (p *singleReplyProvider) CompleteStream(context.Context, *llm.CompletionRequest) (llm.StreamIterator, error) {
	if p.done {
		return &singleReplyIterator{}, nil
	}
	p.done = true
	return &singleReplyIterator{
		chunks: []llm.StreamChunk{{
			Content:      p.content,
			FinishReason: llm.FinishStop,
		}},
	}, nil
}

func (p *singleReplyProvider) SupportsTools() bool {
	return true
}

func (p *singleReplyProvider) AvailableModels() []llm.ModelInfo {
	return nil
}

type singleReplyIterator struct {
	chunks []llm.StreamChunk
	index  int
}

func (it *singleReplyIterator) Next() (*llm.StreamChunk, error) {
	if it.index >= len(it.chunks) {
		return nil, io.EOF
	}
	chunk := it.chunks[it.index]
	it.index++
	return &chunk, nil
}

func (it *singleReplyIterator) Close() error {
	return nil
}

func TestRunTaskWithoutLLMDoesNotPersistSessionOrShowResumeHint(t *testing.T) {
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
		ContextWindow: 4096,
		ReserveTokens: 512,
	})
	ctxManager.SetSystemPrompt("system prompt")

	app := &Application{
		EventCh:    make(chan model.Event, 1),
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

	if _, err := os.Stat(runtimeSession.Path()); !os.IsNotExist(err) {
		t.Fatalf("expected no trajectory without llm reply, got %v", err)
	}
	snapshotPath := filepath.Join(filepath.Dir(runtimeSession.Path()), "snapshot.json")
	if _, err := os.Stat(snapshotPath); !os.IsNotExist(err) {
		t.Fatalf("expected no snapshot without llm reply, got %v", err)
	}
	if got := app.exitResumeHint(); got != "" {
		t.Fatalf("expected no resume hint without llm reply, got %q", got)
	}

	<-done
}

func TestRunTaskWithLLMReplyPersistsSessionAndEnablesResumeHint(t *testing.T) {
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
		ContextWindow: 4096,
		ReserveTokens: 512,
	})
	ctxManager.SetSystemPrompt("system prompt")

	provider := &singleReplyProvider{content: "hi"}
	engine := loop.NewEngine(loop.EngineConfig{
		MaxIterations: 1,
		ContextWindow: 4096,
	}, provider, tools.NewRegistry())
	engine.SetContextManager(ctxManager)
	engine.SetTrajectoryRecorder(newTrajectoryRecorder(runtimeSession, ctxManager))

	app := &Application{
		Engine:     engine,
		EventCh:    make(chan model.Event, 16),
		llmReady:   true,
		session:    runtimeSession,
		ctxManager: ctxManager,
	}

	done := make(chan struct{})
	go func() {
		app.runTask("hello")
		close(done)
	}()

	<-done

	if _, err := os.Stat(runtimeSession.Path()); err != nil {
		t.Fatalf("expected trajectory after llm reply, got %v", err)
	}
	snapshotPath := filepath.Join(filepath.Dir(runtimeSession.Path()), "snapshot.json")
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("expected snapshot after llm reply, got %v", err)
	}
	if got := app.exitResumeHint(); got == "" {
		t.Fatal("expected resume hint after llm reply")
	}
}
