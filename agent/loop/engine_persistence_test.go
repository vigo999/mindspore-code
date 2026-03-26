package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

type scriptedStreamProvider struct {
	responses [][]llm.StreamChunk
	call      int
}

func (p *scriptedStreamProvider) Name() string {
	return "scripted"
}

func (p *scriptedStreamProvider) Complete(context.Context, *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, fmt.Errorf("Complete should not be called in streaming tests")
}

func (p *scriptedStreamProvider) CompleteStream(context.Context, *llm.CompletionRequest) (llm.StreamIterator, error) {
	if p.call >= len(p.responses) {
		return &captureStreamIterator{}, nil
	}
	iter := &captureStreamIterator{chunks: p.responses[p.call]}
	p.call++
	return iter, nil
}

func (p *scriptedStreamProvider) SupportsTools() bool {
	return true
}

func (p *scriptedStreamProvider) AvailableModels() []llm.ModelInfo {
	return nil
}

type fixedTool struct {
	name    string
	content string
}

func (t fixedTool) Name() string {
	return t.name
}

func (t fixedTool) Description() string {
	return "fixed test tool"
}

func (t fixedTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{Type: "object"}
}

func (t fixedTool) Execute(context.Context, json.RawMessage) (*tools.Result, error) {
	return &tools.Result{Content: t.content, Summary: "ok"}, nil
}

func TestRunPersistsUserAndStreamedAssistantBeforeUIEvents(t *testing.T) {
	provider := &scriptedStreamProvider{
		responses: [][]llm.StreamChunk{
			{
				{Content: "hel"},
				{Content: "lo", FinishReason: llm.FinishStop},
			},
		},
	}

	engine := NewEngine(EngineConfig{
		MaxIterations: 1,
		MaxTokens:     8000,
	}, provider, tools.NewRegistry())

	var trace []string
	snapshotCount := 0
	engine.SetTrajectoryRecorder(&TrajectoryRecorder{
		RecordUserInput: func(string) error {
			trace = append(trace, "record:user")
			return nil
		},
		RecordAssistantDelta: func(content string) error {
			trace = append(trace, "record:delta:"+content)
			return nil
		},
		RecordAssistant: func(content string) error {
			trace = append(trace, "record:assistant:"+content)
			return nil
		},
		PersistSnapshot: func() error {
			snapshotCount++
			trace = append(trace, fmt.Sprintf("snapshot:%d", snapshotCount))
			return nil
		},
		PersistAssistantDraftSnapshot: func(content string, _ []llm.ToolCall) error {
			trace = append(trace, "snapshot:draft:"+content)
			return nil
		},
	})

	err := engine.RunWithContextStream(context.Background(), Task{ID: "task-1", Description: "hello"}, func(ev Event) {
		trace = append(trace, "event:"+ev.Type+":"+ev.Message)
	})
	if err != nil {
		t.Fatalf("RunWithContextStream() error = %v", err)
	}

	assertTraceOrder(t, trace, "record:user", "snapshot:1", "event:UserInput:hello")
	assertTraceOrder(t, trace, "record:delta:hel", "snapshot:draft:hel", "event:AgentReplyDelta:hel")
	assertTraceOrder(t, trace, "record:assistant:hello", "snapshot:2", "event:AgentReply:hello")
}

func TestRunPersistsToolStateBeforeToolEvents(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(fixedTool{name: "read", content: "tool-output"})

	toolCall := llm.ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: llm.ToolCallFunc{
			Name:      "read",
			Arguments: json.RawMessage(`{"path":"demo.txt"}`),
		},
	}

	provider := &scriptedStreamProvider{
		responses: [][]llm.StreamChunk{
			{
				{ToolCalls: []llm.ToolCall{toolCall}, FinishReason: llm.FinishToolCalls},
			},
			{
				{Content: "done", FinishReason: llm.FinishStop},
			},
		},
	}

	engine := NewEngine(EngineConfig{
		MaxIterations: 2,
		MaxTokens:     8000,
	}, provider, registry)

	var trace []string
	snapshotCount := 0
	engine.SetTrajectoryRecorder(&TrajectoryRecorder{
		RecordUserInput: func(string) error {
			trace = append(trace, "record:user")
			return nil
		},
		RecordToolCall: func(llm.ToolCall) error {
			trace = append(trace, "record:toolcall")
			return nil
		},
		RecordToolResult: func(_ llm.ToolCall, content string) error {
			trace = append(trace, "record:toolresult:"+content)
			return nil
		},
		RecordAssistant: func(content string) error {
			trace = append(trace, "record:assistant:"+content)
			return nil
		},
		PersistSnapshot: func() error {
			snapshotCount++
			trace = append(trace, fmt.Sprintf("snapshot:%d", snapshotCount))
			return nil
		},
	})

	err := engine.RunWithContextStream(context.Background(), Task{ID: "task-2", Description: "use tool"}, func(ev Event) {
		trace = append(trace, "event:"+ev.Type+":"+ev.Message)
	})
	if err != nil {
		t.Fatalf("RunWithContextStream() error = %v", err)
	}

	assertTraceOrder(t, trace, "record:toolcall", "snapshot:2", "event:ToolCallStart:demo.txt")
	assertTraceOrder(t, trace, "record:toolresult:tool-output", "snapshot:3", "event:ToolRead:tool-output")
}

func assertTraceOrder(t *testing.T, trace []string, ordered ...string) {
	t.Helper()

	prev := -1
	prevWant := ""
	for _, want := range ordered {
		idx := indexOfTrace(trace, want)
		if idx < 0 {
			t.Fatalf("trace missing %q:\n%v", want, trace)
		}
		if idx <= prev {
			t.Fatalf("trace order violation: %q appears before %q:\n%v", want, prevWant, trace)
		}
		prev = idx
		prevWant = want
	}
}

func indexOfTrace(trace []string, want string) int {
	for i, item := range trace {
		if item == want {
			return i
		}
	}
	return -1
}
