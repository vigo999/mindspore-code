# Review: `refactor-arch-1` — Orchestration & Agent/Loop Architecture

## Current Data Flow

```
User Input (string)
    │
    ▼
ui/app.go ──userCh──► internal/app/run.go::inputLoop()
                              │
                    ┌─────────┴──────────┐
                    │ "/" prefix?         │ no "/"
                    ▼                    ▼
           handleCommand()         runTask(description)
           (slash cmds)                  │
                                         │ wraps as loop.Task{Description: str}
                                         ▼
                                 orchestrator.Run(ctx, task)
                                         │
                             ┌───────────┴───────────┐
                             │ ModeStandard           │ ModePlan
                             ▼                        ▼
                      engine.RunWithContext()    planner.Plan() → workflow/loop
                             │                        │
                             ▼                        ▼
                       []loop.Event             []loop.Event
                             │                        │
                             └───────┬────────────────┘
                                     ▼
                           convertEvent(loop.Event → model.Event)
                                     │
                                     ▼
                              EventCh → TUI
```

---

## Architecture Comments

### 1. The `string` → `loop.Task` boundary is too thin

Right now `Application.processInput()` in `run.go:75-85` makes the critical routing decision:

```go
func (a *Application) processInput(input string) {
    if strings.HasPrefix(trimmed, "/") {
        a.handleCommand(trimmed)  // slash commands bypass orchestrator entirely
        return
    }
    go a.runTask(trimmed)  // raw string → loop.Task
}
```

**Problem:** The `Application` layer is doing orchestration work (routing slash commands vs tasks) that should belong to the orchestrator. This means:
- Slash commands like `/mode`, `/compact`, `/clear` live in `Application` but directly mutate orchestrator/engine state
- The orchestrator never sees these commands — it can't log, trace, or intercept them
- Adding a new command type (e.g., multi-turn confirmation, plan approval, interrupt) requires modifying `Application`, not orchestrator

### 2. Recommendation: Use a common input message type — but keep it minimal

I recommend a thin `Input` union type that the orchestrator receives:

```go
// agent/orchestrator/input.go

// InputKind distinguishes what the user sent.
type InputKind int

const (
    InputChat    InputKind = iota // free-form text → LLM
    InputCommand                  // /mode, /clear, etc.
    InputConfirm                  // "yes"/"no" to a pending permission/plan approval
    InputCancel                   // ctrl+c / abort signal
)

// Input is the single entry point into the orchestrator.
type Input struct {
    Kind    InputKind
    Text    string            // raw text for Chat, command string for Command
    Args    []string          // parsed args for Command
    Meta    map[string]string // extensible (session ID, message ID, etc.)
}
```

Then `Orchestrator.Handle(ctx, Input) error` replaces both `Run()` and the slash-command switch in `Application`:

```go
func (o *Orchestrator) Handle(ctx context.Context, input Input) error {
    switch input.Kind {
    case InputChat:
        task := loop.Task{ID: generateID(), Description: input.Text}
        events, err := o.dispatch(ctx, task)
        // push events...
    case InputCommand:
        return o.handleCommand(input)
    case InputConfirm:
        return o.handleConfirmation(input)
    case InputCancel:
        return o.handleCancel(input)
    }
}
```

**Why this is better than letting orchestrator parse raw strings:**
- The TUI already knows whether input is a `/command` or chat — no need to re-parse inside orchestrator
- `InputKind` is an enum, not string parsing — impossible to miss a case
- `InputConfirm` gives you a clean path for plan approval and permission confirmation without hacking it through the chat channel
- The orchestrator can trace/log every input uniformly

**Why this is better than the current approach (Application handles commands):**
- Single responsibility: `Application` becomes a thin wiring layer, orchestrator owns all routing
- `/compact`, `/clear`, `/mode` become orchestrator-internal commands that can participate in tracing, hooks, and event streams
- New input types (file drag-and-drop, multi-turn confirmation) are added in one place

### 3. The two Event types are redundant — collapse them

You currently have `loop.Event` and `model.Event` with a 1:1 `convertEvent()` mapper (`run.go:105-175`). This is 70 lines of boilerplate that will grow with every new tool.

**Recommendation:** Define one `Event` type in a shared package (e.g., `agent/event`), used by both loop and UI. The TUI can filter/ignore event types it doesn't care about. The orchestrator can add events of its own (e.g., `PlanCreated`, `StepStarted`) without needing a parallel model.Event constant.

