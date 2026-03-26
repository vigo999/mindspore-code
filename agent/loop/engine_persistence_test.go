package loop

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

type scriptedStreamProvider struct {
	mu        sync.Mutex
	responses []*llm.CompletionResponse
}

func (p *scriptedStreamProvider) Name() string {
	return "scripted"
}

func (p *scriptedStreamProvider) Complete(context.Context, *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, io.EOF
}

func (p *scriptedStreamProvider) CompleteStream(context.Context, *llm.CompletionRequest) (llm.StreamIterator, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.responses) == 0 {
		return &scriptedStreamIterator{}, nil
	}

	resp := p.responses[0]
	p.responses = p.responses[1:]

	return &scriptedStreamIterator{
		chunks: []llm.StreamChunk{{
			Content:      resp.Content,
			ToolCalls:    append([]llm.ToolCall(nil), resp.ToolCalls...),
			FinishReason: resp.FinishReason,
			Usage:        &resp.Usage,
		}},
	}, nil
}

func (p *scriptedStreamProvider) SupportsTools() bool {
	return true
}

func (p *scriptedStreamProvider) AvailableModels() []llm.ModelInfo {
	return nil
}

type scriptedStreamIterator struct {
	chunks []llm.StreamChunk
	index  int
}

func (it *scriptedStreamIterator) Next() (*llm.StreamChunk, error) {
	if it.index >= len(it.chunks) {
		return nil, io.EOF
	}
	chunk := it.chunks[it.index]
	it.index++
	return &chunk, nil
}

func (it *scriptedStreamIterator) Close() error {
	return nil
}

type stubTool struct {
	name    string
	content string
	summary string
}

func (t stubTool) Name() string {
	return t.name
}

func (t stubTool) Description() string {
	return "stub tool"
}

func (t stubTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{Type: "object"}
}

func (t stubTool) Execute(context.Context, json.RawMessage) (*tools.Result, error) {
	return &tools.Result{Content: t.content, Summary: t.summary}, nil
}

func newPersistenceRecorder(log *[]string) *TrajectoryRecorder {
	last := ""
	appendLog := func(entry string) {
		*log = append(*log, entry)
	}

	return &TrajectoryRecorder{
		RecordUserInput: func(string) error {
			last = "user"
			appendLog(last)
			return nil
		},
		RecordAssistant: func(string) error {
			last = "assistant"
			appendLog(last)
			return nil
		},
		RecordToolCall: func(tc llm.ToolCall) error {
			last = "tool_call:" + tc.Function.Name
			appendLog(last)
			return nil
		},
		RecordToolResult: func(tc llm.ToolCall, _ string) error {
			last = "tool_result:" + tc.Function.Name
			appendLog(last)
			return nil
		},
		RecordSkillActivate: func(skillName string) error {
			last = "skill:" + skillName
			appendLog(last)
			return nil
		},
		PersistSnapshot: func() error {
			appendLog("snapshot:" + last)
			return nil
		},
	}
}

func requireOrder(t *testing.T, log []string, entries ...string) {
	t.Helper()

	next := 0
	for _, entry := range entries {
		found := -1
		for i := next; i < len(log); i++ {
			if log[i] == entry {
				found = i
				next = i + 1
				break
			}
		}
		if found == -1 {
			t.Fatalf("expected log to contain %q after index %d, got %v", entry, next, log)
		}
	}
}

func TestRunPersistsSnapshotBeforeStreamingTaskEvents(t *testing.T) {
	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{{
			Content:      "ok",
			FinishReason: llm.FinishStop,
		}},
	}
	engine := NewEngine(EngineConfig{
		MaxIterations: 1,
		MaxTokens:     4096,
	}, provider, tools.NewRegistry())

	var log []string
	engine.SetTrajectoryRecorder(newPersistenceRecorder(&log))

	err := engine.RunWithContextStream(context.Background(), Task{
		ID:          "persist-before-ui",
		Description: "say ok",
	}, func(ev Event) {
		log = append(log, "ui:"+ev.Type)
	})
	if err != nil {
		t.Fatalf("RunWithContextStream failed: %v", err)
	}

	requireOrder(t, log, "user", "snapshot:user", "ui:TaskStarted")
	requireOrder(t, log, "assistant", "snapshot:assistant", "ui:AgentReply")
}

func TestRunPersistsToolResultBeforeToolRender(t *testing.T) {
	args, err := json.Marshal(map[string]string{"path": "README.md"})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}

	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call-read-1",
					Type: "function",
					Function: llm.ToolCallFunc{
						Name:      "read",
						Arguments: args,
					},
				}},
				FinishReason: llm.FinishToolCalls,
			},
			{
				Content:      "done",
				FinishReason: llm.FinishStop,
			},
		},
	}

	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents", summary: "1 line"})

	engine := NewEngine(EngineConfig{
		MaxIterations: 2,
		MaxTokens:     4096,
	}, provider, registry)

	var log []string
	engine.SetTrajectoryRecorder(newPersistenceRecorder(&log))

	err = engine.RunWithContextStream(context.Background(), Task{
		ID:          "persist-tool-result",
		Description: "read the file",
	}, func(ev Event) {
		log = append(log, "ui:"+ev.Type)
	})
	if err != nil {
		t.Fatalf("RunWithContextStream failed: %v", err)
	}

	requireOrder(t, log, "tool_call:read", "snapshot:tool_call:read", "ui:ToolCallStart")
	requireOrder(t, log, "tool_result:read", "snapshot:tool_result:read", "ui:ToolRead")
}

func TestRunPersistsToolErrorBeforeErrorRender(t *testing.T) {
	args, err := json.Marshal(map[string]string{"path": "missing.txt"})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}

	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call-missing-1",
					Type: "function",
					Function: llm.ToolCallFunc{
						Name:      "missing_tool",
						Arguments: args,
					},
				}},
				FinishReason: llm.FinishToolCalls,
			},
			{
				Content:      "done",
				FinishReason: llm.FinishStop,
			},
		},
	}

	engine := NewEngine(EngineConfig{
		MaxIterations: 2,
		MaxTokens:     4096,
	}, provider, tools.NewRegistry())

	var log []string
	engine.SetTrajectoryRecorder(newPersistenceRecorder(&log))

	err = engine.RunWithContextStream(context.Background(), Task{
		ID:          "persist-tool-error",
		Description: "use the missing tool",
	}, func(ev Event) {
		log = append(log, "ui:"+ev.Type)
	})
	if err != nil {
		t.Fatalf("RunWithContextStream failed: %v", err)
	}

	requireOrder(t, log, "tool_call:missing_tool", "snapshot:tool_call:missing_tool", "ui:ToolCallStart")
	requireOrder(t, log, "tool_result:missing_tool", "snapshot:tool_result:missing_tool", "ui:ToolError")
}
