package planner

import (
	"context"
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// mockProvider returns a fixed response.
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
func (m *mockProvider) SupportsTools() bool          { return false }
func (m *mockProvider) AvailableModels() []llm.ModelInfo { return nil }

func TestPlan_AgentMode(t *testing.T) {
	provider := &mockProvider{
		content: `{"mode": "agent", "goal": "analyze the codebase"}`,
	}
	p := New(provider, DefaultConfig())
	plan, err := p.Plan(context.Background(), "analyze the codebase", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeAgent {
		t.Errorf("expected agent mode, got %q", plan.Mode)
	}
	if plan.Goal != "analyze the codebase" {
		t.Errorf("expected goal 'analyze the codebase', got %q", plan.Goal)
	}
}

func TestPlan_WorkflowMode(t *testing.T) {
	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "fix the bug", "steps": [
			{"description":"Read the file","tool":"read"},
			{"description":"Edit the code","tool":"edit"}
		]}`,
	}
	p := New(provider, DefaultConfig())
	plan, err := p.Plan(context.Background(), "fix the bug", []string{"read", "edit", "shell"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeWorkflow {
		t.Errorf("expected workflow mode, got %q", plan.Mode)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Tool != "read" {
		t.Errorf("expected tool 'read', got %q", plan.Steps[0].Tool)
	}
}

func TestPlan_LegacyStepArray(t *testing.T) {
	provider := &mockProvider{
		content: `[{"description":"Read the file","tool":"read"},{"description":"Edit the code","tool":"edit"}]`,
	}
	p := New(provider, DefaultConfig())
	plan, err := p.Plan(context.Background(), "fix the bug", []string{"read", "edit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Legacy array defaults to workflow mode
	if plan.Mode != ModeWorkflow {
		t.Errorf("expected workflow mode for legacy array, got %q", plan.Mode)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.Steps))
	}
}

func TestPlan_LinesFallback(t *testing.T) {
	provider := &mockProvider{
		content: "Here is the plan:\n1. Read the file\n2. Fix the bug\n3. Run tests\n",
	}
	p := New(provider, DefaultConfig())
	plan, err := p.Plan(context.Background(), "fix the bug", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeWorkflow {
		t.Errorf("expected workflow mode for line fallback, got %q", plan.Mode)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
}

func TestPlan_EmptyGoal(t *testing.T) {
	p := New(&mockProvider{}, DefaultConfig())
	_, err := p.Plan(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty goal")
	}
}

func TestPlan_MaxSteps(t *testing.T) {
	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "do stuff", "steps": [
			{"description":"s1"},{"description":"s2"},{"description":"s3"},{"description":"s4"}
		]}`,
	}
	cfg := DefaultConfig()
	cfg.MaxSteps = 2
	p := New(provider, cfg)
	plan, err := p.Plan(context.Background(), "do stuff", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps (capped), got %d", len(plan.Steps))
	}
}

func TestValidateSteps_Empty(t *testing.T) {
	errs := ValidateSteps(nil, nil)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateSteps_UnknownTool(t *testing.T) {
	steps := []Step{{Description: "do it", Tool: "nope"}}
	errs := ValidateSteps(steps, []string{"read", "edit"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestParsePlan_JSON(t *testing.T) {
	input := `Some preamble {"mode":"agent","goal":"test"} more text`
	plan, err := parsePlan(input, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeAgent {
		t.Errorf("expected agent mode, got %q", plan.Mode)
	}
}

func TestParsePlan_LegacyArray(t *testing.T) {
	input := `[{"description":"step one","tool":"read"}]`
	plan, err := parsePlan(input, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeWorkflow {
		t.Errorf("expected workflow mode for legacy array, got %q", plan.Mode)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
}

func TestParsePlan_Lines(t *testing.T) {
	input := "1. First step\n2. Second step\n- Third step\n"
	plan, err := parsePlan(input, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeWorkflow {
		t.Errorf("expected workflow mode, got %q", plan.Mode)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
}

func TestPlan_EmptyWorkflowDemotesToAgent(t *testing.T) {
	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "do something"}`,
	}
	p := New(provider, DefaultConfig())
	plan, err := p.Plan(context.Background(), "do something", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Workflow with no steps → demoted to agent
	if plan.Mode != ModeAgent {
		t.Errorf("expected agent mode (demoted), got %q", plan.Mode)
	}
}

func TestPlan_NamedWorkflowNoStepsStaysWorkflow(t *testing.T) {
	provider := &mockProvider{
		content: `{"mode": "workflow", "goal": "train model", "workflow": "qwen_train_compare"}`,
	}
	p := New(provider, DefaultConfig())
	plan, err := p.Plan(context.Background(), "train model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeWorkflow {
		t.Errorf("expected workflow mode, got %q", plan.Mode)
	}
	if plan.Workflow != "qwen_train_compare" {
		t.Errorf("expected workflow 'qwen_train_compare', got %q", plan.Workflow)
	}
	if len(plan.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(plan.Steps))
	}
}

func TestNormalizeAndValidateWorkflowPlan_AgentNoOp(t *testing.T) {
	plan := Plan{Mode: ModeAgent, Goal: "explore"}
	if err := normalizeAndValidateWorkflowPlan(&plan, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeAgent {
		t.Errorf("expected agent mode unchanged, got %q", plan.Mode)
	}
}

func TestNormalizeAndValidateWorkflowPlan_DemoteToAgent(t *testing.T) {
	plan := Plan{Mode: ModeWorkflow, Goal: "do stuff", Workflow: "leftover"}
	// No workflow name AND no steps → but Workflow is set, so this should NOT demote.
	// Reset to truly empty to test demotion.
	plan.Workflow = ""
	if err := normalizeAndValidateWorkflowPlan(&plan, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeAgent {
		t.Errorf("expected demoted to agent, got %q", plan.Mode)
	}
	if plan.Workflow != "" {
		t.Errorf("expected workflow cleared, got %q", plan.Workflow)
	}
	if plan.Steps != nil {
		t.Errorf("expected steps cleared, got %v", plan.Steps)
	}
}

func TestNormalizeAndValidateWorkflowPlan_NamedWorkflowNoSteps(t *testing.T) {
	plan := Plan{Mode: ModeWorkflow, Goal: "train", Workflow: "qwen_train"}
	if err := normalizeAndValidateWorkflowPlan(&plan, []string{"shell"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != ModeWorkflow {
		t.Errorf("expected workflow mode preserved, got %q", plan.Mode)
	}
	if plan.Workflow != "qwen_train" {
		t.Errorf("expected workflow preserved, got %q", plan.Workflow)
	}
}

func TestNormalizeAndValidateWorkflowPlan_InvalidTool(t *testing.T) {
	plan := Plan{
		Mode: ModeWorkflow,
		Goal: "do stuff",
		Steps: []Step{
			{Description: "step one", Tool: "nonexistent"},
		},
	}
	err := normalizeAndValidateWorkflowPlan(&plan, []string{"read", "edit"})
	if err == nil {
		t.Fatal("expected validation error for unknown tool")
	}
}

func TestRefine(t *testing.T) {
	provider := &mockProvider{
		content: `{"mode":"workflow","goal":"improved","steps":[{"description":"Improved step","tool":"shell"}]}`,
	}
	p := New(provider, DefaultConfig())
	plan, err := p.Refine(context.Background(), "goal", []Step{{Description: "old step"}}, "make it better")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Description != "Improved step" {
		t.Errorf("expected 'Improved step', got %q", plan.Steps[0].Description)
	}
}