```go
// agent/event/event.go
package event

type Type string
const (
    AgentReply    Type = "AgentReply"
    ToolRead      Type = "ToolRead"
    // ...all in one place
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
    Meta       map[string]any  // extensible
}
```

### 4. Orchestrator → Engine coupling is too tight

`Orchestrator` holds a concrete `*loop.Engine` (via the `Engine` interface, but the interface is defined *inside* the orchestrator package). The `Engine` interface is:

```go
type Engine interface {
    RunWithContext(ctx context.Context, task loop.Task) ([]loop.Event, error)
}
```

This means the orchestrator must produce `loop.Task` and consume `loop.Event` — it's a pass-through that adds no value in standard mode.

**Better:** The orchestrator should own the `Task` and `Event` types. The engine should implement the orchestrator's interface, not the other way around. This inverts the dependency so the orchestrator is the top-level coordinator:

```
orchestrator.Input → orchestrator dispatches → engine (implements orchestrator.Runner)
                                              → planner
                                              → workflow runner
```

### 5. Plan mode: no approval flow

`PlanCallback.OnPlanCreated` / `OnPlanApproved` are called back-to-back in `runPlan()` with no user interaction in between:

```go
// orchestrator.go:103-108
if err := o.callback.OnPlanCreated(steps); err != nil { ... }
if err := o.callback.OnPlanApproved(steps); err != nil { ... }
```

There's no mechanism for the user to review, edit, or reject the plan. This is where `InputConfirm` in the proposed input model would help — `OnPlanCreated` would emit the plan to the UI, then the orchestrator suspends until it receives `InputConfirm{Kind: InputConfirm, Text: "yes"}`.

### 6. The `Application` struct is a god object

`Application` in `wire.go` holds: Engine, Orchestrator, EventCh, provider, toolRegistry, ctxManager, permService, stateManager, traceWriter, Config, WorkDir, RepoURL, Demo flag, llmReady flag...

It also handles:
- Slash commands (`commands.go`)
- Task execution (`run.go`)
- Provider hot-swapping (`SetProvider`)
- State persistence (`SaveState`)
- Demo mode (`runDemo`)

**Recommendation:** Split into:
- `Application` — wiring only (build dependencies, start TUI)
- `Orchestrator` — all runtime routing (commands + tasks + confirmations)
- `SessionManager` — state, persistence, provider switching

### 7. Batch events vs streaming

`engine.RunWithContext()` returns `[]loop.Event` — the entire run completes before any events reach the UI. This means:
- No streaming of partial LLM responses
- No real-time tool execution feedback
- The "thinking" indicator appears, then a wall of text appears all at once

The `EventCh chan model.Event` infrastructure is *already there* for streaming. The engine should push events to a channel as they happen, not batch them.

```go
// Instead of:
func (e *Engine) RunWithContext(ctx context.Context, task Task) ([]Event, error)

// Do:
func (e *Engine) RunWithContext(ctx context.Context, task Task, sink chan<- Event) error
```

---

## Summary: Answer to Your Design Question

**Use a common input type, but don't over-engineer it.**

The right boundary is:

```
TUI ──Input{Kind, Text, Args}──► Orchestrator ──► Engine/Planner/Workflow
                                      │
                                      ▼
                                 chan Event ◄── Engine pushes events as they happen
```

The orchestrator should be the **single entry point** for all user interactions — chat, commands, confirmations, cancellations. The TUI classifies input minimally (chat vs command vs confirm), and the orchestrator handles the rest.

Don't let the orchestrator parse raw strings. Don't let `Application` route commands around the orchestrator. The enum-based `InputKind` approach gives you type safety and extensibility without complexity.

## Priority Actions

| # | Change | Impact |
|---|--------|--------|
| 1 | Add `Input` type, move command handling into orchestrator | Unblocks plan approval, tracing |
| 2 | Unify `loop.Event` / `model.Event` into one shared type | Eliminates 70+ lines of mapping boilerplate |
| 3 | Switch engine from `[]Event` return to `chan Event` push | Enables streaming / real-time feedback |
| 4 | Split `Application` into wiring + orchestrator + session | Reduces god object |
| 5 | Add suspension/resumption for plan approval flow | Completes plan mode |
