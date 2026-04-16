package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mindspore-lab/mindspore-cli/internal/issues"
)

func visPad(s string, w int) string {
	visible := lipgloss.Width(s)
	if visible >= w {
		return s
	}
	return s + strings.Repeat(" ", w-visible)
}

func visTruncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	// Truncate rune by rune until it fits.
	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		candidate := string(runes[:i]) + "..."
		if lipgloss.Width(candidate) <= w {
			return candidate
		}
	}
	return "..."
}

func Dock(data *issues.DockData) string {
	lines := []string{
		TitleStyle.Render("DASHBOARD"),
		"",
		fmt.Sprintf("  %s %s    %s %s",
			LabelStyle.Render("open issues"),
			ValueStyle.Render(fmt.Sprintf("%d", data.OpenCount)),
			LabelStyle.Render("online (24h)"),
			ValueStyle.Render(fmt.Sprintf("%d", data.OnlineCount)),
		),
	}

	if len(data.ReadyIssues) > 0 {
		lines = append(lines, "", LabelStyle.Render("  ready (unassigned)"))
		for _, issue := range data.ReadyIssues {
			kindLabel := ""
			if issue.Kind != "" {
				kindLabel = " [" + string(issue.Kind) + "]"
			}
			lines = append(lines, fmt.Sprintf("    %s%s  %s  %s",
				issue.Key, kindLabel, issue.Title, StatusOpenStyle.Render(issue.Status)))
		}
	}

	if len(data.RecentFeed) > 0 {
		lines = append(lines, "", LabelStyle.Render("  recent activity"))
		for _, a := range data.RecentFeed {
			ts := a.CreatedAt.Format("01-02 15:04")
			lines = append(lines, ActivityStyle.Render(fmt.Sprintf("    %s  %s  %s", ts, a.Actor, a.Text)))
		}
	}

	return strings.Join(lines, "\n")
}
