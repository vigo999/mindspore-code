package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestCmdProjectStreamsFormattedSnapshot(t *testing.T) {
	orig := runProjectGit
	defer func() { runProjectGit = orig }()

	root := t.TempDir()
	err := os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	if err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	err = os.WriteFile(filepath.Join(root, "docs", "project.yaml"), []byte(`top_msg: this is custom project status
top_msg_color: white

overview:
  color: dark_green
  phase: refactor / dogfood
  owner: travis
  focus: status command
progress_pct: 78
today_tasks:
  color: dark_green
  date: 2026-03-19
  items:
    - title: project status command resumed
      color: dark_green
      status: done
      progress: 100
      progress_color: green
      empty_color: gray
      owner: travis
      due-date: 2026-03-21
    - title: status schema draft
      color: dark_green
      status: doing
      progress: 60
      progress_color: yellow
      empty_color: gray
      owner: verylongowner
      due-date: 2026-03-22
    - title: collector wiring blocked on schema
      color: dark_green
      status: block
      progress: 25
      progress_color: red
      empty_color: gray
      owner: alice
      due-date: 2026-03-23
tomorrow:
  color: dark_green
  items:
    - title: define schema
      color: dark_green
      progress: 0
      progress_color: cyan
      empty_color: gray
      owner: travis
    - title: implement collector
      color: dark_green
      progress: 20
      progress_color: cyan
      empty_color: gray
      owner: travis
    - title: add renderer
      color: dark_green
      progress: 40
      progress_color: cyan
      empty_color: gray
      owner: travis
milestone:
  color: dark_green
  items:
    - title: stream card v1
      color: dark_green
      progress: 78
      progress_color: magenta
      empty_color: gray
      owner: travis
`), 0o644)
	if err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	err = os.WriteFile(filepath.Join(root, "roadmap.yaml"), []byte(`version: 1
target_date: "2026-06-30"
phases:
  - id: "phase1"
    name: "Foundation"
    start: "2026-03-01"
    end: "2026-03-31"
    milestones:
      - id: "p1-arch"
        title: "Architecture analysis and development plan"
        status: "done"
      - id: "p1-llm"
        title: "LLM Provider architecture"
        status: "in_progress"
      - id: "p1-config"
        title: "Configuration management system"
        status: "pending"
`), 0o644)
	if err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	runProjectGit = func(workDir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return root, nil
		case "symbolic-ref --short HEAD":
			return "refactor-arch-3", nil
		case "status --short":
			return " M ui/app.go\nA  ui/model/project_test.go\n?? docs/project.yaml", nil
		case "rev-list --left-right --count @{upstream}...HEAD":
			return "2 5", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
			return "", nil
		}
	}

	app := &Application{
		WorkDir: root,
		EventCh: make(chan model.Event, 8),
	}

	app.cmdProject(nil)

	ev := drainUntilEventType(t, app, model.AgentReply)
	for _, want := range []string{
		applyProjectStyle("this is custom project status", "white", true),
		applyProjectStyle("[ OVERVIEW ]", "white", true),
		applyProjectStyle("[ TODAY TASKS ]", "white", true),
		"[2026-03-19]",
		"phase: refactor / dogfood",
		"owner: travis",
		"focus: status command",
		applyProjectStyle("[ PROGRESS PCT ]", "white", true),
		"  78",
		applyProjectStyle("[ TOMORROW ]", "white", true),
		applyProjectStyle("[ 🚀 MILESTONE ]", "white", true),
		"project status command resumed",
		applyProjectStyle("✓", "green", false),
		"status schema draft",
		"collector wiring blocked on schema",
		"due-date: 2026-03-21",
		"due-date: 2026-03-22",
		"due-date: 2026-03-23",
		"define schema",
		"implement collector",
		applyProjectStyle("stream card v1", "magenta", false),
		applyProjectStyle("100%", "green", false) + "  owner: travis",
		applyProjectStyle(" 60%", "green", false) + "  owner: verylongowner",
		applyProjectStyle(" 25%", "green", false) + "  owner: alice",
		applyProjectStyle("  0%", "cyan", false) + "  owner: travis",
		applyProjectStyle(" 20%", "cyan", false) + "  owner: travis",
		applyProjectStyle(" 78%", "magenta", false) + "  owner: travis",
	} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("expected project snapshot to contain %q, got:\n%s", want, ev.Message)
		}
	}
	if !strings.Contains(ev.Message, "\x1b[") {
		t.Fatalf("expected styled project output, got:\n%s", ev.Message)
	}
	if !strings.Contains(ev.Message, applyProjectStyle("this is custom project status", "white", true)) {
		t.Fatalf("expected colored top message, got:\n%s", ev.Message)
	}
	for _, want := range []string{
		applyProjectStyle("[ OVERVIEW ]", "white", true),
		"\x1b[38;5;34m■\x1b[0m",
		applyProjectStyle("✓", "green", false),
		"▶",
		applyProjectStyle("100%", "green", false),
		applyProjectStyle(" 60%", "green", false),
		applyProjectStyle(" 78%", "magenta", false),
		"\x1b[38;5;201m■\x1b[0m",
		"\x1b[38;5;244m□\x1b[0m",
	} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("expected progress bar color fragment %q, got:\n%s", want, ev.Message)
		}
	}
	if strings.Contains(ev.Message, "week goals") {
		t.Fatalf("expected removed yaml section to stay removed, got:\n%s", ev.Message)
	}
	if !strings.Contains(ev.Message, "╭") || !strings.Contains(ev.Message, "│") || !strings.Contains(ev.Message, "╰") {
		t.Fatalf("expected boxed project snapshot, got:\n%s", ev.Message)
	}
	var dueIndices []int
	for _, line := range strings.Split(ev.Message, "\n") {
		if strings.Contains(line, "due-date: 2026-03-2") {
			idx := strings.Index(line, "due-date:")
			if idx < 0 {
				t.Fatalf("expected due-date line to contain due-date label, got %q", line)
			}
			dueIndices = append(dueIndices, lipgloss.Width(line[:idx]))
		}
	}
	if len(dueIndices) != 3 {
		t.Fatalf("expected three due-date task rows, got %d in:\n%s", len(dueIndices), ev.Message)
	}
	for i := 1; i < len(dueIndices); i++ {
		if dueIndices[i] != dueIndices[0] {
			t.Fatalf("expected due-date column alignment, got indices %v in:\n%s", dueIndices, ev.Message)
		}
	}
}

