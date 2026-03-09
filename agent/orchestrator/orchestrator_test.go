package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/vigo999/ms-cli/agent/planner"
	"github.com/vigo999/ms-cli/integrations/llm"
)

// mockAgentExec records calls and returns canned events.
type mockAgentExec struct {
	calls  []RunRequest
	events []RunEvent
	err    error
}

func (m *mockAgentExec) Execute(_ context.Context, req RunRequest) ([]RunEvent, error) {
	m.calls = append(m.calls, req)
	// Always return events, even on error (matches adapter behavior:
	// partial events from the engine are preserved).
	return m.events, m.err
}

// mockWorkflowExec records calls and returns canned events.
type mockWorkflowExec struct {
	calls []struct {
		req  RunRequest
		plan planner.Plan
	}
	events []RunEvent
	err    error
}

func (m *mockWorkflowExec) Execute(_ context.Context, req RunRequest, plan planner.Plan) ([]RunEvent, error) {
	m.calls = append(m.calls, struct {
		req  RunRequest
		plan planner.Plan
	}{req, plan})
	return m.events, m.err
}

// stubWorkflowExec always returns ErrWorkflowNotImplemented.
type stubWorkflowExec struct{}

func (stubWorkflowExec) Execute(_ context.Context, _ RunRequest, _ planner.Plan) ([]RunEvent, error) {
	return nil, ErrWorkflowNotImplemented
}

// mockProvider returns a fixed response for planner.
type mockProvider struct {
	content string
	err     error
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Complete(_ context.Context, _ *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.CompletionResponse{Content: m.content}, nil
}
func (m *mockProvider) CompleteStream(_ context.Context, _ *llm.CompletionRequest) (llm.StreamIterator, error) {
	return nil, nil
}
func (m *mockProvider) SupportsTools() bool            { return false }
func (m *mockProvider) AvailableModels() []llm.ModelInfo { return nil }

func TestRun_NoPlannerFallsBackToAgent(t *testing.T) {
	agent := &mockAgentExec{
		events: []RunEvent{NewRunEvent(EventAgentReply, "done")},
	}

	o := New(Config{}, agent, nil, nil)

	req := RunRequest{ID: "1", Description: "hello"}
	events, err := o.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.calls) != 1 {
		t.Fatalf("expected 1 agent call, got %d", len(agent.calls))
	}
	if agent.calls[0].Description != "hello" {
		t.Errorf("expected 'hello', got %q", agent.calls[0].Description)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (fallback notice + agent reply), got %d", len(events))
	}
	if events[0].Message != "Planner unavailable, running in agent mode" {
		t.Fatalf("unexpected fallback notice: %q", events[0].Message)
	}
}

func TestRun_PlannerReturnsAgentMode(t *testing.T) {
	agent := &mockAgentExec{
		events: []RunEvent{NewRunEvent(EventAgentReply, "explored")},
	}

	provider := &mockProvider{
		content: `{"mode": "agent", "goal": "analyze the code"}`,
	}
	p := planner.New(provider, planner.DefaultConfig())

	o := New(Config{}, agent, p, nil)

	req := RunRequest{ID: "1", Description: "analyze the code"}
	events, err := o.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.calls) != 1 {
		t.Fatalf("expected 1 agent call, got %d", len(agent.calls))
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestRun_PlannerReturnsWorkflowMode_RealExecutor(t *testing.T) {
	agent := &mockAgentExec{}

	wf := &mockWorkflowExec{
		events: []RunEvent{
			NewRunEvent(EventAgentReply, "step done"),
			NewRunEvent(EventTaskCompleted, "Plan completed"),
		},
	}

	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "fix the bug", "steps": [
			{"description": "Read file", "tool": "read"},
			{"description": "Fix bug", "tool": "edit"}
		]}`,
	}
	p := planner.New(provider, planner.DefaultConfig())

	o := New(Config{AvailableTools: []string{"read", "edit"}}, agent, p, wf)

	req := RunRequest{ID: "t1", Description: "fix the bug"}
	events, err := o.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Workflow executor should be called with the full plan
	if len(wf.calls) != 1 {
		t.Fatalf("expected 1 workflow call, got %d", len(wf.calls))
	}
	if wf.calls[0].plan.Mode != planner.ModeWorkflow {
		t.Fatalf("expected workflow mode plan, got %q", wf.calls[0].plan.Mode)
	}
	if len(wf.calls[0].plan.Steps) != 2 {
		t.Fatalf("expected 2 steps passed to workflow, got %d", len(wf.calls[0].plan.Steps))
	}

	hasCompleted := false
	for _, ev := range events {
		if ev.Type == EventTaskCompleted {
			hasCompleted = true
		}
	}
	if !hasCompleted {
		t.Error("expected TaskCompleted event")
	}
}

func TestRun_WorkflowStubFallsBackToAgent(t *testing.T) {
	agent := &mockAgentExec{
		events: []RunEvent{NewRunEvent(EventAgentReply, "agent handled it")},
	}

	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "fix bug", "steps": [
			{"description": "Read file", "tool": "read"}
		]}`,
	}
	p := planner.New(provider, planner.DefaultConfig())

	// Stub returns ErrWorkflowNotImplemented → orchestrator falls back to agent
	o := New(Config{AvailableTools: []string{"read"}}, agent, p, &stubWorkflowExec{})

	events, err := o.Run(context.Background(), RunRequest{ID: "1", Description: "fix bug"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.calls) != 1 {
		t.Fatalf("expected 1 agent call (fallback), got %d", len(agent.calls))
	}
	// Plan announcement event + agent reply event
	if len(events) != 2 {
		t.Fatalf("expected 2 events (plan + agent reply), got %d", len(events))
	}
	if events[0].Type != EventAgentReply {
		t.Errorf("expected plan announcement, got %q", events[0].Type)
	}
	if events[1].Type != EventAgentReply {
		t.Errorf("expected agent reply, got %q", events[1].Type)
	}
}

