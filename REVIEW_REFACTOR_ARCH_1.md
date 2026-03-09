# Review: `refactor-arch-1` — Orchestration & Agent/Loop (Update 2)

## What Changed Since Last Review

You addressed item #4 from the previous review — **decoupling orchestrator from loop types**:

1. **New `orchestrator.RunRequest`** replaces `loop.Task` as the orchestrator's input type
2. **New `orchestrator.RunEvent`** replaces `loop.Event` as the orchestrator's output type
3. **New `internal/app/adapter.go`** bridges `loop.Engine` → `orchestrator.Engine` interface
4. **`convertEvent` simplified** — the 70-line switch is now a map lookup + generic fallback
5. **Orchestrator owns its own event constants** — no longer imports `agent/loop`

This is a solid step. The dependency arrow is now correct:

```
orchestrator  ←──adapts──  internal/app/adapter  ──depends──►  loop
     │                                                           │
     │  (no import)                                              │
     └───────────────── X ──────────────────────────────────────┘
```

---

## Updated Data Flow

```
User Input (string)
    │
    ▼
TUI ──userCh──► Application::inputLoop()
                       │
             ┌─────────┴──────────┐
             │ "/" prefix          │ no "/"
             ▼                    ▼
    handleCommand()          runTask(description)
    (still in Application)        │
                                  │ wraps as orchestrator.RunRequest
                                  ▼
                         orchestrator.Run(ctx, req)
                                  │
                      ┌───────────┴───────────┐
                      │ ModeStandard           │ ModePlan
                      ▼                        ▼
               adapter.Run()            planner → adapter.Run() per step
                      │                        │
                      │  loop.Task ↔ RunRequest │
                      │  loop.Event ↔ RunEvent  │
                      ▼                        ▼
                 []RunEvent               []RunEvent
                      │                        │
                      └───────┬────────────────┘
                              ▼
                    convertRunEvent(RunEvent → model.Event)
                              │
                              ▼
                       EventCh → TUI
```

---

## Comments on the New Changes

### 1. Adapter is clean — but consider an interface extract

