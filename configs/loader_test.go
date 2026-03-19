package configs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigProvider(t *testing.T) {
	cfg := DefaultConfig()

	if got, want := cfg.Model.Provider, "openai-compatible"; got != want {
		t.Fatalf("default provider = %q, want %q", got, want)
	}
}

func TestApplyEnvOverridesProvider(t *testing.T) {
	t.Setenv("MSCLI_PROVIDER", "anthropic")

	cfg := DefaultConfig()
	cfg.Model.Provider = "yaml-provider"

	ApplyEnvOverrides(cfg)

	if got, want := cfg.Model.Provider, "anthropic"; got != want {
		t.Fatalf("provider after env override = %q, want %q", got, want)
	}
}

func TestLoadWithEnvProvider(t *testing.T) {
	t.Run("defaults when yaml provider blank", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "mscli.yaml")

		if err := os.WriteFile(path, []byte("model:\n  model: gpt-4o-mini\n  provider: \"\"\n"), 0600); err != nil {
			t.Fatalf("write yaml: %v", err)
		}

		cfg, err := LoadWithEnv(path)
		if err != nil {
			t.Fatalf("load config: %v", err)
		}

		if got, want := cfg.Model.Provider, "openai-compatible"; got != want {
			t.Fatalf("provider from blank yaml = %q, want %q", got, want)
		}
	})

	t.Run("env overrides yaml provider", func(t *testing.T) {
		t.Setenv("MSCLI_PROVIDER", "anthropic")

		dir := t.TempDir()
		path := filepath.Join(dir, "mscli.yaml")

		if err := os.WriteFile(path, []byte("model:\n  model: gpt-4o-mini\n  provider: yaml-provider\n"), 0600); err != nil {
			t.Fatalf("write yaml: %v", err)
		}

		cfg, err := LoadWithEnv(path)
		if err != nil {
			t.Fatalf("load config: %v", err)
		}

		if got, want := cfg.Model.Provider, "anthropic"; got != want {
			t.Fatalf("provider from env override = %q, want %q", got, want)
		}
	})
}
