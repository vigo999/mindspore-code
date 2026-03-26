package fs

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteToolAcceptsFilePathAlias(t *testing.T) {
	workDir := t.TempDir()
	tool := NewWriteTool(workDir)

	args, err := json.Marshal(map[string]string{
		"file_path": "alias-file-path.txt",
		"content":   "hello",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result, execErr := tool.Execute(context.Background(), args)
	if execErr != nil {
		t.Fatalf("Execute() error = %v", execErr)
	}
	if result.Error != nil {
		t.Fatalf("result.Error = %v", result.Error)
	}
	if !strings.Contains(result.Content, "alias-file-path.txt") {
		t.Fatalf("result.Content = %q, want alias path", result.Content)
	}
}

func TestWriteToolAcceptsFilenameAlias(t *testing.T) {
	workDir := t.TempDir()
	tool := NewWriteTool(workDir)

	args, err := json.Marshal(map[string]string{
		"filename": "alias-filename.txt",
		"content":  "hello",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result, execErr := tool.Execute(context.Background(), args)
	if execErr != nil {
		t.Fatalf("Execute() error = %v", execErr)
	}
	if result.Error != nil {
		t.Fatalf("result.Error = %v", result.Error)
	}
	if !strings.Contains(result.Content, "alias-filename.txt") {
		t.Fatalf("result.Content = %q, want alias path", result.Content)
	}
}

func TestWriteToolMissingPathReturnsGuidanceError(t *testing.T) {
	workDir := t.TempDir()
	tool := NewWriteTool(workDir)

	args, err := json.Marshal(map[string]string{
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result, execErr := tool.Execute(context.Background(), args)
	if execErr != nil {
		t.Fatalf("Execute() error = %v", execErr)
	}
	if result.Error == nil {
		t.Fatal("result.Error = nil, want guidance error")
	}
	if got := result.Error.Error(); !strings.Contains(got, `use "path"`) {
		t.Fatalf("result.Error = %q, want path guidance", got)
	}
}

func TestWriteToolSchemaAndDescriptionGuidance(t *testing.T) {
	tool := NewWriteTool(".")

	if got := tool.Description(); !strings.Contains(got, "required fields path and content") {
		t.Fatalf("Description() = %q, want path/content guidance", got)
	}

	schema := tool.Schema()
	pathDesc := schema.Properties["path"].Description
	if !strings.Contains(pathDesc, "do not use file_path or filename") {
		t.Fatalf("path description = %q, want alias guidance", pathDesc)
	}
}
