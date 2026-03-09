package orchestrator

import "github.com/vigo999/ms-cli/agent/planner"

// PlanCallback receives plan-level lifecycle events.
// Implemented by the UI or app layer.
//
// Step-level callbacks (OnStepStarted, OnStepCompleted) are intentionally
// excluded — they belong to the workflow executor, which will define its
// own StepCallback interface when the real implementation is built.
type PlanCallback interface {
	// OnPlanCreated is called when a plan is generated.
	// Return error to abort.
	OnPlanCreated(plan planner.Plan) error

	// OnPlanApproved is called when execution begins.
	OnPlanApproved(plan planner.Plan) error
}

// NoOpCallback is a default callback that does nothing.
type NoOpCallback struct{}

func (NoOpCallback) OnPlanCreated(planner.Plan) error  { return nil }
func (NoOpCallback) OnPlanApproved(planner.Plan) error { return nil }
