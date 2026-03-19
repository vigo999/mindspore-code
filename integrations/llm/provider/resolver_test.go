package provider

import (
	"testing"

	"github.com/vigo999/ms-cli/configs"
)

func TestResolveConfig_ProviderPriority(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_PROVIDER", "  ANTHROPIC  ")

	cfg := configs.ModelConfig{
		Provider: "openai",
		Key:      "cfg-key",
	}

	got, err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.Kind != ProviderAnthropic {
		t.Fatalf("ResolveConfig() kind = %q, want %q", got.Kind, ProviderAnthropic)
	}
}

func TestResolveConfig_OpenAIKeyPriority(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_API_KEY", "MsCli-Key")
	t.Setenv("OPENAI_API_KEY", "OpenAI-Key")

	cfg := configs.ModelConfig{
		Key: "cfg-key",
	}

	got, err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.APIKey != "MsCli-Key" {
		t.Fatalf("ResolveConfig() APIKey = %q, want %q", got.APIKey, "MsCli-Key")
	}
}

func TestResolveConfig_AnthropicKeyPriority(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "token-key")
	t.Setenv("ANTHROPIC_API_KEY", "api-key")

	cfg := configs.ModelConfig{
		Key: "cfg-key",
	}

	got, err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.APIKey != "token-key" {
		t.Fatalf("ResolveConfig() APIKey = %q, want %q", got.APIKey, "token-key")
	}
}

func TestResolveConfig_OpenAIBaseURLPriority(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_BASE_URL", "HTTPS://MsCli.Example/V1")
	t.Setenv("OPENAI_BASE_URL", "HTTPS://OpenAI.Example/V1")

	cfg := configs.ModelConfig{
		URL: "HTTPS://Cfg.Example/V1",
		Key: "cfg-key",
	}

	got, err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.BaseURL != "HTTPS://MsCli.Example/V1" {
		t.Fatalf("ResolveConfig() BaseURL = %q, want %q", got.BaseURL, "HTTPS://MsCli.Example/V1")
	}
}

func TestResolveConfig_AnthropicBaseURLPriority(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_PROVIDER", "anthropic")
	t.Setenv("MSCLI_BASE_URL", "HTTPS://MsCli.Example/V1")
	t.Setenv("ANTHROPIC_BASE_URL", "HTTPS://Anthropic.Example/V1")

	cfg := configs.ModelConfig{
		URL: "HTTPS://Cfg.Example/V1",
		Key: "cfg-key",
	}

	got, err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.BaseURL != "HTTPS://MsCli.Example/V1" {
		t.Fatalf("ResolveConfig() BaseURL = %q, want %q", got.BaseURL, "HTTPS://MsCli.Example/V1")
	}
}

func TestResolveConfig_AnthropicKeyPreservesCase(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "AnThRoPiC-ToKeN")

	got, err := ResolveConfig(configs.ModelConfig{})
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.APIKey != "AnThRoPiC-ToKeN" {
		t.Fatalf("ResolveConfig() APIKey = %q, want %q", got.APIKey, "AnThRoPiC-ToKeN")
	}
}

func TestResolveConfig_AnthropicBaseURLPreservesCase(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_BASE_URL", "HTTPS://Anthropic.Example/Path/V1")

	got, err := ResolveConfig(configs.ModelConfig{Key: "cfg-key"})
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.BaseURL != "HTTPS://Anthropic.Example/Path/V1" {
		t.Fatalf("ResolveConfig() BaseURL = %q, want %q", got.BaseURL, "HTTPS://Anthropic.Example/Path/V1")
	}
}

func TestResolveConfig_AnthropicHeaderConflict(t *testing.T) {
	clearResolverEnv(t)
	t.Setenv("MSCLI_PROVIDER", "anthropic")

	cfg := configs.ModelConfig{
		Key: "cfg-key",
		Headers: map[string]string{
			"X-API-KEY":         "user-key",
			"Anthropic-Version": "2024-01-01",
			"X-Trace-ID":        "trace-123",
		},
	}

	got, err := ResolveConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if got.AuthHeaderName != "x-api-key" {
		t.Fatalf("ResolveConfig() AuthHeaderName = %q, want %q", got.AuthHeaderName, "x-api-key")
	}

	if got.Headers["x-api-key"] != "cfg-key" {
		t.Fatalf("ResolveConfig() x-api-key = %q, want %q", got.Headers["x-api-key"], "cfg-key")
	}

	if got.Headers["anthropic-version"] != "2023-06-01" {
		t.Fatalf("ResolveConfig() anthropic-version = %q, want %q", got.Headers["anthropic-version"], "2023-06-01")
	}

	if got.Headers["X-Trace-ID"] != "trace-123" {
		t.Fatalf("ResolveConfig() X-Trace-ID = %q, want %q", got.Headers["X-Trace-ID"], "trace-123")
	}
}

func clearResolverEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"MSCLI_PROVIDER",
		"MSCLI_API_KEY",
		"OPENAI_API_KEY",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_API_KEY",
		"MSCLI_BASE_URL",
		"OPENAI_BASE_URL",
		"ANTHROPIC_BASE_URL",
	} {
		t.Setenv(key, "")
	}
}
