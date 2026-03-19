package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ctxmanager "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/permission"
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/trace"
)

// EngineConfig holds engine configuration.
type EngineConfig struct {
	MaxIterations  int
	MaxTokens      int
	Temperature    float32
	TimeoutPerTurn time.Duration
	SystemPrompt   string
}

// Engine runs the ReAct loop: LLM → tool call → LLM → done.
type Engine struct {
	config     EngineConfig
	provider   llm.Provider
	tools      *tools.Registry
	ctxManager *ctxmanager.Manager
	permission permission.PermissionService
	trace      trace.Writer
}

// NewEngine creates a new engine.
func NewEngine(cfg EngineConfig, provider llm.Provider, tools *tools.Registry) *Engine {
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt()
	}

	engine := &Engine{
		config:   cfg,
		provider: provider,
		tools:    tools,
	}

	engine.ctxManager = ctxmanager.NewManager(ctxmanager.ManagerConfig{
		MaxTokens:     cfg.MaxTokens,
		ReserveTokens: 4000,
	})
	engine.ctxManager.SetSystemPrompt(cfg.SystemPrompt)
	engine.permission = permission.NewNoOpPermissionService()

	return engine
}

// SetContextManager sets the context manager.
func (e *Engine) SetContextManager(cm *ctxmanager.Manager) {
	if cm == nil {
		return
	}
	if cm.GetSystemPrompt() == nil {
		switch {
		case e.ctxManager != nil && e.ctxManager.GetSystemPrompt() != nil:
			cm.SetSystemPrompt(e.ctxManager.GetSystemPrompt().Content)
		case e.config.SystemPrompt != "":
			cm.SetSystemPrompt(e.config.SystemPrompt)
		}
	}
	e.ctxManager = cm
}

// SetPermissionService sets the permission service.
func (e *Engine) SetPermissionService(ps permission.PermissionService) {
	e.permission = ps
}

// SetTraceWriter sets the trace writer.
func (e *Engine) SetTraceWriter(w trace.Writer) {
	e.trace = w
}

// ToolNames returns the names of registered tools.
func (e *Engine) ToolNames() []string {
	toolList := e.tools.List()
	names := make([]string, len(toolList))
	for i, t := range toolList {
		names[i] = t.Name()
	}
	return names
}

// Run executes a task and returns events.
func (e *Engine) Run(task Task) ([]Event, error) {
	return e.RunWithContext(context.Background(), task)
}

// RunWithContext executes the ReAct loop for a task.
func (e *Engine) RunWithContext(ctx context.Context, task Task) ([]Event, error) {
	startedAt := time.Now()
	e.writeTrace("run_started", map[string]any{
		"task_id":     task.ID,
		"description": task.Description,
		"started_at":  startedAt,
	})

	exec := &executor{
		engine:    e,
		task:      task,
		events:    make([]Event, 0),
		startTime: startedAt,
	}
	events, err := exec.run(ctx)

	e.writeTrace("run_finished", map[string]any{
		"task_id":     task.ID,
		"duration_ms": time.Since(startedAt).Milliseconds(),
		"event_count": len(events),
	})
	return events, err
}

func (e *Engine) writeTrace(eventType string, payload any) {
	if e.trace == nil {
		return
	}
	_ = e.trace.Write(eventType, payload)
}

// executor manages a single ReAct loop run.
type executor struct {
	engine     *Engine
	task       Task
	events     []Event
	iterCount  int
	startTime  time.Time
	totalUsage llm.Usage
}

func (ex *executor) run(ctx context.Context) ([]Event, error) {
	ex.engine.ctxManager.AddMessage(llm.NewUserMessage(ex.task.Description))
	ex.addEvent(NewEvent(EventTaskStarted, fmt.Sprintf("Task: %s", ex.task.Description)))
	ex.addEvent(NewEvent(EventAgentThinking, ""))

	for ex.engine.config.MaxIterations == 0 || ex.iterCount < ex.engine.config.MaxIterations {
		ex.iterCount++

		if err := ctx.Err(); err != nil {
			ex.addEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Context cancelled: %v", err)))
			return ex.events, err
		}

		resp, err := ex.callLLM(ctx)
		if err != nil {
			return ex.events, err
		}

		ex.trackUsage(resp.Usage)

		continueLoop, err := ex.handleResponse(ctx, resp)
		if err != nil {
			ex.addEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Handle response error: %v", err)))
			return ex.events, err
		}
		if !continueLoop {
			break
		}
	}

	if ex.engine.config.MaxIterations > 0 && ex.iterCount >= ex.engine.config.MaxIterations {
		ex.addEvent(NewEvent(EventTaskFailed, "Task exceeded maximum iterations."))
	} else {
		ex.addEvent(NewEvent(EventTaskCompleted, "Task completed successfully"))
	}

	return ex.events, nil
}

