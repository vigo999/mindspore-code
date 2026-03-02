package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/integrations/domain"
)

const plannerSystemPrompt = `You are ms-cli planner.
Return ONLY JSON, no markdown.
Schema:
{
  "action":"read|grep|edit|write|shell|final",
  "path":"optional",
  "pattern":"optional regex for grep",
  "old_text":"optional for edit",
  "new_text":"optional for edit",
  "content":"optional for write",
  "command":"optional for shell",
  "final":"required when action=final"
}
Rules:
- Prefer small, safe steps.
- Use relative paths.
- Use shell only when needed.
- If enough evidence exists, return action=final with concise conclusion.`

type Config struct {
	FS             FSTool
	Shell          ShellTool
	ModelFactory   domain.Factory
	Permission     PermissionService
	Trace          TraceWriter
	DefaultMaxStep int
	MaxOutputLines int
}

// Engine drives task execution and emits events.
type Engine struct {
	fs             FSTool
	shell          ShellTool
	modelFactory   domain.Factory
	permission     PermissionService
	trace          TraceWriter
	defaultMaxStep int
	maxOutputLines int
}

const assistantSystemPrompt = `You are ms-cli assistant in a terminal UI.
Be concise and practical.`

type plannerAction struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
	Content string `json:"content"`
	Command string `json:"command"`
	Final   string `json:"final"`
}

func NewEngine(cfg Config) *Engine {
	maxSteps := cfg.DefaultMaxStep
	if maxSteps < 0 {
		maxSteps = 0
	}
	maxOutputLines := cfg.MaxOutputLines
	if maxOutputLines <= 0 {
		maxOutputLines = 200
	}

	return &Engine{
		fs:             cfg.FS,
		shell:          cfg.Shell,
		modelFactory:   cfg.ModelFactory,
		permission:     cfg.Permission,
		trace:          cfg.Trace,
		defaultMaxStep: maxSteps,
		maxOutputLines: maxOutputLines,
	}
}

func (e *Engine) Run(task Task) ([]Event, error) {
	return e.runWithContext(context.Background(), task, nil)
}

func (e *Engine) RunWithContext(ctx context.Context, task Task) ([]Event, error) {
	return e.runWithContext(ctx, task, nil)
}

func (e *Engine) RunWithContextStream(ctx context.Context, task Task, emit func(Event)) error {
	_, err := e.runWithContext(ctx, task, emit)
	return err
}

