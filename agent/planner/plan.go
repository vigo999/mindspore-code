package planner

// ExecutionMode determines how the plan should be executed.
type ExecutionMode string

const (
	// ModeAgent uses the ReAct loop for exploratory, open-ended tasks.
	ModeAgent ExecutionMode = "agent"
	// ModeWorkflow uses structured step-by-step execution.
	ModeWorkflow ExecutionMode = "workflow"
)

// Plan is the structured output of the planner.
// The orchestrator dispatches based on Plan.Mode.
type Plan struct {
	Mode     ExecutionMode  `json:"mode"`
	Goal     string         `json:"goal"`
	Workflow string         `json:"workflow,omitempty"` // named workflow ID, e.g. "qwen_train_compare"
	Steps    []Step         `json:"steps,omitempty"`    // inline step definitions
	Params   map[string]any `json:"params,omitempty"`   // input params for named workflow or steps
}
