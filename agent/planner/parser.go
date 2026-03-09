package planner

import (
	"encoding/json"
	"regexp"
	"strings"
)

// parsePlan extracts a Plan from LLM output.
//
// Confidence levels:
// 1. JSON object Plan: high confidence. The LLM explicitly chose a mode.
// 2. Legacy []Step array: medium confidence. The output is structured, but
//    older and less explicit than the current JSON object format.
// 3. Line-based parsing: low confidence heuristic fallback. This is only a
//    compatibility path when the model does not return structured JSON.
//
// Note: line-based parsing is not a strong semantic signal that the request
// truly belongs to workflow mode. Agent-style reasoning can also appear as a
// numbered list. We currently map parsed lines into a workflow-shaped Plan to
// preserve backward compatibility, but future planner/executor logic may treat
// this source with lower trust than JSON-based plans.
func parsePlan(content string, maxSteps int) (Plan, error) {
	// Try new format: {"mode": "...", "goal": "...", "steps": [...]}
	if plan, err := parsePlanJSON(content); err == nil {
		if len(plan.Steps) > maxSteps {
			plan.Steps = plan.Steps[:maxSteps]
		}
		return plan, nil
	}

	// Fallback: try legacy []Step array (medium-confidence compatibility path)
	if steps, err := parseStepArray(content, maxSteps); err == nil && len(steps) > 0 {
		return Plan{
			Mode:  ModeWorkflow,
			Goal:  "",
			Steps: steps,
		}, nil
	}

	// Fallback: line-based parsing (low-confidence heuristic compatibility path).
	// We still shape this as workflow for now, but this should not be treated
	// as equivalent in confidence to an explicit JSON plan.
	if steps := parseLines(content, maxSteps); len(steps) > 0 {
		return Plan{
			Mode:  ModeWorkflow,
			Goal:  "",
			Steps: steps,
		}, nil
	}

	// Nothing parseable → agent mode with original content as goal
	return Plan{Mode: ModeAgent}, nil
}

func parsePlanJSON(content string) (Plan, error) {
	raw := extractJSONObject(content)
	if raw == "" {
		return Plan{}, errNoJSON
	}

	var plan Plan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return Plan{}, err
	}

	// Must have an explicit mode field to be a valid Plan JSON
	if plan.Mode != ModeAgent && plan.Mode != ModeWorkflow {
		return Plan{}, errNoJSON
	}

	return plan, nil
}

func parseStepArray(content string, maxSteps int) ([]Step, error) {
	raw := extractJSONArray(content)
	if raw == "" {
		return nil, errNoJSON
	}

	var parsed []Step
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}

	if len(parsed) > maxSteps {
		parsed = parsed[:maxSteps]
	}
	return parsed, nil
}

func parseLines(content string, maxSteps int) []Step {
	var steps []Step
	for _, line := range strings.Split(content, "\n") {
		if len(steps) >= maxSteps {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if desc := matchStepLine(line); desc != "" {
			steps = append(steps, Step{Description: desc})
		}
	}
	return steps
}

var stepPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\d+[.)]\s*(.+)$`),
	regexp.MustCompile(`^-\s*(.+)$`),
	regexp.MustCompile(`^\*\s*(.+)$`),
	regexp.MustCompile(`^Step\s+\d+[:.]\s*(.+)$`),
}

func matchStepLine(line string) string {
	for _, re := range stepPatterns {
		if m := re.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

func extractJSONArray(text string) string {
	start := strings.Index(text, "[")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

type parseError string

func (e parseError) Error() string { return string(e) }

const errNoJSON = parseError("no JSON found in content")