func TestCmdProjectCloseExplainsStreamMode(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}

	app.cmdProject([]string{"close"})

	ev := drainUntilEventType(t, app, model.AgentReply)
	if !strings.Contains(ev.Message, "stream-only") {
		t.Fatalf("expected stream-only explanation, got %q", ev.Message)
	}
}

func TestCmdProjectInvalidYAMLReturnsError(t *testing.T) {
	orig := runProjectGit
	defer func() { runProjectGit = orig }()

	root := t.TempDir()
	err := os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	if err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	err = os.WriteFile(filepath.Join(root, "docs", "project.yaml"), []byte(`top_msg: this is ms-cli project status
 color: blue
`), 0o644)
	if err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	runProjectGit = func(workDir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return root, nil
		case "symbolic-ref --short HEAD":
			return "refactor-arch-3", nil
		case "status --short":
			return "", nil
		case "rev-list --left-right --count @{upstream}...HEAD":
			return "0 0", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
			return "", nil
		}
	}

	app := &Application{
		WorkDir: root,
		EventCh: make(chan model.Event, 8),
	}

	app.cmdProject(nil)

	ev := drainUntilEventType(t, app, model.ToolError)
	if !strings.Contains(ev.Message, "parse") || !strings.Contains(ev.Message, "docs/project.yaml") {
		t.Fatalf("expected parse error mentioning docs/project.yaml, got %q", ev.Message)
	}
}

