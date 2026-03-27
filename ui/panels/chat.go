package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/model"
)

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	agentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Italic(true)

	// expanded tool block
	toolBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	toolHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	toolContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				PaddingLeft(2)

	// collapsed tool (dim, single line)
	collapsedIconStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	collapsedNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	collapsedTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true)

	collapsedSummaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true)

	// error tool block
	errorBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	errorHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	errorContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("203")).
				PaddingLeft(2)

	// edit/write diff styles
	diffAddStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114"))

	diffRemoveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	diffNeutralStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				PaddingLeft(2)

	toolPendingDotStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
	toolSuccessDotStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("114"))
	toolWarningDotStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))
	toolErrorDotStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))
	toolCallLineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	toolPendingStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Italic(true)
	toolResultPrefixStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))
	toolResultSummaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))
	toolResultDetailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
	toolResultWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))
	toolResultErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("203"))
)

// RenderMessages converts messages into styled text for the viewport.
// compact uses single-line spacing (for the train agent box).
func RenderMessages(state model.State, spinnerView, spinnerFrame string, width int, compact ...bool) string {
	var parts []string
	messages := state.Messages
	if width < 12 {
		width = 12
	}

	for _, m := range messages {
		switch m.Kind {
		case model.MsgUser:
			parts = append(parts, renderUserMsg(m.Content, width))
		case model.MsgAgent:
			parts = append(parts, renderAgentMsg(m.Content, width))
		case model.MsgTool:
			parts = append(parts, renderTool(state, m, spinnerFrame, width))
		}
	}

	if state.IsThinking {
		parts = append(parts, renderThinking(spinnerView, width))
	}

	sep := "\n\n"
	if len(compact) > 0 && compact[0] {
		sep = "\n"
	}
	return strings.Join(parts, sep)
}

func renderUserMsg(content string, width int) string {
	return renderPrefixedBlock(userStyle.Render(content), width, "  "+userStyle.Render(">")+" ", "    ")
}

func renderAgentMsg(content string, width int) string {
	return renderPrefixedBlock(agentStyle.Render(content), width, "  ", "  ")
}

func renderThinking(thinkingView string, width int) string {
	// Animated thinking indicator with Braille spinner
	// thinkingView already contains the spinner and text from ThinkingSpinner.View()
	return renderPrefixedBlock(thinkingView, width, "  ", "  ")
}

func renderTool(state model.State, m model.Message, spinnerFrame string, width int) string {
	call := renderToolCallLine(state, m, spinnerFrame)
	if m.Pending {
		return renderPrefixedBlock(call, width, "  ", "  ")
	}
	summary, details := toolResult(m)
	if summary == "" && len(details) == 0 {
		return renderPrefixedBlock(call, width, "  ", "  ")
	}
	lines := []string{call}
	if summary != "" {
		lines = append(lines, "  "+toolResultPrefixStyle.Render("⎿")+"  "+renderToolSummary(m, summary))
	}
	for _, line := range details {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, "      "+renderToolDetail(m, line))
	}
	return renderPrefixedBlock(strings.Join(lines, "\n"), width, "  ", "  ")
}

func renderToolCallLine(state model.State, m model.Message, spinnerFrame string) string {
	dot := toolPendingDotStyle.Render("⏺")
	suffix := ""
	switch {
	case m.Pending:
		if strings.TrimSpace(spinnerFrame) != "" && state.WaitKind == model.WaitTool {
			dot = spinnerFrame
		} else {
			dot = toolPendingDotStyle.Render("⏺")
		}
		suffix = renderPendingToolStatus(state, m)
	case m.Display == model.DisplayWarning:
		dot = toolWarningDotStyle.Render("⏺")
	case m.Display == model.DisplayError:
		dot = toolErrorDotStyle.Render("⏺")
	default:
		dot = toolSuccessDotStyle.Render("⏺")
	}
	return toolCallLineStyle.Render(dot+" "+strings.TrimSpace(m.ToolName)+"("+strings.TrimSpace(toolCallArgs(m))+")") + suffix
}

func renderPendingToolStatus(state model.State, m model.Message) string {
	status := strings.TrimSpace(m.Summary)
	if status == "" {
		status = "running..."
	}
	if state.WaitKind == model.WaitTool && !state.WaitStartedAt.IsZero() {
		status += " " + model.FormatWaitDuration(time.Since(state.WaitStartedAt))
	}
	return " " + toolPendingStatusStyle.Render(status)
}

func toolCallArgs(m model.Message) string {
	args := strings.TrimSpace(m.ToolArgs)
	if args == "" {
		args = strings.TrimSpace(toolHeadline(m.Content))
	}
	if args == "" {
		args = "none"
	}
	return args
}

func toolResult(m model.Message) (string, []string) {
	lines := nonEmptyLines(m.Content)
	summary := strings.TrimSpace(m.Summary)
	if summary == "" && len(lines) > 0 {
		summary = lines[0]
		lines = lines[1:]
	} else if summary != "" && len(lines) > 0 && strings.TrimSpace(lines[0]) == summary {
		lines = lines[1:]
	}
	return summary, lines
}

func renderToolSummary(m model.Message, line string) string {
	if m.Display == model.DisplayWarning {
		return toolResultWarningStyle.Render(line)
	}
	if m.Display == model.DisplayError {
		return toolResultErrorStyle.Render(line)
	}
	return toolResultSummaryStyle.Render(line)
}

func renderToolDetail(m model.Message, line string) string {
	if m.Display == model.DisplayWarning {
		return toolResultWarningStyle.Render(line)
	}
	if m.Display == model.DisplayError {
		return toolResultErrorStyle.Render(line)
	}
	return toolResultDetailStyle.Render(line)
}

func toolHeadline(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		headline := strings.TrimSpace(line)
		if headline != "" {
			return headline
		}
	}
	return ""
}

func nonEmptyLines(content string) []string {
	raw := strings.Split(strings.TrimSpace(content), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func renderPrefixedBlock(content string, width int, firstPrefix, restPrefix string) string {
	prefixWidth := lipgloss.Width(firstPrefix)
	if w := lipgloss.Width(restPrefix); w > prefixWidth {
		prefixWidth = w
	}
	bodyWidth := width - prefixWidth
	if bodyWidth < 1 {
		bodyWidth = 1
	}
	wrapped := lipgloss.NewStyle().Width(bodyWidth).Render(content)
	lines := strings.Split(wrapped, "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = firstPrefix + lines[i]
			continue
		}
		lines[i] = restPrefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func renderToolHeader(icon, title string, borderStyle, titleStyle lipgloss.Style, width int) string {
	dividerWidth := width - lipgloss.Width(title) - 6
	if dividerWidth < 6 {
		dividerWidth = 6
	}
	return fmt.Sprintf("  %s %s %s",
		borderStyle.Render(icon),
		titleStyle.Render(title),
		borderStyle.Render(strings.Repeat("─", dividerWidth)),
	)
}

func maxBodyWidth(width int) int {
	if width < 1 {
		return 1
	}
	return width
}
