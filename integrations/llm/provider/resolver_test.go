package provider

import (
	"errors"
	"testing"

	"github.com/vigo999/ms-cli/configs"
)

func TestResolveConfig_ProviderFromConfig(t *testing.T) {
	got, err := ResolveConfig(configs.ModelConfig{Provider: "anthropic", Key: "cfg-key"})
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}
	if got.Kind != ProviderAnthropic {
		t.Fatalf("ResolveConfig() kind = %q, want %q", got.Kind, ProviderAnthropic)
	}
}

func TestResolveConfig_DefaultProvider(t *testing.T) {
	got, err := ResolveConfig(configs.ModelConfig{Key: "cfg-key"})
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}
	if got.Kind != ProviderOpenAICompatible {
		t.Fatalf("ResolveConfig() kind = %q, want %q", got.Kind, ProviderOpenAICompatible)
	}
}

func TestResolveConfig_KeyFromConfig(t *testing.T) {
	got, err := ResolveConfig(configs.ModelConfig{Key: "Cfg-Key"})
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}
	if got.APIKey != "Cfg-Key" {
		t.Fatalf("ResolveConfig() APIKey = %q, want %q", got.APIKey, "Cfg-Key")
	}
}

func TestResolveConfig_MissingKey(t *testing.T) {
	_, err := ResolveConfig(configs.ModelConfig{})
	if err == nil {
		t.Fatal("ResolveConfig() error = nil, want missing key error")
	}
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("ResolveConfig() error = %v, want ErrMissingAPIKey", err)
	}
}

func TestResolveConfig_BaseURLFromConfig(t *testing.T) {
	got, err := ResolveConfig(configs.ModelConfig{
		URL: "https://cfg.example/v1",
		Key: "cfg-key",
	})
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}
	if got.BaseURL != "https://cfg.example/v1" {
		t.Fatalf("ResolveConfig() BaseURL = %q, want %q", got.BaseURL, "https://cfg.example/v1")
	}
}

func TestResolveConfig_DefaultBaseURLs(t *testing.T) {
	t.Run("openai-compatible default", func(t *testing.T) {
		got, err := ResolveConfig(configs.ModelConfig{Key: "cfg-key"})
		if err != nil {
			t.Fatalf("ResolveConfig() error = %v", err)
		}
		if got.BaseURL != defaultOpenAIBaseURL {
			t.Fatalf("ResolveConfig() BaseURL = %q, want %q", got.BaseURL, defaultOpenAIBaseURL)
		}
	})

	t.Run("anthropic default", func(t *testing.T) {
		got, err := ResolveConfig(configs.ModelConfig{Provider: "anthropic", Key: "cfg-key"})
		if err != nil {
			t.Fatalf("ResolveConfig() error = %v", err)
		}
		if got.BaseURL != defaultAnthropicBaseURL {
			t.Fatalf("ResolveConfig() BaseURL = %q, want %q", got.BaseURL, defaultAnthropicBaseURL)
		}
	})
}

func TestResolveConfig_AnthropicUsesDefaultURLWhenGivenInheritedOpenAIDefault(t *testing.T) {
	cfg := configs.DefaultConfig()
	cfg.Model.Provider = "anthropic"
	cfg.Model.URL = "HTTPS://API.OPENAI.COM/v1/"
	cfg.Model.Key = "cfg-key"

	got, err := ResolveConfig(cfg.Model)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}
	if got.BaseURL != defaultAnthropicBaseURL {
		t.Fatalf("ResolveConfig() BaseURL = %q, want %q", got.BaseURL, defaultAnthropicBaseURL)
	}
}

func TestResolveConfig_UnsupportedProvider(t *testing.T) {
	_, err := ResolveConfig(configs.ModelConfig{Provider: "unsupported", Key: "cfg-key"})
	if err == nil {
		t.Fatal("ResolveConfig() error = nil, want unsupported provider error")
	}
}
