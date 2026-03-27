package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestCreateDefersDiskWritesUntilActivate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workDir := t.TempDir()
	s, err := Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Fatalf("expected no trajectory before activate, got err=%v", err)
	}
	if _, err := os.Stat(snapshotPath(s.Path())); !os.IsNotExist(err) {
		t.Fatalf("expected no snapshot before activate, got err=%v", err)
	}

	if err := s.AppendSkillActivation("demo-skill"); err != nil {
		t.Fatalf("append skill activation: %v", err)
	}
	if err := s.AppendUserInput("hello"); err != nil {
		t.Fatalf("append user input: %v", err)
	}
	if err := s.AppendAssistant("hi"); err != nil {
		t.Fatalf("append assistant reply: %v", err)
	}
	if err := s.SaveSnapshot("updated prompt", []llm.Message{
		llm.NewUserMessage("hello"),
		llm.NewAssistantMessage("hi"),
	}); err != nil {
		t.Fatalf("save buffered snapshot: %v", err)
	}

	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Fatalf("expected no trajectory before activate after buffering, got err=%v", err)
	}
	if _, err := os.Stat(snapshotPath(s.Path())); !os.IsNotExist(err) {
		t.Fatalf("expected no snapshot before activate after buffering, got err=%v", err)
	}

	if err := s.Activate(); err != nil {
		t.Fatalf("activate session: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close activated session: %v", err)
	}

	if _, err := os.Stat(s.Path()); err != nil {
		t.Fatalf("expected trajectory after activate: %v", err)
	}
	if _, err := os.Stat(snapshotPath(s.Path())); err != nil {
		t.Fatalf("expected snapshot after activate: %v", err)
	}

	loaded, err := LoadByID(workDir, s.ID())
	if err != nil {
		t.Fatalf("load activated session: %v", err)
	}
	t.Cleanup(func() {
		_ = loaded.Close()
	})

	if !loaded.HasPersistedDialogue() {
		t.Fatal("expected persisted dialogue after activation")
	}
	if got := loaded.Meta().SystemPrompt; got != "updated prompt" {
		t.Fatalf("meta system prompt = %q, want %q", got, "updated prompt")
	}

	systemPrompt, restored := loaded.RestoreContext()
	if systemPrompt != "updated prompt" {
		t.Fatalf("restored system prompt = %q, want %q", systemPrompt, "updated prompt")
	}
	if len(restored) != 2 {
		t.Fatalf("restored message count = %d, want 2", len(restored))
	}

	replay := loaded.ReplayEvents()
	if len(replay) != 3 {
		t.Fatalf("replay event count = %d, want 3", len(replay))
	}
}

func TestWorkDirKeySanitizesWindowsInvalidFilenameChars(t *testing.T) {
	key := workDirKey(`C:\Users\alice\work\ms-cli`)

	for _, invalid := range []string{`\\`, ":", "*", "?", `"`, "<", ">", "|", "/"} {
		if strings.Contains(key, invalid) {
			t.Fatalf("workDirKey(%q) = %q, contains invalid filename char %q", `C:\Users\alice\work\ms-cli`, key, invalid)
		}
	}
	if strings.Trim(key, ".- ") == "" {
		t.Fatalf("workDirKey(%q) = %q, want non-empty safe key", `C:\Users\alice\work\ms-cli`, key)
	}
}

func TestReplayTimelinePreservesRecordTimestamps(t *testing.T) {
	t0 := time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(150 * time.Millisecond)
	t2 := t1.Add(250 * time.Millisecond)

	s := &Session{
		records: []MessageRecord{
			{Type: recordTypeUser, Timestamp: t0, Content: "hello"},
			{Type: recordTypeToolCall, Timestamp: t1, ToolName: "shell", Arguments: []byte(`{"command":"pwd"}`)},
			{Type: recordTypeAssistant, Timestamp: t2, Content: "done"},
		},
	}

	timeline := s.ReplayTimeline()
	if len(timeline) != 3 {
		t.Fatalf("timeline length = %d, want 3", len(timeline))
	}
	if !timeline[0].Timestamp.Equal(t0) {
		t.Fatalf("first timestamp = %v, want %v", timeline[0].Timestamp, t0)
	}
	if timeline[0].Event.Type != "UserInput" {
		t.Fatalf("first event type = %q, want %q", timeline[0].Event.Type, "UserInput")
	}
	if !timeline[1].Timestamp.Equal(t1) {
		t.Fatalf("second timestamp = %v, want %v", timeline[1].Timestamp, t1)
	}
	if timeline[1].Event.Type != "ToolCallStart" {
		t.Fatalf("second event type = %q, want %q", timeline[1].Event.Type, "ToolCallStart")
	}
	if !timeline[2].Timestamp.Equal(t2) {
		t.Fatalf("third timestamp = %v, want %v", timeline[2].Timestamp, t2)
	}
	if timeline[2].Event.Type != "AgentReply" {
		t.Fatalf("third event type = %q, want %q", timeline[2].Event.Type, "AgentReply")
	}
}