`engineAdapter` is a concrete struct in `internal/app`. This is fine for now, but if you ever need to test the orchestrator with a different backend (e.g., a mock that doesn't go through loop at all), the adapter pattern means you just implement `orchestrator.Engine` directly. Good.

One nit: `engineAdapter` holds `*loop.Engine` (concrete). If `loop.Engine` already has a public interface, prefer depending on that instead:

```go
// Instead of:
type engineAdapter struct {
    engine *loop.Engine  // concrete
}

// Consider:
type engineAdapter struct {
    engine interface {
        RunWithContext(ctx context.Context, task loop.Task) ([]loop.Event, error)
    }
}
```

This makes the adapter testable in isolation without constructing a full `loop.Engine`. Low priority — the current approach works.

### 2. Three event type sets remain — now it's worse, not better

Before: `loop.Event` types + `model.EventType` (2 sets, 1 mapping layer)
After: `loop.Event` types + `orchestrator.RunEvent` types + `model.EventType` (3 sets, 2 mapping layers)

The mapping chain is now:

```
loop.Event  ──adapter.Run()──►  orchestrator.RunEvent  ──convertRunEvent()──►  model.Event
   20+ consts                      7 consts                  11 consts
```

The adapter passes `ev.Type` (a string) through verbatim. The orchestrator only defines 7 event constants (`TaskStarted`, `TaskCompleted`, `TaskFailed`, `AgentThinking`, `AgentReply`, `LLMResponse`, `ToolError`) but the engine emits ~20 types (`ToolRead`, `ToolGrep`, `ToolGlob`, `ToolEdit`, `ToolWrite`, `CmdStarted`, `AnalysisReady`, `TokenUpdate`, etc.). These pass through the orchestrator as untyped strings — the orchestrator doesn't know they exist.

**This is the core problem:** The orchestrator claims to own the event vocabulary but actually doesn't. The `convertRunEvent` typeMap in `run.go` maps strings that the orchestrator never declared.

**Fix options (pick one):**

**Option A — Orchestrator declares the full vocabulary:**
```go
// orchestrator/types.go — add all event types the adapter might produce
const (
    EventToolRead      = "ToolRead"
    EventToolGrep      = "ToolGrep"
    EventToolGlob      = "ToolGlob"
    EventToolEdit      = "ToolEdit"
    EventToolWrite     = "ToolWrite"
    EventCmdStarted    = "CmdStarted"
    EventAnalysisReady = "AnalysisReady"
    EventTokenUpdate   = "TokenUpdate"
    // ...
)
```

Downside: orchestrator becomes aware of every tool, which defeats the purpose of decoupling.

**Option B — Shared event package (recommended):**
```go
// agent/event/types.go — single source of truth
package event

type Type string
const (
    AgentReply    Type = "AgentReply"
    ToolRead      Type = "ToolRead"
    // ... all in one place
)

type Event struct {
    Type       Type
    Message    string
    ToolName   string
    Summary    string
    CtxUsed    int
    CtxMax     int
    TokensUsed int
    Timestamp  time.Time
    Meta       map[string]any
}
```

Then `loop`, `orchestrator`, and `ui/model` all import `agent/event`. No adapters needed for the event type — only the request type needs adapting.

**Option C — Let the orchestrator pass opaque events:**
The orchestrator doesn't need to understand tool-level events. It only needs its own lifecycle events (`TaskStarted`, `TaskFailed`, etc.). Engine events pass through as `[]RunEvent` with no semantic interpretation. The orchestrator prepends/appends its own events. The UI maps the full set.

This is basically what you have now — just make it explicit by not defining tool-level constants in orchestrator at all. The `typeMap` in `convertRunEvent` becomes the single canonical mapping.

### 3. `RunRequest` is too thin — won't survive the next feature

`RunRequest` has only `ID` and `Description`. This is fine for "user typed text, run the LLM." But you'll need more soon:

- **Context/metadata**: session ID, message ID, parent request ID (for plan steps)
- **Attachments**: file references, images, code blocks
- **Continuation signals**: "user approved the plan", "user wants to abort step 3"
- **Mode overrides**: "run this one request in plan mode even though default is standard"

Consider making it extensible now:

```go
type RunRequest struct {
    ID          string
    Description string
    Context     map[string]string  // loop.Task already has this field
    Meta        map[string]any     // future-proof
}
```

At minimum, carry over `Context map[string]string` from `loop.Task` — the adapter currently drops it.

### 4. Slash commands still bypass the orchestrator

The previous review's item #1 is still open. `Application.processInput()` routes `/` commands directly to `handleCommand()` which mutates engine/orchestrator/config state. The orchestrator never sees these.

With the new `RunRequest` type, you could extend it:

```go
type RunRequest struct {
    ID          string
    Kind        RequestKind  // Chat, Command, Confirm, Cancel
    Description string
    Command     string       // for Kind=Command: "mode", "compact", "clear"
    Args        []string     // command arguments
    Context     map[string]string
}
```

Or keep `RunRequest` as-is and add a separate `Orchestrator.HandleCommand()` method. Either way, the orchestrator should be the single entry point.

### 5. The `convertRunEvent` map approach is better but fragile

The new table-driven approach is much cleaner than the old switch:

```go
typeMap := map[string]model.EventType{
    "AgentReply":    model.AgentReply,
    "AgentThinking": model.AgentThinking,
    // ...
}
```

But the map is rebuilt on every call (allocated on heap each time). Move it to a package-level var:

```go
var runEventToUIType = map[string]model.EventType{
    "AgentReply":    model.AgentReply,
    // ...
}
```

Also: if someone adds a new event type in `loop` but forgets to update this map, it silently falls through to the default case and becomes a generic `AgentReply`. Consider logging unknown event types during development.

### 6. `SetProvider` rebuilds everything — fragile

`SetProvider()` in `wire.go` now correctly preserves the mode via `a.Orchestrator.CurrentMode()` (good fix). But it rebuilds engine + adapter + orchestrator from scratch. If the orchestrator later holds state (pending plan approvals, in-flight requests), this will drop them.

Consider a `Reconfigure()` method on the orchestrator that hot-swaps the engine adapter without reconstructing the orchestrator itself.

---

## Remaining Items from Previous Review

| # | Item | Status |
|---|------|--------|
| 1 | Move command handling into orchestrator | **Still open** — commands bypass orchestrator |
| 2 | Unify event types into shared package | **Worse** — now 3 type sets instead of 2 |
| 3 | Switch from batch `[]Event` to streaming `chan Event` | **Still open** |
| 4 | Decouple orchestrator from loop types | **Done** — adapter pattern, RunRequest/RunEvent |
| 5 | Plan approval flow | **Still open** |

## Priority for Next Iteration

| # | Action | Why |
|---|--------|-----|
| 1 | Unify events into `agent/event` shared package | Eliminates 2 mapping layers, fixes the 3-type-set problem |
| 2 | Add `Context`/`Meta` to `RunRequest` | Currently drops `loop.Task.Context` in adapter |
| 3 | Move slash commands into orchestrator | Single entry point, enables tracing |
| 4 | Package-level var for event type map | Avoids per-call allocation |
| 5 | Streaming events via channel | Enables real-time UI feedback |
