package panels

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/model"
)

var (
	projectHUDTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))
	projectHUDKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))
	projectHUDValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	projectHUDDirtyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220"))
	projectHUDCleanStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("114"))
)

func RenderProjectHUD(status model.ProjectStatusView, width int) string {
	if width < 1 {
		width = 1
	}

	name := status.Name
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(status.Root)
	}
	branch := status.Branch
	if strings.TrimSpace(branch) == "" {
		branch = "-"
	}
	summary := status.Summary
	if strings.TrimSpace(summary) == "" {
		summary = "project status unavailable"
	}

	stateText := "clean"
	stateStyle := projectHUDCleanStyle
	if status.Dirty {
		stateText = "dirty"
		stateStyle = projectHUDDirtyStyle
	}

	line1 := fmt.Sprintf(
		"%s  %s %s  %s %s",
		projectHUDTitleStyle.Render("project status"),
		projectHUDKeyStyle.Render("name"),
		projectHUDValueStyle.Render(name),
		projectHUDKeyStyle.Render("branch"),
		projectHUDValueStyle.Render(branch),
	)

	line2 := fmt.Sprintf(
		"  %s %s  %s %s  %s %s",
		projectHUDKeyStyle.Render("state"),
		stateStyle.Render(stateText),
		projectHUDKeyStyle.Render("changes"),
		projectHUDValueStyle.Render(fmt.Sprintf("staged:%d modified:%d untracked:%d", status.Staged, status.Modified, status.Untracked)),
		projectHUDKeyStyle.Render("sync"),
		projectHUDValueStyle.Render(fmt.Sprintf("ahead:%d behind:%d", status.Ahead, status.Behind)),
	)

	line3 := fmt.Sprintf(
		"  %s %s",
		projectHUDKeyStyle.Render("root"),
		projectHUDValueStyle.Render(status.Root),
	)

	line4 := "  " + projectHUDKeyStyle.Render("working tree")
	line5 := fmt.Sprintf(
		"   %s  %s  %s",
		projectHUDValueStyle.Render(fmt.Sprintf("staged: %d", status.Staged)),
		projectHUDValueStyle.Render(fmt.Sprintf("modified: %d", status.Modified)),
		projectHUDValueStyle.Render(fmt.Sprintf("untracked: %d", status.Untracked)),
	)

	line6 := "  " + projectHUDKeyStyle.Render("summary")
	line7 := "   " + projectHUDValueStyle.Render(summary)

	return strings.Join([]string{line1, line2, line3, "", line4, line5, "", line6, line7}, "\n")
}
