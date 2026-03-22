package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/internal/update"
	"github.com/vigo999/ms-cli/internal/version"
)

// updateChoice is the result of the update prompt.
type updateChoice int

const (
	updateChoiceUpdate updateChoice = iota
	updateChoiceSkip
)

// updatePrompt is a mini Bubble Tea model for the pre-TUI update screen.
type updatePrompt struct {
	result   *update.CheckResult
	cursor   int
	options  []string
	chosen   bool
	choice   updateChoice
	message  string // status message after selection
	quitting bool
}

func newUpdatePrompt(result *update.CheckResult) updatePrompt {
	options := []string{"Update now", "Skip this time"}
	if result.ForceUpdate {
		options = []string{"Update now"}
	}
	return updatePrompt{
		result:  result,
		options: options,
	}
}

func (m updatePrompt) Init() tea.Cmd { return nil }

func (m updatePrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.result.ForceUpdate {
				// Can't quit on forced update, must update
				return m, nil
			}
			m.choice = updateChoiceSkip
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.chosen = true
			if m.cursor == 0 {
				m.choice = updateChoiceUpdate
				m.message = "  Downloading..."
				return m, doUpdate(m.result)
			}
			// Skip (only reachable for non-forced)
			m.choice = updateChoiceSkip
			m.quitting = true
			return m, tea.Quit
		}

	case updateDoneMsg:
		m.quitting = true
		if msg.err != nil {
			m.message = fmt.Sprintf("  %v\n  Continuing with current version...", msg.err)
			m.choice = updateChoiceSkip
		} else {
			m.message = fmt.Sprintf("  Updated to %s. Please restart ms-cli.", m.result.LatestVersion)
			m.choice = updateChoiceUpdate
		}
		return m, tea.Quit
	}

	return m, nil
}

func (m updatePrompt) View() string {
	var b strings.Builder

	b.WriteString("\n")
	if m.result.ForceUpdate {
		b.WriteString(fmt.Sprintf("  Required update: %s → %s\n", m.result.CurrentVersion, m.result.LatestVersion))
	} else {
		b.WriteString(fmt.Sprintf("  Update available: %s → %s\n", m.result.CurrentVersion, m.result.LatestVersion))
	}
	b.WriteString("\n")

	if m.chosen {
		b.WriteString(m.message)
		b.WriteString("\n")
		return b.String()
	}

	for i, opt := range m.options {
		if i == m.cursor {
			b.WriteString(fmt.Sprintf("  > %s\n", opt))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n", opt))
		}
	}
	b.WriteString("\n  Use ↑/↓ to select, Enter to confirm\n")

	return b.String()
}

// updateDoneMsg is sent when download+install completes.
type updateDoneMsg struct{ err error }

func doUpdate(result *update.CheckResult) tea.Cmd {
	return func() tea.Msg {
		tmpPath, err := update.Download(context.Background(), result.DownloadURL)
		if err != nil {
			return updateDoneMsg{fmt.Errorf("download failed: %w", err)}
		}
		defer os.Remove(tmpPath)

		if err := update.Install(tmpPath); err != nil {
			return updateDoneMsg{fmt.Errorf("install failed: %w", err)}
		}
		return updateDoneMsg{}
	}
}

// checkAndPromptUpdate checks for updates before the TUI launches.
// Returns true if the caller should exit (user updated successfully).
func checkAndPromptUpdate() bool {
	if version.Version == "dev" || version.Version == "" {
		return false
	}

	result, err := update.Check(context.Background(), version.Version)
	if err != nil || result == nil || !result.UpdateAvailable {
		return false
	}

	prompt := newUpdatePrompt(result)
	p := tea.NewProgram(prompt)
	finalModel, err := p.Run()
	if err != nil {
		return false
	}

	final := finalModel.(updatePrompt)
	fmt.Println()
	return final.choice == updateChoiceUpdate && final.quitting
}

// cleanUpdateTmp removes leftover temp files from previous update attempts.
func cleanUpdateTmp() {
	tmpDir := update.ConfigDir() + "/tmp"
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "ms-cli-update-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > 24*time.Hour {
			os.Remove(tmpDir + "/" + e.Name())
		}
	}
}
