// Package executor will provide the real workflow engine that executes
// structured plans. For now it is a stub that signals "not implemented"
// so the orchestrator can fall back to agent mode.
package executor

import (
	"context"

	"github.com/vigo999/ms-cli/agent/orchestrator"
	"github.com/vigo999/ms-cli/agent/planner"
)

// Stub satisfies orchestrator.WorkflowExecutor but always returns
// orchestrator.ErrWorkflowNotImplemented. Replace with a real
// implementation when the workflow engine is built.
type Stub struct{}

// New creates a workflow executor stub.
func New() *Stub {
	return &Stub{}
}

// Execute always returns ErrWorkflowNotImplemented.
func (s *Stub) Execute(_ context.Context, _ orchestrator.RunRequest, _ planner.Plan) ([]orchestrator.RunEvent, error) {
	return nil, orchestrator.ErrWorkflowNotImplemented
}