func (ex *executor) callLLM(ctx context.Context) (*llm.CompletionResponse, error) {
	timeout := ex.engine.config.TimeoutPerTurn
	if timeout == 0 {
		timeout = 180 * time.Second
	}

	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := &llm.CompletionRequest{
		Messages:    ex.engine.ctxManager.GetMessages(),
		Tools:       ex.engine.tools.ToLLMTools(),
		Temperature: ex.engine.config.Temperature,
	}
	ex.engine.writeTrace("llm_request", map[string]any{
		"iteration": ex.iterCount,
	})

	resp, err := ex.engine.provider.Complete(llmCtx, req)
	if err != nil {
		errMsg := fmt.Sprintf("LLM error: %v", err)
		if ctx.Err() == context.DeadlineExceeded || llmCtx.Err() == context.DeadlineExceeded {
			errMsg = fmt.Sprintf("Request timeout (ctx: %d tokens). Try /compact.",
				ex.engine.ctxManager.TokenUsage().Current)
		}
		ex.addEvent(NewEvent(EventTaskFailed, errMsg))
		return nil, fmt.Errorf("LLM completion: %w", err)
	}

	ex.engine.writeTrace("llm_response", map[string]any{
		"iteration": ex.iterCount,
	})
	return resp, nil
}

func (ex *executor) handleResponse(ctx context.Context, resp *llm.CompletionResponse) (bool, error) {
	ex.engine.ctxManager.AddMessage(llm.Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	})

	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			if err := ex.executeToolCall(ctx, tc); err != nil {
				return false, err
			}
		}
		return true, nil
	}

	if resp.Content != "" {
		ex.addEvent(NewEvent(EventAgentReply, resp.Content))
	}
	return false, nil
}

func (ex *executor) executeToolCall(ctx context.Context, tc llm.ToolCall) error {
	toolName := tc.Function.Name

	tool, ok := ex.engine.tools.Get(toolName)
	if !ok {
		errMsg := fmt.Sprintf("Tool not found: %s", toolName)
		ex.addEvent(NewEvent(EventToolError, errMsg))
		ex.engine.ctxManager.AddToolResult(tc.ID, errMsg)
		return nil
	}

	// Check permission
	action := extractAction(toolName, tc.Function.Arguments)
	path := extractPathArg(tc.Function.Arguments)
	granted, err := ex.engine.permission.Request(ctx, toolName, action, path)
	if err != nil {
		return err
	}
	if !granted {
		errMsg := fmt.Sprintf("Permission denied for tool: %s", toolName)
		ex.addEvent(NewEvent(EventToolError, errMsg))
		ex.engine.ctxManager.AddToolResult(tc.ID, errMsg)
		return nil
	}

	// Execute
	result, err := tool.Execute(ctx, tc.Function.Arguments)
	if err != nil {
		errMsg := fmt.Sprintf("Tool execution error: %v", err)
		ex.addEvent(NewEvent(EventToolError, errMsg))
		ex.engine.ctxManager.AddToolResult(tc.ID, errMsg)
		return nil
	}

	if result.Error != nil {
		errMsg := result.Error.Error()
		ex.addEvent(NewEvent(EventToolError, fmt.Sprintf("Tool %s failed: %s", toolName, errMsg)))
		ex.engine.ctxManager.AddToolResult(tc.ID, errMsg)
		return nil
	}

	ex.addToolEvent(toolName, result)
	ex.engine.ctxManager.AddToolResult(tc.ID, result.Content)
	return nil
}

var toolEventMap = map[string]string{
	"read":  EventToolRead,
	"grep":  EventToolGrep,
	"glob":  EventToolGlob,
	"edit":  EventToolEdit,
	"write": EventToolWrite,
	"shell": EventCmdStarted,
}

func (ex *executor) addToolEvent(toolName string, result *tools.Result) {
	eventType := EventToolStarted
	if t, ok := toolEventMap[toolName]; ok {
		eventType = t
	}
	ev := NewEvent(eventType, result.Content)
	ev.ToolName = toolName
	ev.Summary = result.Summary
	ex.addEvent(ev)
}

func (ex *executor) addEvent(ev Event) {
	usage := ex.engine.ctxManager.TokenUsage()
	ev.CtxUsed = usage.Current
	ev.CtxMax = usage.Max
	ev.TokensUsed = ex.totalUsage.TotalTokens
	ex.events = append(ex.events, ev)
	ex.engine.writeTrace("event", ev)
}

func (ex *executor) trackUsage(u llm.Usage) {
	ex.totalUsage.PromptTokens += u.PromptTokens
	ex.totalUsage.CompletionTokens += u.CompletionTokens
	ex.totalUsage.TotalTokens += u.TotalTokens
}

func extractAction(toolName string, raw json.RawMessage) string {
	if toolName == "shell" {
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(raw, &args); err == nil {
			if cmd := strings.TrimSpace(args.Command); cmd != "" {
				return cmd
			}
		}
	}
	return string(raw)
}

func extractPathArg(raw json.RawMessage) string {
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return ""
	}
	for _, key := range []string{"path", "file_path"} {
		if v, ok := params[key].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func DefaultSystemPrompt() string {
	return `You are an AI assistant that helps users with software development tasks.

You have access to the following tools:
- read: Read file contents
- write: Create or overwrite files
- edit: Edit files by replacing text
- grep: Search for patterns in files
- glob: Find files matching patterns
- shell: Execute shell commands
- load_skill: Load a skill's detailed instructions. Call this when the user's task matches an available skill listed in the system prompt.

Guidelines:
1. Use tools to gather information before making changes
2. Always read files before editing them
3. Make minimal, focused changes
4. Use grep and glob to explore the codebase
5. Run tests with shell to verify changes

IMPORTANT: When you have gathered enough information to answer the user's question, you MUST provide your final answer directly WITHOUT using any more tools. Do not keep calling tools indefinitely - provide a clear, concise response once you have the information needed.

When making edits, ensure the old_string matches exactly (including whitespace and newlines).`
}
