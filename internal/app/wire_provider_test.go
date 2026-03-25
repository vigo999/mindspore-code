package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestInitProviderAnthropic(t *testing.T) {
	provider, err := initProvider(configs.ModelConfig{
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet",
		Key:      "anthropic-token",
	}, providerResolveNoOverrides())
	if err != nil {
		t.Fatalf("initProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("initProvider() provider = nil, want provider")
	}
	if got, want := provider.Name(), "anthropic"; got != want {
		t.Fatalf("provider.Name() = %q, want %q", got, want)
	}
}

func TestInitProviderOpenAICompletionDefault(t *testing.T) {
	provider, err := initProvider(configs.ModelConfig{Model: "gpt-4o-mini", Key: "mscli-token"}, providerResolveNoOverrides())
	if err != nil {
		t.Fatalf("initProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("initProvider() provider = nil, want provider")
	}
	if got, want := provider.Name(), "openai-completion"; got != want {
		t.Fatalf("provider.Name() = %q, want %q", got, want)
	}
}

func TestInitProviderMapsMissingKeyToAppSentinel(t *testing.T) {
	_, err := initProvider(configs.ModelConfig{Model: "gpt-4o-mini"}, providerResolveNoOverrides())
	if err == nil {
		t.Fatal("initProvider() error = nil, want missing api key error")
	}
	if !errors.Is(err, errAPIKeyNotFound) {
		t.Fatalf("initProvider() error = %v, want errAPIKeyNotFound", err)
	}
}

func TestWireBootstrapKeyAndURLOverrideEnvDuringProviderInit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Setenv("MSCLI_PROVIDER", "openai-completion")
	t.Setenv("MSCLI_API_KEY", "env-key")
	t.Setenv("MSCLI_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var gotAuth string
	var gotPath string
	origBuildProvider := buildProvider
	buildProvider = func(resolved llm.ResolvedConfig) (llm.Provider, error) {
		return newOpenAICompletionTestProvider(t, resolved, func(req *http.Request) {
			gotPath = req.URL.Path
			gotAuth = req.Header.Get("Authorization")
		}), nil
	}
	defer func() { buildProvider = origBuildProvider }()

	app, err := Wire(BootstrapConfig{
		URL:   "https://example.test/v1",
		Key:   "cli-key",
		Model: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("Wire() error = %v", err)
	}
	if got, want := app.Config.Model.URL, "https://example.test/v1"; got != want {
		t.Fatalf("config.Model.URL = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.Key, "cli-key"; got != want {
		t.Fatalf("config.Model.Key = %q, want %q", got, want)
	}

	_, err = app.provider.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "ping"},
		},
	})
	if err != nil {
		t.Fatalf("provider.Complete() error = %v", err)
	}

	if got, want := gotPath, "/v1/chat/completions"; got != want {
		t.Fatalf("request path = %q, want %q", got, want)
	}
	if got, want := gotAuth, "Bearer cli-key"; got != want {
		t.Fatalf("Authorization header = %q, want %q", got, want)
	}
}

func providerResolveNoOverrides() llm.ResolveOptions {
	return llm.ResolveOptions{}
}
