package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
top_msg_color: magenta

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
    - title: status schema draft
      color: dark_green
      status: doing
      progress: 60
      progress_color: yellow
      empty_color: gray
      owner: travis
    - title: collector wiring blocked on schema
      color: dark_green
      status: block
      progress: 25
      progress_color: red
      empty_color: gray
      owner: alice
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
		"this is custom project status",
		"overview",
		"today tasks",
		"[2026-03-19]",
		"phase: refactor / dogfood",
		"owner: travis",
		"focus: status command",
		"progress pct",
		"  78",
		"tomorrow",
		"milestone",
		"project status command resumed",
		"[x] ",
		"status schema draft",
		"collector wiring blocked on schema",
		"define schema",
		"implement collector",
		"stream card v1",
		"100%  owner: travis",
		"  60%  owner: travis",
		"  25%  owner: alice",
		"   0%  owner: travis",
		"  20%  owner: travis",
		"  78%  owner: travis",
	} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("expected project snapshot to contain %q, got:\n%s", want, ev.Message)
		}
	}
	if !strings.Contains(ev.Message, "\x1b[") {
		t.Fatalf("expected styled project output, got:\n%s", ev.Message)
	}
	if !strings.Contains(ev.Message, "\x1b[38;5;201mthis is custom project status\x1b[0m") {
		t.Fatalf("expected colored top message, got:\n%s", ev.Message)
	}
	for _, want := range []string{
		"\x1b[38;5;34m■\x1b[0m",
		"\x1b[38;5;220m■\x1b[0m",
		"\x1b[38;5;196m■\x1b[0m",
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
