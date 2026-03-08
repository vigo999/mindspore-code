package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/components"
	"github.com/vigo999/ms-cli/ui/model"
	"github.com/vigo999/ms-cli/ui/panels"
	"github.com/vigo999/ms-cli/ui/slash"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	topBarHeight   = 3 // brand line + info line + divider
	chatLineHeight = 2
	hintBarHeight  = 2
	inputHeight    = 1
	verticalPad    = 2
)

var chatLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))

type appViewMode int

const (
	viewModeChat appViewMode = iota
	viewModeTrain
)

// App is the TUI root model.
type App struct {
	state          model.State
	train          model.TrainDashboard
	trainChatStart int
	trainCopyMode  bool
	trainSnapshot  string
	viewMode       appViewMode
	viewport       components.Viewport
	trainViewport  components.Viewport
	input          components.TextInput
	thinking       components.ThinkingSpinner
	width          int
	height         int
	eventCh        <-chan model.Event
	userCh         chan<- string // sends user input to the engine bridge
	lastInterrupt  time.Time     // track last ctrl+c for double-press exit
	trainSlash     *slash.Registry
}

// New creates a new App driven by the given event channel.
// userCh may be nil (demo mode) — user input won't be forwarded.
func New(ch <-chan model.Event, userCh chan<- string, version, workDir, repoURL, modelName string, ctxMax int) App {
	return App{
		state:         model.NewState(version, workDir, repoURL, modelName, ctxMax),
		train:         model.NewTrainDashboard(),
		viewMode:      viewModeChat,
		viewport:      components.NewViewport(1, 1),
		trainViewport: components.NewViewport(1, 1),
		input:         components.NewTextInput(),
		thinking:      components.NewThinkingSpinner(),
		eventCh:       ch,
		userCh:        userCh,
		trainSlash:    slash.DefaultRegistry.Without("/train"),
	}
}

func (a App) waitForEvent() tea.Msg {
	ev, ok := <-a.eventCh
	if !ok {
		return model.Event{Type: model.Done}
	}
	return ev
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.thinking.Tick(),
		a.waitForEvent,
	)
}

func (a App) chatHeight() int {
	h := a.height - topBarHeight - chatLineHeight - hintBarHeight - inputHeight - verticalPad
	// Adjust for input height (including suggestions)
	inputH := a.input.Height()
	if inputH > 1 {
		h -= (inputH - 1)
	}
	if h < 1 {
		return 1
	}
	return h
}

func (a App) trainBodyHeight() int {
	h := a.height - topBarHeight - hintBarHeight
	if h < 8 {
		return 8
	}
	return h
}

func (a App) trainChatLayout() panels.TrainEmbeddedChatLayout {
	return panels.ResolveTrainEmbeddedChatLayout(a.train, a.width, a.trainBodyHeight(), a.trainCopyMode)
}

func (a App) isTrainChatActive() bool {
	return a.viewMode == viewModeTrain && !a.trainCopyMode && a.trainChatLayout().Active
}

func (a App) trainChatViewportSize(layout panels.TrainEmbeddedChatLayout) (int, bool) {
	if !layout.Active {
		return 0, false
	}
	inputH := a.input.Height()
	if layout.Height <= inputH {
		return 0, false
	}
	remaining := layout.Height - inputH
	if remaining >= 2 {
		return remaining - 1, true
	}
	return remaining, false
}

