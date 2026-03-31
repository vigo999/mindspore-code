package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/mindspore-code/ui/model"
	"github.com/vigo999/mindspore-code/ui/panels"
)

var (
	inlineBannerBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Align(lipgloss.Left).
				Padding(1, 2)

	inlineBadgeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("63")).
				Foreground(lipgloss.Color("255")).
				Bold(true).
				Padding(0, 1)

	inlineTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true)

	inlineModeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")).
			Italic(true)

	inlineLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	inlineValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	inlineCmdOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	inlineCmdStderrStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("203"))

	inlineMetaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)
)

// RenderInlineBanner renders the one-shot banner shown after boot in inline mode.
func RenderInlineBanner(version, workDir, repoURL, modelName string, ctxMax int) string {
	title := lipgloss.JoinHorizontal(
		lipgloss.Left,
		inlineBadgeStyle.Render("MS"),
		" ",
		inlineTitleStyle.Render("MindSpore Code"),
		"  ",
		inlineModeStyle.Render("inline mode"),
	)

	rows := []string{
		inlineBannerRow("Version", valueOrString(strings.TrimSpace(version), "unknown")),
		inlineBannerRow("Model", valueOrString(strings.TrimSpace(modelName), "unknown")),
		inlineBannerRow("Directory", valueOrString(shortenInlinePath(strings.TrimSpace(workDir)), ".")),
		inlineBannerRow("Context", formatInlineContext(ctxMax)),
	}
	if repo := strings.TrimSpace(repoURL); repo != "" {
		rows = append(rows, inlineBannerRow("Repo", repo))
	}

	body := title + "\n\n" + strings.Join(rows, "\n")
	return inlineBannerBoxStyle.Render(body)
}

func inlineBannerRow(label, value string) string {
	return inlineLabelStyle.Render(label+":") + " " + inlineValueStyle.Render(value)
}

func formatInlineContext(ctxMax int) string {
	if ctxMax <= 0 {
		return "-"
	}
	return strconv.Itoa(ctxMax) + " tokens"
}

func shortenInlinePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func (a App) inlineRenderWidth() int {
	if a.width > 8 {
		return a.width - 4
	}
	return 96
}

func (a App) renderInlineMainView() string {
	parts := []string{}
	if a.trainView.Active {
		parts = append(parts, panels.RenderTrainHUD(a.trainView, a.width, a.agentStatus()))
	}
	if status := a.inlineStatusView(); status != "" {
		parts = append(parts, status)
	}
	if len(a.queuedInputs) > 0 {
		parts = append(parts, queueBannerStyle.Render("messages queued (press esc to interrupt)"))
	}
	parts = append(parts, a.input.View())
	if a.trainView.Active {
		parts = append(parts, panels.RenderTrainHUDHintBar(a.width))
	} else {
		parts = append(parts, panels.RenderHintBar(a.state, a.width))
	}
	return trimViewHeight(lipgloss.JoinVertical(lipgloss.Left, parts...), a.height, false)
}

func (a App) inlineStatusView() string {
	return strings.TrimSpace(a.inlineActivePreview())
}

func (a App) inlineActivePreview() string {
	if msg, ok := a.inlineLastStreamingAgent(); ok {
		return tailInlineLines(a.renderInlineTranscriptMessage(msg), 8)
	}
	if msg, ok := a.inlineLastActiveTool(); ok {
		return tailInlineLines(a.renderInlineTranscriptMessage(msg), 8)
	}
	if a.state.WaitKind == model.WaitModel {
		return a.thinking.View()
	}
	if a.state.WaitKind == model.WaitTool {
		return inlineMetaStyle.Render("waiting for tool result...")
	}
	return ""
}

func (a App) renderInlineTranscriptMessage(msg model.Message) string {
	temp := model.NewState("", "", "", "", 0)
	temp.Messages = []model.Message{msg}
	temp.WaitKind = a.state.WaitKind
	temp.WaitStartedAt = a.state.WaitStartedAt
	return panels.RenderMessages(temp, a.thinking.View(), a.thinking.FrameView(), a.inlineRenderWidth(), a.trainView.Active)
}

func (a App) inlinePrintMessage(msg model.Message) tea.Cmd {
	rendered := strings.TrimSpace(a.renderInlineTranscriptMessage(msg))
	if rendered == "" {
		return nil
	}
	return tea.Println(rendered)
}

func (a App) inlinePrintUserInput(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	return a.inlinePrintMessage(model.Message{Kind: model.MsgUser, Content: input})
}

func (a App) inlinePrintResolvedTool(ev model.Event) tea.Cmd {
	msg, ok := a.inlineResolvedToolMessage(ev)
	if !ok {
		return nil
	}
	return a.inlinePrintMessage(msg)
}

