package app

import (
	"strings"
	"testing"

	agentctx "github.com/mindspore-lab/mindspore-cli/agent/context"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestCmdCtxShowsProviderBackedSource(t *testing.T) {
	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow: 1000,
		ReserveTokens: 100,
	})
	if err := ctxManager.AddMessage(llm.NewUserMessage("hello")); err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}
	ctxManager.SetPromptTokenUsage("openai-responses", 120)
	if err := ctxManager.AddMessage(llm.NewAssistantMessage("ok")); err != nil {
		t.Fatalf("AddMessage assistant failed: %v", err)
	}

	app := newModelCommandTestApp()
	app.ctxManager = ctxManager

	app.cmdCtx()

	ev := drainUntilEventType(t, app, model.TokenUpdate)
	reply := drainUntilEventType(t, app, model.AgentReply)

	if got, want := ev.CtxUsed, ctxManager.TokenUsage().Current; got != want {
		t.Fatalf("TokenUpdate.CtxUsed = %d, want %d", got, want)
	}
	if !strings.Contains(reply.Message, "openai-responses API prompt tokens + local delta") {
		t.Fatalf("reply message = %q, want provider source", reply.Message)
	}
	if !strings.Contains(reply.Message, "Provider prompt tokens: 120") {
		t.Fatalf("reply message = %q, want provider token count", reply.Message)
	}
}

func TestCmdCtxShowsLocalEstimateFallback(t *testing.T) {
	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow: 1000,
		ReserveTokens: 100,
	})
	if err := ctxManager.AddMessage(llm.NewUserMessage("hello")); err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	app := newModelCommandTestApp()
	app.ctxManager = ctxManager

	app.cmdCtx()

	reply := drainUntilEventType(t, app, model.AgentReply)
	if !strings.Contains(reply.Message, "local heuristic estimate fallback") {
		t.Fatalf("reply message = %q, want local estimate source", reply.Message)
	}
	if !strings.Contains(reply.Message, "Method: utf8 chars / 4 plus message/tool-call overhead") {
		t.Fatalf("reply message = %q, want local estimate method", reply.Message)
	}
}
