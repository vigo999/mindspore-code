// Package planner owns task decomposition and planning strategy selection.
// It calls the LLM to decide execution mode (agent vs workflow) and
// produces a structured Plan for the orchestrator.
package planner

import (
	"context"
	"fmt"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// Config controls planner behavior.
type Config struct {
	MaxSteps    int
	Temperature float32
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxSteps:    10,
		Temperature: 0.3,
	}
}

// Planner calls the LLM to analyze a goal and produce an execution plan.
type Planner struct {
	provider llm.Provider
	config   Config
}

// New creates a Planner.
func New(provider llm.Provider, cfg Config) *Planner {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 10
	}
	return &Planner{
		provider: provider,
		config:   cfg,
	}
}

// Plan analyzes the goal and returns a structured Plan with execution mode,
// optional workflow selection, and optional inline steps.
func (p *Planner) Plan(ctx context.Context, goal string, tools []string) (Plan, error) {
	if goal == "" {
		return Plan{}, fmt.Errorf("goal cannot be empty")
	}

	prompt := buildPlanPrompt(goal, tools)

	resp, err := p.provider.Complete(ctx, &llm.CompletionRequest{
		Messages:    []llm.Message{llm.NewUserMessage(prompt)},
		Temperature: p.config.Temperature,
		MaxTokens:   2000,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("llm completion: %w", err)
	}

	plan, err := parsePlan(resp.Content, p.config.MaxSteps)
	if err != nil {
		return Plan{}, fmt.Errorf("parse plan: %w", err)
	}

	// Set goal from request if planner didn't refine it
	if plan.Goal == "" {
		plan.Goal = goal
	}

	if err := normalizeAndValidateWorkflowPlan(&plan, tools); err != nil {
		return Plan{}, fmt.Errorf("validate plan: %w", err)
	}

	return plan, nil
}

// Refine takes existing steps and feedback, returns an improved plan.
func (p *Planner) Refine(ctx context.Context, goal string, steps []Step, feedback string) (Plan, error) {
	prompt := buildRefinePrompt(goal, steps, feedback)

	resp, err := p.provider.Complete(ctx, &llm.CompletionRequest{
		Messages:    []llm.Message{llm.NewUserMessage(prompt)},
		Temperature: p.config.Temperature,
		MaxTokens:   2000,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("llm completion: %w", err)
	}

	plan, err := parsePlan(resp.Content, p.config.MaxSteps)
	if err != nil {
		return Plan{}, fmt.Errorf("parse refined plan: %w", err)
	}

	if plan.Goal == "" {
		plan.Goal = goal
	}

	// Refine currently validates workflow shape and step structure, but does not
	// validate tool names because Refine does not yet accept the available tool list.
	if err := normalizeAndValidateWorkflowPlan(&plan, nil); err != nil {
		return Plan{}, fmt.Errorf("validate refined plan: %w", err)
	}

	return plan, nil
}

func normalizeAndValidateWorkflowPlan(plan *Plan, tools []string) error {
	if plan.Mode != ModeWorkflow {
		return nil
	}

	if plan.Workflow == "" && len(plan.Steps) == 0 {
		plan.Mode = ModeAgent
		plan.Workflow = ""
		plan.Steps = nil
		return nil
	}

	// Named workflow plans may omit inline steps; the workflow executor resolves
	// the workflow definition later.
	if len(plan.Steps) == 0 {
		return nil
	}

	if errs := ValidateSteps(plan.Steps, tools); len(errs) > 0 {
		return errs[0]
	}
	return nil
}
