package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	t.Run("with valid frontmatter", func(t *testing.T) {
		content := "---\nname: pdf\ndescription: Process PDF files\n---\n\n# Body content\n"
		meta, body, err := parseFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.Name != "pdf" {
			t.Errorf("expected name %q, got %q", "pdf", meta.Name)
		}
		if meta.Description != "Process PDF files" {
			t.Errorf("expected description %q, got %q", "Process PDF files", meta.Description)
		}
		if body != "# Body content" {
			t.Errorf("unexpected body: %q", body)
		}
	})

	t.Run("without frontmatter", func(t *testing.T) {
		content := "# Just a body\nNo frontmatter here."
		meta, body, err := parseFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.Name != "" {
			t.Errorf("expected empty name, got %q", meta.Name)
		}
		if body == "" {
			t.Error("expected non-empty body")
		}
	})

	t.Run("with BOM prefix", func(t *testing.T) {
		content := "\ufeff---\nname: bom\n---\n\nbody"
		meta, _, err := parseFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.Name != "bom" {
			t.Errorf("expected name %q, got %q", "bom", meta.Name)
		}
	})
}

func TestScanDir(t *testing.T) {
	dir := t.TempDir()

	// Create a skill directory with SKILL.md
	skillDir := filepath.Join(dir, "mypdf")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: mypdf\ndescription: My PDF skill\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a directory without SKILL.md (should be skipped)
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	summaries := scanDir(dir)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Name != "mypdf" {
		t.Errorf("expected name %q, got %q", "mypdf", summaries[0].Name)
	}
	if summaries[0].Description != "My PDF skill" {
		t.Errorf("expected description %q, got %q", "My PDF skill", summaries[0].Description)
	}
}

func TestLoaderPriority(t *testing.T) {
	low := t.TempDir()
	high := t.TempDir()

	for _, tc := range []struct {
		dir  string
		desc string
	}{
		{low, "low priority description"},
		{high, "high priority description"},
	} {
		skillDir := filepath.Join(tc.dir, "pdf")
		_ = os.MkdirAll(skillDir, 0o755)
		content := "---\nname: pdf\ndescription: " + tc.desc + "\n---\n\nbody"
		_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)
	}

	loader := NewLoader(low, high)
	summaries := loader.List()
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary (deduplicated), got %d", len(summaries))
	}
	if summaries[0].Description != "high priority description" {
		t.Errorf("expected high priority to win, got %q", summaries[0].Description)
	}
}

func TestLoaderLoad(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "pdf")
	_ = os.MkdirAll(skillDir, 0o755)
	skillContent := "---\nname: pdf\ndescription: Process PDFs\n---\n\n# PDF instructions\nDo things."
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644)

	loader := NewLoader(dir)
	content, err := loader.Load("pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty content")
	}
	// Should be wrapped
	if content[:len("<skill")] != "<skill" {
		t.Errorf("expected content to start with <skill, got: %s", content[:20])
	}
	// Should not contain frontmatter
	if contains(content, "description: Process PDFs") {
		t.Error("expected frontmatter to be stripped")
	}
	// Should contain body
	if !contains(content, "# PDF instructions") {
		t.Error("expected body to be present")
	}
}

func TestLoaderLoadNotFound(t *testing.T) {
	loader := NewLoader(t.TempDir())
	_, err := loader.Load("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