func TestLoadReplayPathAcceptsTrajectoryJSONFilename(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workDir := t.TempDir()
	s, err := Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := s.AppendUserInput("hello"); err != nil {
		t.Fatalf("append user input: %v", err)
	}
	if err := s.AppendAssistant("hi"); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	if err := s.Activate(); err != nil {
		t.Fatalf("activate session: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close session: %v", err)
	}

	replayDir := t.TempDir()
	replayPath := filepath.Join(replayDir, "trajectory.json")
	data, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatalf("read trajectory: %v", err)
	}
	if err := os.WriteFile(replayPath, data, 0600); err != nil {
		t.Fatalf("write replay trajectory: %v", err)
	}

	loaded, err := LoadReplayPath(replayPath)
	if err != nil {
		t.Fatalf("load replay path: %v", err)
	}
	t.Cleanup(func() {
		_ = loaded.Close()
	})

	if got := filepath.Base(loaded.Path()); got != "trajectory.json" {
		t.Fatalf("loaded path base = %q, want %q", got, "trajectory.json")
	}
	replay := loaded.ReplayEvents()
	if len(replay) != 2 {
		t.Fatalf("replay event count = %d, want 2", len(replay))
	}
	if got := replay[0].Type; got != "UserInput" {
		t.Fatalf("first event type = %q, want %q", got, "UserInput")
	}
	if got := replay[1].Type; got != "AgentReply" {
		t.Fatalf("second event type = %q, want %q", got, "AgentReply")
	}
}

func TestPlaybackTimelineInsertsThinkingBetweenUserAndLLMResponse(t *testing.T) {
	t0 := time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(2 * time.Second)
	t2 := t1.Add(3 * time.Second)
	t3 := t2.Add(1 * time.Second)

	s := &Session{
		records: []MessageRecord{
			{Type: recordTypeUser, Timestamp: t0, Content: "hello"},
			{Type: recordTypeToolCall, Timestamp: t1, ToolName: "shell", Arguments: []byte(`{"command":"pwd"}`)},
			{Type: recordTypeToolResult, Timestamp: t2, ToolName: "shell", Content: "/tmp"},
			{Type: recordTypeAssistant, Timestamp: t3, Content: "done"},
		},
	}

	playback := s.PlaybackTimeline()
	if len(playback) != 6 {
		t.Fatalf("playback timeline length = %d, want 6", len(playback))
	}
	if playback[0].Event.Type != "UserInput" {
		t.Fatalf("first event type = %q, want %q", playback[0].Event.Type, "UserInput")
	}
	if playback[1].Event.Type != "AgentThinking" {
		t.Fatalf("second event type = %q, want %q", playback[1].Event.Type, "AgentThinking")
	}
	if !playback[1].Timestamp.Equal(t0) {
		t.Fatalf("thinking timestamp after user = %v, want %v", playback[1].Timestamp, t0)
	}
	if playback[2].Event.Type != "ToolCallStart" {
		t.Fatalf("third event type = %q, want %q", playback[2].Event.Type, "ToolCallStart")
	}
	if playback[3].Event.Type != "ToolReplay" {
		t.Fatalf("fourth event type = %q, want %q", playback[3].Event.Type, "ToolReplay")
	}
	if playback[4].Event.Type != "AgentThinking" {
		t.Fatalf("fifth event type = %q, want %q", playback[4].Event.Type, "AgentThinking")
	}
	if !playback[4].Timestamp.Equal(t2) {
		t.Fatalf("thinking timestamp after tool result = %v, want %v", playback[4].Timestamp, t2)
	}
	if playback[5].Event.Type != "AgentReply" {
		t.Fatalf("sixth event type = %q, want %q", playback[5].Event.Type, "AgentReply")
	}
}