func TestHandleCommandProjectAddWritesYAMLAndRendersSnapshot(t *testing.T) {
	orig := runProjectGit
	defer func() { runProjectGit = orig }()

	root := t.TempDir()
	err := os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	if err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	err = os.WriteFile(filepath.Join(root, "docs", "project.yaml"), []byte(`top_msg: this is project status
tasks:
  items:
    - id: existing
      title: existing task
      progress: 20
      owner: alice
`), 0o644)
	if err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	runProjectGit = func(workDir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return root, nil
		case "symbolic-ref --short HEAD":
			return "refactor-arch-4", nil
		case "status --short":
			return " M docs/project.yaml", nil
		case "rev-list --left-right --count @{upstream}...HEAD":
			return "0 0", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
			return "", nil
		}
	}

	app := &Application{
		WorkDir: root,
		EventCh: make(chan model.Event, 8),
	}

	app.handleCommand(`/project add today "new task title" --id new-task --owner bob --progress 30 --due-date 2026-03-21`)

	ev := drainUntilEventType(t, app, model.AgentReply)
	for _, want := range []string{"new task title", applyProjectStyle(" 30%", "green", false) + "  owner: bob"} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("expected rendered snapshot to contain %q, got:\n%s", want, ev.Message)
		}
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "project.yaml"))
	if err != nil {
		t.Fatalf("read project.yaml: %v", err)
	}
	for _, want := range []string{"id: new-task", "title: new task title", "progress: \"30\"", "owner: bob", "due-date: \"2026-03-21\""} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("expected project.yaml to contain %q, got:\n%s", want, string(data))
		}
	}
}

func TestHandleCommandProjectUpdateWritesYAMLAndRendersSnapshot(t *testing.T) {
	orig := runProjectGit
	defer func() { runProjectGit = orig }()

	root := t.TempDir()
	err := os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	if err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	err = os.WriteFile(filepath.Join(root, "docs", "project.yaml"), []byte(`top_msg: this is project status
tasks:
  items:
    - id: existing
      title: existing task
      progress: 20
      owner: alice
`), 0o644)
	if err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	runProjectGit = func(workDir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return root, nil
		case "symbolic-ref --short HEAD":
			return "refactor-arch-4", nil
		case "status --short":
			return " M docs/project.yaml", nil
		case "rev-list --left-right --count @{upstream}...HEAD":
			return "0 0", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
			return "", nil
		}
	}

	app := &Application{
		WorkDir: root,
		EventCh: make(chan model.Event, 8),
	}

	app.handleCommand(`/project update today existing --title "updated task" --owner carol --progress 80 --due-date 2026-03-22`)

	ev := drainUntilEventType(t, app, model.AgentReply)
	for _, want := range []string{"updated task", applyProjectStyle(" 80%", "green", false) + "  owner: carol"} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("expected rendered snapshot to contain %q, got:\n%s", want, ev.Message)
		}
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "project.yaml"))
	if err != nil {
		t.Fatalf("read project.yaml: %v", err)
	}
	for _, want := range []string{"title: updated task", "progress: \"80\"", "owner: carol", "due-date: \"2026-03-22\""} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("expected project.yaml to contain %q, got:\n%s", want, string(data))
		}
	}
}

func TestHandleCommandProjectRemoveWritesYAMLAndRendersSnapshot(t *testing.T) {
	orig := runProjectGit
	defer func() { runProjectGit = orig }()

	root := t.TempDir()
	err := os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	if err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	err = os.WriteFile(filepath.Join(root, "docs", "project.yaml"), []byte(`top_msg: this is project status
tasks:
  items:
    - id: existing
      title: existing task
      progress: 20
      owner: alice
    - id: keep
      title: keep task
      progress: 40
      owner: bob
`), 0o644)
	if err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	runProjectGit = func(workDir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return root, nil
		case "symbolic-ref --short HEAD":
			return "refactor-arch-4", nil
		case "status --short":
			return " M docs/project.yaml", nil
		case "rev-list --left-right --count @{upstream}...HEAD":
			return "0 0", nil
		default:
			t.Fatalf("unexpected git args: %v", args)
			return "", nil
		}
	}

	app := &Application{
		WorkDir: root,
		EventCh: make(chan model.Event, 8),
	}

	app.handleCommand(`/project rm today existing`)

	ev := drainUntilEventType(t, app, model.AgentReply)
	if strings.Contains(ev.Message, "existing task") {
		t.Fatalf("expected removed task to disappear from snapshot, got:\n%s", ev.Message)
	}
	if !strings.Contains(ev.Message, "keep task") {
		t.Fatalf("expected remaining task to stay in snapshot, got:\n%s", ev.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "project.yaml"))
	if err != nil {
		t.Fatalf("read project.yaml: %v", err)
	}
	if strings.Contains(string(data), "existing task") {
		t.Fatalf("expected removed task to disappear from project.yaml, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "keep task") {
		t.Fatalf("expected remaining task to stay in project.yaml, got:\n%s", string(data))
	}
}