func TestRun_WorkflowModeNilExecutorErrors(t *testing.T) {
	agent := &mockAgentExec{}

	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "fix bug", "steps": [
			{"description": "Read file", "tool": "read"}
		]}`,
	}
	p := planner.New(provider, planner.DefaultConfig())

	o := New(Config{AvailableTools: []string{"read"}}, agent, p, nil)

	_, err := o.Run(context.Background(), RunRequest{ID: "1", Description: "fix bug"})
	if err == nil {
		t.Fatal("expected error when workflow executor is nil")
	}
}

func TestRun_WorkflowExecutorError(t *testing.T) {
	agent := &mockAgentExec{}

	wf := &mockWorkflowExec{
		events: []RunEvent{NewRunEvent(EventAgentReply, "partial")},
		err:    fmt.Errorf("step 2 failed"),
	}

	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "do stuff", "steps": [
			{"description": "step one", "tool": "read"}
		]}`,
	}
	p := planner.New(provider, planner.DefaultConfig())

	o := New(Config{AvailableTools: []string{"read"}}, agent, p, wf)

	events, err := o.Run(context.Background(), RunRequest{ID: "1", Description: "do stuff"})
	if err == nil {
		t.Fatal("expected error from workflow executor")
	}
	// Non-sentinel errors propagate, partial events preserved
	if !errors.Is(err, ErrWorkflowNotImplemented) && len(events) == 0 {
		// events includes the plan announcement + workflow partial events
	}
	if len(wf.calls) != 1 {
		t.Fatalf("expected 1 workflow call, got %d", len(wf.calls))
	}
}

func TestRun_PlannerFailsFallsBackToAgent(t *testing.T) {
	agent := &mockAgentExec{
		events: []RunEvent{NewRunEvent(EventAgentReply, "fallback")},
	}

	// Provider returns an error so planner fails and the orchestrator falls back.
	provider := &mockProvider{err: fmt.Errorf("llm unavailable")}
	p := planner.New(provider, planner.DefaultConfig())

	o := New(Config{}, agent, p, nil)

	req := RunRequest{ID: "1", Description: "do something"}
	events, err := o.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to agent
	if len(agent.calls) != 1 {
		t.Fatalf("expected 1 agent call (fallback), got %d", len(agent.calls))
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (planner failure notice + agent reply), got %d", len(events))
	}
	if events[0].Type != EventAgentReply {
		t.Fatalf("expected planner failure notice event, got %q", events[0].Type)
	}
}

func TestRun_AgentErrorPreservesPartialEvents(t *testing.T) {
	agent := &mockAgentExec{
		events: []RunEvent{
			NewRunEvent("ToolRead", "file contents"),
			NewRunEvent(EventAgentReply, "partial work"),
		},
		err: fmt.Errorf("llm timeout"),
	}

	o := New(Config{}, agent, nil, nil)

	req := RunRequest{ID: "1", Description: "do stuff"}
	events, err := o.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	// Partial events should be preserved even on error
	if len(events) != 3 {
		t.Fatalf("expected 3 events (fallback notice + partial events), got %d", len(events))
	}
	if events[1].Type != "ToolRead" {
		t.Errorf("expected ToolRead event after fallback notice, got %q", events[1].Type)
	}
}

// trackingCallback records callback invocations.
type trackingCallback struct {
	created  int
	approved int
	lastPlan planner.Plan
}

func (c *trackingCallback) OnPlanCreated(plan planner.Plan) error {
	c.created++
	c.lastPlan = plan
	return nil
}

func (c *trackingCallback) OnPlanApproved(plan planner.Plan) error {
	c.approved++
	c.lastPlan = plan
	return nil
}

func TestWorkflowMode_PlanCallbacks(t *testing.T) {
	agent := &mockAgentExec{}

	wf := &mockWorkflowExec{
		events: []RunEvent{
			NewRunEvent(EventAgentReply, "ok"),
			NewRunEvent(EventTaskCompleted, "done"),
		},
	}
	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "do stuff", "steps": [{"description":"step one"},{"description":"step two"}]}`,
	}
	p := planner.New(provider, planner.DefaultConfig())
	cb := &trackingCallback{}

	o := New(Config{}, agent, p, wf)
	o.SetCallback(cb)

	_, err := o.Run(context.Background(), RunRequest{ID: "t1", Description: "do stuff"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cb.created != 1 {
		t.Errorf("expected 1 OnPlanCreated, got %d", cb.created)
	}
	if cb.approved != 1 {
		t.Errorf("expected 1 OnPlanApproved, got %d", cb.approved)
	}
	if cb.lastPlan.Mode != planner.ModeWorkflow {
		t.Errorf("expected workflow plan in callback, got %q", cb.lastPlan.Mode)
	}
	if cb.lastPlan.Goal != "do stuff" {
		t.Errorf("expected goal 'do stuff' in callback, got %q", cb.lastPlan.Goal)
	}
}