func (a *App) syncInputMode() {
	registry := slash.DefaultRegistry
	inputWidth := a.width - 6
	if inputWidth < 12 {
		inputWidth = 12
	}

	if a.viewMode == viewModeTrain {
		layout := a.trainChatLayout()
		if layout.Active && !a.trainCopyMode {
			registry = a.trainSlash
			inputWidth = layout.Width
			if inputWidth < 12 {
				inputWidth = 12
			}
		}
	}

	a.input = a.input.WithSlashRegistry(registry)
	a.input.Model.Width = inputWidth
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		return a.handleKey(msg)

	case tea.MouseMsg:
		if !a.state.MouseEnabled {
			return a, nil
		}
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.syncInputMode()
		a.updateViewport()
		return a, nil

	case model.Event:
		return a.handleEvent(msg)

	default:
		var cmd tea.Cmd
		a.thinking, cmd = a.thinking.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// 重新渲染 viewport 以显示动画
		a.updateViewport()
	}

	return a, tea.Batch(cmds...)
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.viewMode == viewModeTrain {
		return a.handleTrainKey(msg)
	}

	// Check if we're in slash suggestion mode
	if a.input.IsSlashMode() {
		switch msg.String() {
		case "up", "down", "tab", "enter", "esc":
			// Let input handle these for suggestion navigation
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			a.syncInputMode()
			a.updateViewport()
			return a, cmd
		}
	}

	switch msg.String() {
	case "ctrl+c":
		now := time.Now()
		// If last ctrl+c was within 1 second, quit
		if now.Sub(a.lastInterrupt) < time.Second {
			return a, tea.Quit
		}
		// Otherwise, cancel current input and show hint
		a.lastInterrupt = now
		a.input = a.input.Reset()
		a.state = a.state.WithMessage(model.Message{
			Kind:    model.MsgAgent,
			Content: "⚠️  Interrupted. Press Ctrl+C again within 1 second to exit.",
		})
		a.updateViewport()
		return a, nil

	case "enter":
		// Don't process enter if in slash mode (handled above)
		if a.input.IsSlashMode() {
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			a.syncInputMode()
			a.updateViewport()
			return a, cmd
		}

		val := a.input.Value()
		if val == "" {
			return a, nil
		}
		a.submitInput(val, false)
		return a, nil

	case "pgup", "pgdown", "home", "end":
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	case "up", "down":
		// Only scroll chat if not in input at top/bottom or if shift is held
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	default:
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		a.syncInputMode()
		a.updateViewport()
		return a, cmd
	}
}

func (a App) handleTrainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		now := time.Now()
		if now.Sub(a.lastInterrupt) < time.Second {
			return a, tea.Quit
		}
		a.lastInterrupt = now
		return a, nil
	case "ctrl+y":
		a.trainCopyMode = !a.trainCopyMode
		if a.trainCopyMode {
			a.trainSnapshot = a.renderTrainView()
		} else {
			a.trainSnapshot = ""
		}
		a.syncInputMode()
		a.updateViewport()
		return a, nil
	case "ctrl+r":
		if a.train.Status != "failed" {
			return a, nil
		}
		a.trainCopyMode = false
		a.trainSnapshot = ""
		a.syncInputMode()
		a.updateViewport()
		if a.userCh != nil {
			select {
			case a.userCh <- "/train retry":
			default:
			}
		}
		return a, nil
	}

	if a.trainCopyMode {
		switch msg.String() {
		case "esc":
			return a.leaveTrainView()
		default:
			return a, nil
		}
	}

	if a.isTrainChatActive() {
		return a.handleTrainChatKey(msg)
	}

	switch msg.String() {
	case "esc":
		return a.leaveTrainView()
	default:
		return a, nil
	}
}

func (a App) handleTrainChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.input.IsSlashMode() {
		switch msg.String() {
		case "up", "down", "tab", "enter", "esc":
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			a.syncInputMode()
			a.updateViewport()
			return a, cmd
		}
	}

	switch msg.String() {
	case "enter":
		if a.input.IsSlashMode() {
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			a.syncInputMode()
			a.updateViewport()
			return a, cmd
		}
		val := a.input.Value()
		if val == "" {
			return a, nil
		}
		a.submitInput(val, true)
		return a, nil

	case "pgup", "pgdown", "home", "end", "up", "down":
		var cmd tea.Cmd
		a.trainViewport, cmd = a.trainViewport.Update(msg)
		return a, cmd

	case "esc":
		if strings.TrimSpace(a.input.Value()) == "" {
			return a.leaveTrainView()
		}
		return a, nil

	default:
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		a.syncInputMode()
		a.updateViewport()
		return a, cmd
	}
}

func (a *App) submitInput(raw string, trainChat bool) {
	val := strings.TrimRight(raw, "\n")
	if strings.TrimSpace(val) == "" {
		return
	}

	a.state = a.state.ResetStats()
	a.state = a.state.WithThinking(false)
	a.state = a.state.WithMessage(model.Message{Kind: model.MsgUser, Content: val})
	a.input = a.input.Reset()
	a.syncInputMode()
	a.updateViewport()

	routed, ok := a.routeSubmittedInput(val, trainChat)
	if !ok || a.userCh == nil {
		return
	}
	select {
	case a.userCh <- routed:
	default:
	}
}