func (e *Engine) runWithContext(ctx context.Context, task Task, emit func(Event)) ([]Event, error) {
	events := make([]Event, 0, 32)
	push := func(ev Event) {
		events = append(events, ev)
		if emit != nil {
			emit(ev)
		}
	}

	if strings.TrimSpace(task.Description) == "" {
		return nil, fmt.Errorf("task description is required")
	}
	if e.modelFactory == nil {
		return nil, fmt.Errorf("model factory is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		push(e.newEvent(EventReply, "任务已暂停。", "", ""))
		return events, err
	}

	client, err := e.modelFactory.ClientFor(domain.ModelSpec{
		Provider: task.Model.Provider,
		Model:    task.Model.Name,
		Endpoint: task.Model.Endpoint,
	})
	if err != nil {
		ev := e.newEvent(EventToolError, err.Error(), "Model", "")
		push(ev)
		return events, err
	}

	if shouldDirectReply(task.Description) {
		directEvents, directErr := e.runDirectReply(ctx, client, task)
		if errors.Is(directErr, context.Canceled) {
			push(e.newEvent(EventReply, "任务已暂停。", "", ""))
			return events, directErr
		}
		for _, ev := range directEvents {
			push(ev)
		}
		return events, directErr
	}

	maxSteps := task.MaxSteps
	if maxSteps < 0 {
		maxSteps = 0
	}
	if maxSteps == 0 {
		maxSteps = e.defaultMaxStep
	}
	if maxSteps < 0 {
		maxSteps = 0
	}

	historyCap := 16
	if maxSteps > 0 {
		historyCap = maxSteps * 2
	}
	history := make([]string, 0, historyCap)
	totalCtx := 0
	totalTokens := 0

	for step := 1; ; step++ {
		if maxSteps > 0 && step > maxSteps {
			break
		}
		if err := ctx.Err(); err != nil {
			push(e.newEvent(EventReply, "任务已暂停。", "", ""))
			return events, err
		}
		thinking := fmt.Sprintf("Planning step %d...", step)
		if maxSteps > 0 {
			thinking = fmt.Sprintf("Planning step %d/%d...", step, maxSteps)
		}
		push(e.newEvent(EventThinking, thinking, "", ""))

		action, usage, raw, planErr := e.planNext(ctx, client, task, history, step, maxSteps)
		if usage != nil {
			totalCtx += usage.PromptTokens
			totalTokens += usage.TotalTokens
			push(Event{
				Type:       EventTokenUsage,
				CtxUsed:    totalCtx,
				TokensUsed: totalTokens,
				Time:       time.Now().UTC(),
			})
		}
		if planErr != nil {
			if errors.Is(planErr, context.Canceled) {
				push(e.newEvent(EventReply, "任务已暂停。", "", ""))
				return events, planErr
			}
			ev := e.newEvent(EventToolError, planErr.Error(), "Planner", "")
			push(ev)
			_ = e.writeTrace("planner_error", map[string]any{"error": planErr.Error()})
			return events, planErr
		}

		if strings.TrimSpace(action.Action) == "" {
			action.Action = "final"
			action.Final = strings.TrimSpace(raw)
		}

		switch strings.ToLower(action.Action) {
		case "final":
			finalMsg := strings.TrimSpace(action.Final)
			if finalMsg == "" {
				finalMsg = strings.TrimSpace(raw)
			}
			if finalMsg == "" {
				finalMsg = "Done."
			}
			push(e.newEvent(EventReply, finalMsg, "", ""))
			_ = e.writeTrace("task_complete", map[string]any{"task": task.Description, "steps": step})
			return events, nil

		case "read":
			if !e.checkPermission("read", action.Path, action.Path, &events, emit) {
				history = append(history, "READ denied by permissions")
				continue
			}
			content, readErr := e.fs.Read(action.Path)
			if readErr != nil {
				// Model may occasionally choose reading "." or a directory.
				// Keep this internal and let planner continue instead of surfacing noisy tool errors.
				if isDirectoryErr(readErr) {
					history = append(history, "READ failed: target is a directory")
					continue
				}
				push(e.newEvent(EventToolError, readErr.Error(), "Read", ""))
				history = append(history, "READ failed: "+readErr.Error())
				continue
			}
			summary := fmt.Sprintf("%d lines", countLines(content))
			push(e.newEvent(EventToolRead, action.Path, "Read", summary))
			history = append(history, "READ "+action.Path+":\n"+truncate(content, 4000))
			_ = e.writeTrace("tool_read", map[string]any{"path": action.Path})

		case "grep":
			target := strings.TrimSpace(action.Path)
			if target == "" {
				target = "."
			}
			if !e.checkPermission("grep", action.Pattern, target, &events, emit) {
				history = append(history, "GREP denied by permissions")
				continue
			}
			matches, grepErr := e.fs.Grep(target, action.Pattern, 50)
			if grepErr != nil {
				push(e.newEvent(EventToolError, grepErr.Error(), "Grep", ""))
				history = append(history, "GREP failed: "+grepErr.Error())
				continue
			}
			msg := fmt.Sprintf("%q %s", action.Pattern, target)
			push(e.newEvent(EventToolGrep, msg, "Grep", fmt.Sprintf("%d matches", len(matches))))
			history = append(history, "GREP "+msg+"\n"+truncate(strings.Join(matches, "\n"), 4000))
			_ = e.writeTrace("tool_grep", map[string]any{"path": target, "pattern": action.Pattern, "matches": len(matches)})

		case "edit":
			if !e.checkPermission("edit", action.Path, action.Path, &events, emit) {
				history = append(history, "EDIT denied by permissions")
				continue
			}
			diff, editErr := e.fs.Edit(action.Path, action.OldText, action.NewText)
			if editErr != nil {
				push(e.newEvent(EventToolError, editErr.Error(), "Edit", ""))
				history = append(history, "EDIT failed: "+editErr.Error())
				continue
			}
			push(e.newEvent(EventToolEdit, action.Path+"\n\n"+diff, "Edit", ""))
			history = append(history, "EDIT "+action.Path+" success")
			_ = e.writeTrace("tool_edit", map[string]any{"path": action.Path})

		case "write":
			if !e.checkPermission("write", action.Path, action.Path, &events, emit) {
				history = append(history, "WRITE denied by permissions")
				continue
			}
			written, writeErr := e.fs.Write(action.Path, action.Content)
			if writeErr != nil {
				push(e.newEvent(EventToolError, writeErr.Error(), "Write", ""))
				history = append(history, "WRITE failed: "+writeErr.Error())
				continue
			}
			msg := fmt.Sprintf("%s\n\n+ wrote %d bytes", action.Path, written)
			push(e.newEvent(EventToolWrite, msg, "Write", ""))
			history = append(history, "WRITE "+action.Path+" success")
			_ = e.writeTrace("tool_write", map[string]any{"path": action.Path, "bytes": written})

		case "shell":
			if !e.checkPermission("shell", action.Command, "", &events, emit) {
				history = append(history, "SHELL denied by permissions")
				continue
			}
			push(e.newEvent(EventCmdStarted, action.Command, "Shell", ""))

			output, exitCode, runErr := e.shell.Run(ctx, action.Command)
			for _, line := range splitLines(output, e.maxOutputLines) {
				push(e.newEvent(EventCmdOutput, line, "Shell", ""))
			}
			if runErr != nil {
				if errors.Is(runErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
					push(e.newEvent(EventReply, "任务已暂停。", "", ""))
					push(e.newEvent(EventCmdFinish, "", "Shell", ""))
					return events, context.Canceled
				}
				errMsg := fmt.Sprintf("command failed (exit=%d): %v", exitCode, runErr)
				push(e.newEvent(EventToolError, errMsg, "Shell", ""))
				history = append(history, "SHELL failed: "+errMsg)
			} else {
				push(e.newEvent(EventCmdOutput, fmt.Sprintf("exit status %d", exitCode), "Shell", ""))
				history = append(history, "SHELL success")
			}
			push(e.newEvent(EventCmdFinish, "", "Shell", ""))
			_ = e.writeTrace("tool_shell", map[string]any{"command": action.Command, "exit_code": exitCode})

		default:
			finalMsg := strings.TrimSpace(raw)
			if finalMsg == "" {
				finalMsg = "Planner returned unsupported action."
			}
			push(e.newEvent(EventReply, finalMsg, "", ""))
			return events, nil
		}
	}

	msg := "Stopped after max steps without a final answer."
	push(e.newEvent(EventReply, msg, "", ""))
	return events, nil
}

func (e *Engine) planNext(
	ctx context.Context,
	client domain.ModelClient,
	task Task,
	history []string,
	step int,
	maxSteps int,
) (plannerAction, *domain.Usage, string, error) {
	stepInfo := fmt.Sprintf("%d", step)
	if maxSteps > 0 {
		stepInfo = fmt.Sprintf("%d of %d", step, maxSteps)
	}

	userPrompt := fmt.Sprintf(
		"Task:\n%s\n\nModel:\nprovider=%s\nname=%s\n\nStep:\n%s\n\nHistory:\n%s",
		task.Description,
		task.Model.Provider,
		task.Model.Name,
		stepInfo,
		joinHistory(history),
	)

	resp, err := client.Generate(ctx, domain.GenerateRequest{
		Model:        task.Model.Name,
		SystemPrompt: plannerSystemPrompt,
		Input:        userPrompt,
		Temperature:  0.1,
		MaxTokens:    800,
	})
	if err != nil {
		return plannerAction{}, nil, "", err
	}

	raw := strings.TrimSpace(resp.Text)
	action, parseErr := parsePlannerAction(raw)
	if parseErr != nil {
		return plannerAction{
			Action: "final",
			Final:  raw,
		}, &resp.Usage, raw, nil
	}
	return action, &resp.Usage, raw, nil
}

func (e *Engine) checkPermission(tool, action, path string, events *[]Event, emit func(Event)) bool {
	if e.permission == nil {
		return true
	}
	allowed, err := e.permission.Request(tool, action, path)
	if err != nil {
		ev := e.newEvent(EventToolError, err.Error(), title(tool), "")
		*events = append(*events, ev)
		if emit != nil {
			emit(ev)
		}
		return false
	}
	if !allowed {
		msg := fmt.Sprintf("permission denied: %s %s", tool, strings.TrimSpace(action))
		ev := e.newEvent(EventToolError, msg, title(tool), "")
		*events = append(*events, ev)
		if emit != nil {
			emit(ev)
		}
		return false
	}
	return true
}

func (e *Engine) newEvent(t EventType, msg, toolName, summary string) Event {
	return Event{
		Type:     t,
		Message:  msg,
		ToolName: toolName,
		Summary:  summary,
		Time:     time.Now().UTC(),
	}
}

func parsePlannerAction(raw string) (plannerAction, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return plannerAction{}, fmt.Errorf("empty planner response")
	}

	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var action plannerAction
	if err := json.Unmarshal([]byte(trimmed), &action); err != nil {
		return plannerAction{}, err
	}
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	return action, nil
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}

