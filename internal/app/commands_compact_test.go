package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentctx "github.com/mindspore-lab/mindspore-cli/agent/context"
	"github.com/mindspore-lab/mindspore-cli/agent/loop"
	"github.com/mindspore-lab/mindspore-cli/agent/session"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestCmdCompactCompactsContextAndEmitsTokenUpdate(t *testing.T) {
	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow:       100,
		ReserveTokens:       10,
		CompactionThreshold: 0.9,
		EnableSmartCompact:  false,
	})
	for i := 0; i < 3; i++ {
		if err := ctxManager.AddMessage(llm.NewUserMessage(strings.Repeat("x", 80))); err != nil {
			t.Fatalf("AddMessage #%d failed: %v", i+1, err)
		}
	}

	before := ctxManager.TokenUsage().Current
	app := newModelCommandTestApp()
	app.ctxManager = ctxManager

	app.cmdCompact()

	drainUntilEventType(t, app, model.AgentThinking)
	notice := drainUntilEventType(t, app, model.ContextNotice)
	ev := drainUntilEventType(t, app, model.TokenUpdate)
	reply := drainUntilEventType(t, app, model.AgentReply)

	if got, want := notice.Message, loop.ContextCompactStartMessage; got != want {
		t.Fatalf("context notice = %q, want %q", got, want)
	}
	if got := ctxManager.TokenUsage().Current; got > before/2 {
		t.Fatalf("context usage after cmdCompact = %d, want <= %d", got, before/2)
	}
	if got, want := ev.CtxUsed, ctxManager.TokenUsage().Current; got != want {
		t.Fatalf("TokenUpdate.CtxUsed = %d, want %d", got, want)
	}
	if got, want := ev.CtxMax, ctxManager.TokenUsage().ContextWindow; got != want {
		t.Fatalf("TokenUpdate.CtxMax = %d, want %d", got, want)
	}
	if !strings.Contains(reply.Message, "Context compacted:") {
		t.Fatalf("reply message = %q, want compaction summary", reply.Message)
	}
}

func TestCmdCompactDebugDumpsPreCompactSnapshot(t *testing.T) {
	workDir := t.TempDir()
	runtimeSession, err := session.Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeSession.Close()
	})

	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow:       100,
		ReserveTokens:       10,
		CompactionThreshold: 0.9,
		EnableSmartCompact:  false,
	})
	ctxManager.SetSystemPrompt("system prompt")
	for i := 0; i < 3; i++ {
		if err := ctxManager.AddMessage(llm.NewUserMessage("before compact " + strings.Repeat("x", 70))); err != nil {
			t.Fatalf("AddMessage #%d failed: %v", i+1, err)
		}
	}

	app := newModelCommandTestApp()
	app.WorkDir = workDir
	app.session = runtimeSession
	app.ctxManager = ctxManager
	app.llmDebugDumper = llm.NewDebugDumper(filepath.Dir(runtimeSession.Path()))
	ctxManager.SetPreCompactSnapshotHook(app.dumpPreCompactSnapshot)

	app.cmdCompact()

	drainUntilEventType(t, app, model.AgentThinking)
	notice := drainUntilEventType(t, app, model.ContextNotice)
	drainUntilEventType(t, app, model.TokenUpdate)
	drainUntilEventType(t, app, model.AgentReply)

	if got, want := notice.Message, loop.ContextCompactStartMessage; got != want {
		t.Fatalf("context notice = %q, want %q", got, want)
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(runtimeSession.Path()), "snapshot.compact-pre-manual-*.json"))
	if err != nil {
		t.Fatalf("glob pre-compact snapshots: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("pre-compact snapshot count = %d, want 1", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read pre-compact snapshot: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"system_prompt": "system prompt"`) {
		t.Fatalf("pre-compact snapshot missing system prompt:\n%s", text)
	}
	if !strings.Contains(text, "before compact") {
		t.Fatalf("pre-compact snapshot missing original messages:\n%s", text)
	}

	loaded, err := session.LoadReplayPath(runtimeSession.Path())
	if err != nil {
		t.Fatalf("load compact trajectory: %v", err)
	}
	t.Cleanup(func() {
		_ = loaded.Close()
	})
	replay := loaded.ReplayEvents()
	if len(replay) != 1 {
		t.Fatalf("replay event count = %d, want 1", len(replay))
	}
	if got := replay[0].Type; got != model.ContextNotice {
		t.Fatalf("compact replay event type = %q, want %q", got, model.ContextNotice)
	}
	if !strings.Contains(replay[0].Message, "Context compacted:") {
		t.Fatalf("compact replay message = %q, want compaction summary", replay[0].Message)
	}
}