func (a *App) routeSubmittedInput(val string, trainChat bool) (string, bool) {
	trimmed := strings.TrimSpace(val)
	if !trainChat {
		return val, true
	}
	if strings.HasPrefix(trimmed, "/train") {
		a.state = a.state.WithMessage(model.Message{
			Kind:    model.MsgAgent,
			Content: "Embedded /train chat does not accept `/train` slash commands. Ask in natural language to rerun or stop training, or press Esc to return to the main chat.",
		})
		a.updateViewport()
		return "", false
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed, true
	}
	return model.TrainChatInputPrefix + trimmed, true
}

func (a App) leaveTrainView() (tea.Model, tea.Cmd) {
	a.trainCopyMode = false
	a.trainSnapshot = ""
	a.viewMode = viewModeChat
	a.syncInputMode()
	a.updateViewport()
	if a.state.MouseEnabled {
		return a, tea.EnableMouseCellMotion
	}
	return a, nil
}

func (a App) handleEvent(ev model.Event) (tea.Model, tea.Cmd) {
	var eventCmd tea.Cmd

	switch ev.Type {
	case model.AgentThinking:
		// Start thinking - set flag and ensure we have a thinking message
		a.state = a.state.WithThinking(true)
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgThinking})

	case model.AgentReply:
		// Stop thinking and show result
		a.state = a.state.WithThinking(false)
		a.state = a.replaceThinking(model.Message{Kind: model.MsgAgent, Content: ev.Message})

	case model.CmdStarted:
		// Update command count
		stats := a.state.Stats
		stats.Commands++
		a.state = a.state.WithStats(stats)
		// Shell tool message already contains the full output with $ prefix
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Shell",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.CmdOutput:
		a.state = a.appendToLastTool(ev.Message)

	case model.CmdFinished:
		// output already in the tool block

	case model.ToolRead:
		// Update files read count
		stats := a.state.Stats
		stats.FilesRead++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Read",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolGrep:
		// Update search count
		stats := a.state.Stats
		stats.Searches++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Grep",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolGlob:
		// Update search count
		stats := a.state.Stats
		stats.Searches++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Glob",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolEdit:
		// Update files edited count
		stats := a.state.Stats
		stats.FilesEdited++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Edit",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolWrite:
		// Update files edited count
		stats := a.state.Stats
		stats.FilesEdited++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Write",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolError:
		// Update error count
		stats := a.state.Stats
		stats.Errors++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: ev.ToolName,
			Display:  model.DisplayError,
			Content:  ev.Message,
		})

	case model.AnalysisReady:
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgAgent, Content: ev.Message})

	case model.TokenUpdate:
		mi := a.state.Model
		mi.CtxUsed = ev.CtxUsed
		mi.CtxMax = ev.CtxMax
		mi.TokensUsed = ev.TokensUsed
		a.state = a.state.WithModel(mi)

	case model.TaskUpdated:
		// no-op for now

	case model.ClearScreen:
		// Clear all messages and add the notification
		a.state.Messages = []model.Message{
			{Kind: model.MsgAgent, Content: ev.Message},
		}

	case model.ModelUpdate:
		// Update model name in top bar
		mi := a.state.Model
		mi.Name = ev.Message
		a.state = a.state.WithModel(mi)

	case model.MouseModeToggle:
		enabled := a.state.MouseEnabled
		switch strings.ToLower(strings.TrimSpace(ev.Message)) {
		case "", "toggle":
			enabled = !enabled
		case "on", "enable", "enabled", "true", "1":
			enabled = true
		case "off", "disable", "disabled", "false", "0":
			enabled = false
		}
		a.state = a.state.WithMouseEnabled(enabled)
		if enabled {
			eventCmd = tea.EnableMouseCellMotion
		} else {
			eventCmd = tea.DisableMouse
		}

	case model.TrainUpdateEvent:
		if ev.Train != nil {
			a.train.Apply(*ev.Train)
			if ev.Train.Kind == model.TrainUpdateOpen {
				a.trainCopyMode = false
				a.trainSnapshot = ""
				a.trainChatStart = len(a.state.Messages)
				a.viewMode = viewModeTrain
				eventCmd = tea.DisableMouse
			}
			if ev.Train.Kind == model.TrainUpdateClose {
				a.trainCopyMode = false
				a.trainSnapshot = ""
				a.trainChatStart = len(a.state.Messages)
				a.viewMode = viewModeChat
				if a.state.MouseEnabled {
					eventCmd = tea.EnableMouseCellMotion
				}
			}
		}

	case model.Done:
		return a, tea.Quit
	}

	a.syncInputMode()
	a.updateViewport()
	if eventCmd != nil {
		return a, tea.Batch(eventCmd, a.waitForEvent)
	}
	return a, a.waitForEvent
}