func splitLines(s string, max int) []string {
	if max <= 0 {
		max = 200
	}
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, min(len(raw), max))
	for _, line := range raw {
		if len(out) >= max {
			break
		}
		out = append(out, line)
	}
	if len(raw) > max {
		out = append(out, "... output truncated ...")
	}
	return out
}

func joinHistory(history []string) string {
	if len(history) == 0 {
		return "(none)"
	}
	return strings.Join(history, "\n\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (e *Engine) writeTrace(eventType string, payload any) error {
	if e.trace == nil {
		return nil
	}
	return e.trace.Write(eventType, payload)
}

func (e *Engine) runDirectReply(ctx context.Context, client domain.ModelClient, task Task) ([]Event, error) {
	resp, err := client.Generate(ctx, domain.GenerateRequest{
		Model:        task.Model.Name,
		SystemPrompt: assistantSystemPrompt,
		Input:        task.Description,
		Temperature:  0.2,
		MaxTokens:    200,
	})
	if err != nil {
		ev := e.newEvent(EventToolError, err.Error(), "Model", "")
		return []Event{ev}, err
	}

	events := []Event{
		{
			Type:       EventTokenUsage,
			CtxUsed:    resp.Usage.PromptTokens,
			TokensUsed: resp.Usage.TotalTokens,
			Time:       time.Now().UTC(),
		},
		e.newEvent(EventReply, strings.TrimSpace(resp.Text), "", ""),
	}
	_ = e.writeTrace("direct_reply", map[string]any{
		"input": task.Description,
	})
	return events, nil
}

func shouldDirectReply(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.Trim(s, "!?.,;:()[]{}\"'")
	if s == "" {
		return false
	}
	greetings := map[string]struct{}{
		"hi": {}, "hello": {}, "hey": {}, "yo": {}, "hola": {},
		"你好": {}, "您好": {}, "嗨": {},
	}
	_, ok := greetings[s]
	return ok
}

func isDirectoryErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "is a directory")
}
