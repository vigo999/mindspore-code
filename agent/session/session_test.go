package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestBackupSnapshotBeforeCompactWritesBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := filepath.Join(home, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	s, err := Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer s.Close()

	before := []llm.Message{
		llm.NewUserMessage("hello"),
		llm.NewAssistantMessage("world"),
	}
	if err := s.BackupSnapshotBeforeCompact("system prompt", before); err != nil {
		t.Fatalf("BackupSnapshotBeforeCompact() error = %v", err)
	}
	if err := s.SaveSnapshot("system prompt", []llm.Message{llm.NewAssistantMessage("summary")}); err != nil {
		t.Fatalf("SaveSnapshot() error = %v", err)
	}

	sessionDir := filepath.Dir(s.Path())
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	var backupPath string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), snapshotBackupPrefix) {
			backupPath = filepath.Join(sessionDir, entry.Name())
			break
		}
	}
	if backupPath == "" {
		t.Fatal("expected compact snapshot backup to be created")
	}

	current, err := loadSnapshot(filepath.Join(sessionDir, snapshotFilename))
	if err != nil {
		t.Fatalf("loadSnapshot(current) error = %v", err)
	}
	if got := len(current.Messages); got != 1 {
		t.Fatalf("current snapshot message count = %d, want 1", got)
	}
	if got := current.Messages[0].Content; got != "summary" {
		t.Fatalf("current snapshot content = %q, want %q", got, "summary")
	}

	backup, err := loadSnapshot(backupPath)
	if err != nil {
		t.Fatalf("loadSnapshot(backup) error = %v", err)
	}
	if got := len(backup.Messages); got != 2 {
		t.Fatalf("backup snapshot message count = %d, want 2", got)
	}
	if got := backup.Messages[0].Content; got != "hello" {
		t.Fatalf("backup first message = %q, want %q", got, "hello")
	}
	if got := backup.Messages[1].Content; got != "world" {
		t.Fatalf("backup second message = %q, want %q", got, "world")
	}
}

func TestReplayEventsUsesFinalAssistantInsteadOfDeltaFragments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := filepath.Join(home, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	s, err := Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sessionID := s.ID()

	if err := s.AppendUserInput("hi"); err != nil {
		t.Fatalf("AppendUserInput() error = %v", err)
	}
	if err := s.AppendAssistantDelta("hel"); err != nil {
		t.Fatalf("AppendAssistantDelta() error = %v", err)
	}
	if err := s.AppendAssistantDelta("lo"); err != nil {
		t.Fatalf("AppendAssistantDelta() error = %v", err)
	}
	if err := s.AppendAssistant("hello"); err != nil {
		t.Fatalf("AppendAssistant() error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	loaded, err := LoadByID(workDir, sessionID)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	defer loaded.Close()

	events := loaded.ReplayEvents()
	if got := len(events); got != 2 {
		t.Fatalf("ReplayEvents() count = %d, want 2", got)
	}
	if got := events[0].Message; got != "hi" {
		t.Fatalf("first replay message = %q, want %q", got, "hi")
	}
	if got := events[1].Message; got != "hello" {
		t.Fatalf("second replay message = %q, want %q", got, "hello")
	}
}

func TestReplayEventsFlushesAssistantDeltaWithoutFinalRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := filepath.Join(home, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	s, err := Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sessionID := s.ID()

	if err := s.AppendAssistantDelta("par"); err != nil {
		t.Fatalf("AppendAssistantDelta() error = %v", err)
	}
	if err := s.AppendAssistantDelta("tial"); err != nil {
		t.Fatalf("AppendAssistantDelta() error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	loaded, err := LoadByID(workDir, sessionID)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	defer loaded.Close()

	events := loaded.ReplayEvents()
	if got := len(events); got != 1 {
		t.Fatalf("ReplayEvents() count = %d, want 1", got)
	}
	if got := events[0].Message; got != "partial" {
		t.Fatalf("replayed delta content = %q, want %q", got, "partial")
	}
}