func (a App) inlinePrintShellFinished(ev model.Event, before []model.Message) tea.Cmd {
	prevMsg, hadPrev := inlineFindToolMessage(before, ev.ToolCallID)
	if hadPrev && strings.TrimSpace(prevMsg.Content) != "" {
		summary := strings.TrimSpace(ev.Summary)
		if summary == "" || summary == "completed" {
			return nil
		}
		return tea.Println(inlineMetaStyle.Render("shell " + summary))
	}

	msg, ok := a.inlineResolvedToolMessage(ev)
	if !ok {
		return nil
	}
	if strings.TrimSpace(msg.Content) == "(No output)" && strings.TrimSpace(msg.Summary) == "completed" {
		return nil
	}
	return a.inlinePrintMessage(msg)
}

func (a App) inlinePrintCommandOutput(chunk string) tea.Cmd {
	chunk = strings.ReplaceAll(chunk, "\r\n", "\n")
	chunk = strings.TrimSuffix(chunk, "\n")
	if strings.TrimSpace(chunk) == "" {
		return nil
	}

	lines := strings.Split(chunk, "\n")
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		style := inlineCmdOutputStyle
		if strings.HasPrefix(strings.TrimSpace(line), "[stderr]") {
			style = inlineCmdStderrStyle
		}
		rendered = append(rendered, "      "+style.Render(line))
	}
	return tea.Println(strings.Join(rendered, "\n"))
}

func (a App) inlineFallbackPrint(prevLen int) tea.Cmd {
	if prevLen < 0 || prevLen > len(a.state.Messages) {
		prevLen = len(a.state.Messages)
	}
	var cmds []tea.Cmd
	for i := prevLen; i < len(a.state.Messages); i++ {
		msg := a.state.Messages[i]
		if msg.Kind == model.MsgUser || msg.Pending || msg.Streaming {
			continue
		}
		cmds = append(cmds, a.inlinePrintMessage(msg))
	}
	return combineCmds(cmds...)
}

func (a App) inlineEventCmd(ev model.Event, prevMessages []model.Message) tea.Cmd {
	prevLen := len(prevMessages)

	switch ev.Type {
	case model.UserInput:
		return a.inlinePrintUserInput(ev.Message)
	case model.AgentReply:
		if msg, ok := a.inlineLastStableAgent(); ok {
			return a.inlinePrintMessage(msg)
		}
		return a.inlinePrintMessage(model.Message{Kind: model.MsgAgent, Content: ev.Message})
	case model.AgentReplyDelta, model.AgentThinking, model.TaskDone, model.TokenUpdate:
		return nil
	case model.ToolCallStart:
		return a.inlinePrintMessage(a.pendingToolMessage(ev))
	case model.CmdOutput:
		return a.inlinePrintCommandOutput(ev.Message)
	case model.CmdFinished:
		return a.inlinePrintShellFinished(ev, prevMessages)
	case model.ToolRead, model.ToolGrep, model.ToolGlob, model.ToolEdit, model.ToolWrite, model.ToolSkill, model.ToolWarning, model.ToolError, model.ToolReplay:
		return a.inlinePrintResolvedTool(ev)
	case model.ClearScreen:
		return inlineClearMessage()
	default:
		return a.inlineFallbackPrint(prevLen)
	}
}

func (a App) inlineResolvedToolMessage(ev model.Event) (model.Message, bool) {
	if msg, ok := inlineFindToolMessage(a.state.Messages, ev.ToolCallID); ok {
		return msg, true
	}
	for i := len(a.state.Messages) - 1; i >= 0; i-- {
		if a.state.Messages[i].Kind == model.MsgTool && !a.state.Messages[i].Pending {
			return a.state.Messages[i], true
		}
	}
	return model.Message{}, false
}

func (a App) inlineLastStableAgent() (model.Message, bool) {
	for i := len(a.state.Messages) - 1; i >= 0; i-- {
		msg := a.state.Messages[i]
		if msg.Kind == model.MsgAgent && !msg.Streaming {
			return msg, true
		}
	}
	return model.Message{}, false
}

func inlineFindToolMessage(messages []model.Message, toolCallID string) (model.Message, bool) {
	if idx := toolMessageIndex(messages, toolCallID); idx >= 0 {
		return messages[idx], true
	}
	return model.Message{}, false
}

func (a App) inlineLastStreamingAgent() (model.Message, bool) {
	for i := len(a.state.Messages) - 1; i >= 0; i-- {
		msg := a.state.Messages[i]
		if msg.Kind == model.MsgAgent && msg.Streaming {
			return msg, true
		}
	}
	return model.Message{}, false
}

func (a App) inlineLastActiveTool() (model.Message, bool) {
	for i := len(a.state.Messages) - 1; i >= 0; i-- {
		msg := a.state.Messages[i]
		if msg.Kind == model.MsgTool && (msg.Pending || msg.Streaming) {
			return msg, true
		}
	}
	return model.Message{}, false
}

func tailInlineLines(content string, limit int) string {
	if limit <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) <= limit {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-limit:], "\n")
}

func combineCmds(cmds ...tea.Cmd) tea.Cmd {
	filtered := make([]tea.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			filtered = append(filtered, cmd)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}

func inlineClearMessage() tea.Cmd {
	return tea.Println(inlineMetaStyle.Render("conversation cleared"))
}

func inlineTimestampLabel(now time.Time) string {
	return inlineMetaStyle.Render(fmt.Sprintf("[%s]", now.Format("15:04:05")))
}
