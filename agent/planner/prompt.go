package planner

import (
	"fmt"
	"strings"
)

func buildPlanPrompt(goal string, tools []string) string {
	var toolDesc string
	if len(tools) > 0 {
		toolDesc = strings.Join(tools, ", ")
	} else {
		toolDesc = "read, write, edit, grep, glob, shell"
	}

	return fmt.Sprintf(`You are a planning assistant. Analyze the following task and decide how to execute it.

Task: %s

Available tools: %s

First, decide the execution mode:
- "agent": for exploratory, open-ended, or coding-assistant-style tasks
  (e.g. analyze code, investigate a bug, explain architecture, propose changes,
  fix a bug, add a feature, refactor code)
- "workflow": for structured, reproducible tasks with stable execution stages
  (e.g. run training, benchmark comparison, build/test/verify pipeline,
  migration pipeline, batch data processing)

Most coding tasks are agent mode. Use workflow only when the task is a
repeatable pipeline with well-defined stages.

Then respond with JSON:

For agent mode:
{"mode": "agent", "goal": "refined goal description"}

For workflow mode with inline steps:
{"mode": "workflow", "goal": "refined goal description", "steps": [
  {"description": "Step 1 description", "tool": "tool_name"},
  {"description": "Step 2 description", "tool": "tool_name"}
]}

For workflow mode with a named workflow:
{"mode": "workflow", "goal": "refined goal description", "workflow": "workflow_id"}

Rules:
- Default to agent mode for coding, debugging, and exploratory tasks
- Choose workflow mode only for pipeline-style, reproducible operations
- Keep steps concise (3-10 steps max)
- Only use tools from the available list`, goal, toolDesc)
}

func buildRefinePrompt(goal string, steps []Step, feedback string) string {
	var sb strings.Builder
	for i, s := range steps {
		fmt.Fprintf(&sb, "%d. %s", i+1, s.Description)
		if s.Tool != "" {
			fmt.Fprintf(&sb, " (tool: %s)", s.Tool)
		}
		sb.WriteByte('\n')
	}

	return fmt.Sprintf(`Given the following plan and feedback, refine the plan.

Original Goal: %s

Current Plan:
%s
Feedback: %s

Please provide an improved plan in the same JSON format (with mode, goal, and steps).`, goal, sb.String(), feedback)
}
