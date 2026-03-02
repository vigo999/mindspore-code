package loop

import "time"

type ModelSpec struct {
	Provider string
	Name     string
	Endpoint string
}

type Task struct {
	ID          string
	SessionID   string
	Mode        string
	Description string
	MaxSteps    int
	Model       ModelSpec
}

type EventType string

const (
	EventThinking   EventType = "thinking"
	EventReply      EventType = "reply"
	EventToolRead   EventType = "tool_read"
	EventToolGrep   EventType = "tool_grep"
	EventToolEdit   EventType = "tool_edit"
	EventToolWrite  EventType = "tool_write"
	EventCmdStarted EventType = "cmd_started"
	EventCmdOutput  EventType = "cmd_output"
	EventCmdFinish  EventType = "cmd_finished"
	EventToolError  EventType = "tool_error"
	EventTokenUsage EventType = "token_usage"
)

type Event struct {
	Type       EventType
	Message    string
	ToolName   string
	Summary    string
	CtxUsed    int
	TokensUsed int
	Time       time.Time
}
