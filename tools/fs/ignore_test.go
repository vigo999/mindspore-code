package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobToolIgnoresGitPaths(t *testing.T) {
	workDir := t.TempDir()
	mustWriteTestFile(t, filepath.Join(workDir, "visible.txt"), "visible")
	mustWriteTestFile(t, filepath.Join(workDir, "nested", "found.go"), "package nested")
	mustWriteTestFile(t, filepath.Join(workDir, ".git", "config"), "hidden")

	params, err := json.Marshal(map[string]any{
		"pattern": "**",
		"path":    ".",
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	result, err := NewGlobTool(workDir).Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Execute result error = %v, want nil", result.Error)
	}
	if strings.Contains(result.Content, ".git") {
		t.Fatalf("glob content = %q, should not include .git paths", result.Content)
	}
	if !strings.Contains(result.Content, "visible.txt") {
		t.Fatalf("glob content = %q, want visible.txt", result.Content)
	}
	if !strings.Contains(result.Content, filepath.Join("nested", "found.go")) {
		t.Fatalf("glob content = %q, want nested/found.go", result.Content)
	}
}

func TestGrepToolIgnoresGitPaths(t *testing.T) {
	workDir := t.TempDir()
	mustWriteTestFile(t, filepath.Join(workDir, "visible.txt"), "needle\n")
	mustWriteTestFile(t, filepath.Join(workDir, ".git", "config"), "needle\n")

	params, err := json.Marshal(map[string]any{
		"pattern":        "needle",
		"path":           ".",
		"case_sensitive": true,
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	result, err := NewGrepTool(workDir).Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Execute result error = %v, want nil", result.Error)
	}
	if strings.Contains(result.Content, ".git") {
		t.Fatalf("grep content = %q, should not include .git paths", result.Content)
	}
	if !strings.Contains(result.Content, "visible.txt:1:needle") {
		t.Fatalf("grep content = %q, want visible.txt match", result.Content)
	}
}

func mustWriteTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
