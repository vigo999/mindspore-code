package loop

import (
	"strings"
	"testing"

	ctxmanager "github.com/mindspore-lab/mindspore-cli/agent/context"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
)

func TestAddToolResultWithFallbackOnOversizedContent(t *testing.T) {
	cm := ctxmanager.NewManager(ctxmanager.ManagerConfig{
		ContextWindow:       120,
		ReserveTokens:       20,
		CompactionThreshold: 0.9,
	})

	engine := &Engine{ctxManager: cm}
	ex := &executor{engine: engine}

	oversized := strings.Repeat("x", 1000) // ~250 tokens, exceeds max usable 100
	if _, err := ex.addToolResultWithFallback("call_1", oversized); err != nil {
		t.Fatalf("addToolResultWithFallback returned error: %v", err)
	}

	msgs := cm.GetNonSystemMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected exactly one tool message after fallback, got %d", len(msgs))
	}
	if msgs[0].Role != "tool" {
		t.Fatalf("expected tool role, got %q", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, "tool result replaced due to context limit") {
		t.Fatalf("expected fallback content, got %q", msgs[0].Content)
	}
}

func TestSyncContextPromptUsageFallsBackToEstimateWhenPromptTokensMissing(t *testing.T) {
	cm := ctxmanager.NewManager(ctxmanager.ManagerConfig{
		ContextWindow:       1000,
		ReserveTokens:       20,
		CompactionThreshold: 0.9,
	})
	if err := cm.AddMessage(llm.NewUserMessage("hello")); err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	estimated := cm.TokenUsage().Current
	engine := &Engine{ctxManager: cm}
	ex := &executor{engine: engine}

	ex.syncContextPromptUsage(llm.Usage{PromptTokens: 111})
	if got := cm.TokenUsage().Current; got != 111 {
		t.Fatalf("TokenUsage().Current with provider usage = %d, want 111", got)
	}

	ex.syncContextPromptUsage(llm.Usage{})
	if got := cm.TokenUsage().Current; got != estimated {
		t.Fatalf("TokenUsage().Current after fallback = %d, want %d", got, estimated)
	}
}