func (a App) replaceThinking(m model.Message) model.State {
	msgs := make([]model.Message, 0, len(a.state.Messages))
	for _, msg := range a.state.Messages {
		if msg.Kind != model.MsgThinking {
			msgs = append(msgs, msg)
		}
	}
	msgs = append(msgs, m)
	next := a.state
	next.Messages = msgs
	return next
}

func (a App) appendToLastTool(line string) model.State {
	msgs := make([]model.Message, len(a.state.Messages))
	copy(msgs, a.state.Messages)

	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Kind == model.MsgTool {
			msgs[i] = model.Message{
				Kind:     model.MsgTool,
				ToolName: msgs[i].ToolName,
				Display:  msgs[i].Display,
				Content:  msgs[i].Content + "\n" + line,
			}
			break
		}
	}

	next := a.state
	next.Messages = msgs
	return next
}

func (a *App) updateViewport() {
	content := panels.RenderMessages(a.state, a.thinking.View())
	if a.width > 0 {
		a.viewport = a.viewport.SetSize(a.width-4, a.chatHeight())
	}
	a.viewport = a.viewport.SetContent(content)

	layout := a.trainChatLayout()
	if !layout.Active {
		a.trainViewport = a.trainViewport.SetSize(1, 1)
		a.trainViewport = a.trainViewport.SetContent(content)
		return
	}

	viewportHeight, _ := a.trainChatViewportSize(layout)
	if viewportHeight <= 0 {
		viewportHeight = 1
	}
	a.trainViewport = a.trainViewport.SetSize(layout.Width, viewportHeight)
	a.trainViewport = a.trainViewport.SetContent(panels.RenderMessages(a.trainChatState(), a.thinking.View()))
}

func (a App) trainChatState() model.State {
	start := a.trainChatStart
	if start < 0 {
		start = 0
	}
	if start > len(a.state.Messages) {
		start = len(a.state.Messages)
	}

	next := a.state
	next.Messages = make([]model.Message, 0, len(a.state.Messages)-start)
	for _, msg := range a.state.Messages[start:] {
		switch msg.Kind {
		case model.MsgUser, model.MsgAgent, model.MsgThinking:
			next.Messages = append(next.Messages, msg)
		}
	}
	return next
}

func (a App) chatLine() string {
	return chatLineStyle.Render(strings.Repeat("─", a.width))
}

func (a App) View() string {
	if a.viewMode == viewModeTrain {
		if a.trainCopyMode && a.trainSnapshot != "" {
			return a.trainSnapshot
		}
		return a.renderTrainView()
	}

	topBar := panels.RenderTopBar(a.state, a.width)
	line := a.chatLine()
	chat := a.viewport.View()
	input := "  " + a.input.View()
	hintBar := panels.RenderHintBar(a.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		line,
		chat,
		line,
		input,
		hintBar,
	)
}

func (a App) renderTrainView() string {
	topBar := panels.RenderTopBar(a.state, a.width)
	bodyHeight := a.trainBodyHeight()
	var lowerPanel *panels.TrainLowerPanel
	layout := a.trainChatLayout()
	if layout.Active && !a.trainCopyMode {
		lowerPanel = &panels.TrainLowerPanel{
			Title: layout.Title,
			Body:  a.renderTrainChatBody(layout),
		}
	}
	dashboard := panels.RenderTrainDashboard(a.train, a.width, bodyHeight, a.trainCopyMode, lowerPanel)
	hintBar := panels.RenderTrainHintBar(a.width, a.trainCopyMode, a.train.Status == "failed", layout.Active && !a.trainCopyMode)
	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		dashboard,
		hintBar,
	)
}

func (a App) renderTrainChatBody(layout panels.TrainEmbeddedChatLayout) string {
	viewportHeight, showDivider := a.trainChatViewportSize(layout)
	parts := make([]string, 0, 3)
	if viewportHeight > 0 {
		parts = append(parts, a.trainViewport.View())
	}
	if showDivider {
		parts = append(parts, chatLineStyle.Render(strings.Repeat("─", layout.Width)))
	}
	parts = append(parts, a.input.View())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
