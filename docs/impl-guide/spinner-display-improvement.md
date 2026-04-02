# Spinner Display Improvement

## Context

The current live area spinner is limited — it shows "Thinking..." for all model wait states and plain text "waiting for tool result..." during tool execution. Claude Code (crush) uses 29+ distinct spinner states with tool-specific labels. This plan improves the spinner to show contextual information.

## Current State

| State | Current display |
|-------|----------------|
| WaitModel (thinking) | `⣻ Thinking... 3s` |
| WaitModel (summarizing) | `⣻ Thinking... 3s` (same, no distinction) |
| WaitTool | `waiting for tool result...` (no spinner) |
| Tool streaming | Tool header only |
| Delta buffering | `⣻ Thinking...` (same as thinking) |

## Target State

| State | Target display |
|-------|---------------|
| WaitModel (thinking) | `⣻ Thinking... 3s` |
| WaitModel (summarizing) | `⣻ Summarizing... 2s` |
| WaitTool | `⣻ Running Read... 2s` |
| Delta buffering | `⣻ Responding...` |

## Crush Reference

Crush (`internal/ui/chat/assistant.go:213-219`) differentiates thinking vs summarizing via `message.IsSummaryMessage` flag. Each tool has a display name (Bash, View, Edit, Grep, Glob, Write, Fetch, Search, Agent, etc.).

## Implementation

### Step 1: Add summarizing detection

In `agent/loop/engine.go` or the event system, add a way to distinguish context compaction from normal thinking. Options:
- Add `EventAgentSummarizing` event type
- Or set a flag on `AgentThinking` event (e.g., `ev.Summary = "summarizing"`)

In `ui/app.go` `handleEvent`, when compaction is detected, call `a.thinking.SetText("Summarizing...")`.

### Step 2: WaitTool spinner

In `ui/inline.go` `activePreview()`, replace the plain text:

```go
// Before:
if a.state.WaitKind == model.WaitTool {
    return metaStyle.Render("waiting for tool result...")
}

// After:
if a.state.WaitKind == model.WaitTool {
    label := "Running tool..."
    if msg, ok := a.lastActiveTool(); ok {
        label = "Running " + msg.ToolName + "..."
    }
    a.thinking.SetText(label)
    return a.thinking.View()
}
```

### Step 3: Delta buffering label

Change the delta buffering case to show "Responding..." instead of "Thinking...":

```go
if hasDelta {
    a.thinking.SetText("Responding...")
    return a.thinking.View()
}
```

### Step 4: Short tool display names

Add a `shortToolName()` function mapping verbose tool calls to short names:

| Engine name | Display name |
|-------------|-------------|
| Shell | Bash |
| Read | Read |
| Edit | Edit |
| Write | Write |
| Grep | Grep |
| Glob | Glob |

Use in both the spinner label and the scrollback tool call line.

### Step 5: Tool call line format (optional, bigger scope)

Change scrollback format from `⏺ Shell($ ls -la)` to `⏺ Bash $ ls -la` — matching crush's cleaner style. This touches `renderToolCallLine`, `RenderToolCallHeader`, and `pendingToolMessage`.

## Files to Modify

- `ui/inline.go` — `activePreview()` for WaitTool spinner and delta label
- `ui/components/thinking.go` — `SetText()` already exists, may need `Reset()` to restore default
- `ui/app.go` — detect summarizing state, set spinner text
- `agent/loop/engine.go` — (optional) add summarizing event/flag

## Depends On

- Tip-line feature (should be implemented first as it's smaller)
- Does NOT depend on full-screen chat migration

## Verification

1. `go build ./... && go test ./ui/...`
2. Manual: send a prompt → verify "Thinking..." during model wait
3. Manual: trigger a tool call → verify "Running Read..." during tool wait
4. Manual: observe "Responding..." during delta buffering
