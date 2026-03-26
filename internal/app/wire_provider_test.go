package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/vigo999/ms-cli/agent/loop"
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

type captureStreamProvider struct {
	lastReq *llm.CompletionRequest
}

func (p *captureStreamProvider) Name() string {
	return "capture"
}

func (p *captureStreamProvider) Complete(context.Context, *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, io.EOF
}

func (p *captureStreamProvider) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	copied := *req
	copied.Messages = append([]llm.Message(nil), req.Messages...)
	copied.Tools = append([]llm.Tool(nil), req.Tools...)
	p.lastReq = &copied

	return &captureTestStreamIterator{
		chunks: []llm.StreamChunk{{
			Content:      "ok",
			FinishReason: llm.FinishStop,
		}},
	}, nil
}

func (p *captureStreamProvider) SupportsTools() bool {
	return true
}

func (p *captureStreamProvider) AvailableModels() []llm.ModelInfo {
	return nil
}

type captureTestStreamIterator struct {
	chunks []llm.StreamChunk
	index  int
}

func (it *captureTestStreamIterator) Next() (*llm.StreamChunk, error) {
	if it.index >= len(it.chunks) {
		return nil, io.EOF
	}
	chunk := it.chunks[it.index]
	it.index++
	return &chunk, nil
}

func (it *captureTestStreamIterator) Close() error {
	return nil
}

func TestWirePassesMSCLIMaxTokensToModelRequests(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MSCLI_PROVIDER", "anthropic")
	t.Setenv("MSCLI_API_KEY", "anthropic-token")
	t.Setenv("MSCLI_MODEL", "claude-sonnet-4-5")
	t.Setenv("MSCLI_MAX_TOKENS", "2048")

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	provider := &captureStreamProvider{}
	origBuildProvider := buildProvider
	buildProvider = func(resolved llm.ResolvedConfig) (llm.Provider, error) {
		return provider, nil
	}
	defer func() { buildProvider = origBuildProvider }()

	app, err := Wire(BootstrapConfig{})
	if err != nil {
		t.Fatalf("Wire() error = %v", err)
	}

	_, err = app.Engine.Run(loop.Task{
		ID:          "wire-max-tokens",
		Description: "ping",
	})
	if err != nil {
		t.Fatalf("Engine.Run() error = %v", err)
	}

	if provider.lastReq == nil {
		t.Fatal("expected provider to receive completion request")
	}
	if provider.lastReq.MaxTokens == nil {
		t.Fatal("provider.lastReq.MaxTokens = nil, want value")
	}
	if got, want := *provider.lastReq.MaxTokens, 2048; got != want {
		t.Fatalf("provider.lastReq.MaxTokens = %d, want %d", got, want)
	}
	if provider.lastReq.Temperature != nil {
		t.Fatalf("provider.lastReq.Temperature = %v, want nil", *provider.lastReq.Temperature)
	}
}

func TestWireOmitsRequestOverridesWhenEnvUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MSCLI_PROVIDER", "openai-completion")
	t.Setenv("MSCLI_API_KEY", "token")
	t.Setenv("MSCLI_MODEL", "gpt-4o-mini")

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	provider := &captureStreamProvider{}
	origBuildProvider := buildProvider
	buildProvider = func(resolved llm.ResolvedConfig) (llm.Provider, error) {
		return provider, nil
	}
	defer func() { buildProvider = origBuildProvider }()

	app, err := Wire(BootstrapConfig{})
	if err != nil {
		t.Fatalf("Wire() error = %v", err)
	}

	_, err = app.Engine.Run(loop.Task{
		ID:          "wire-no-overrides",
		Description: "ping",
	})
	if err != nil {
		t.Fatalf("Engine.Run() error = %v", err)
	}

	if provider.lastReq == nil {
		t.Fatal("expected provider to receive completion request")
	}
	if provider.lastReq.MaxTokens != nil {
		t.Fatalf("provider.lastReq.MaxTokens = %d, want nil", *provider.lastReq.MaxTokens)
	}
	if provider.lastReq.Temperature != nil {
		t.Fatalf("provider.lastReq.Temperature = %v, want nil", *provider.lastReq.Temperature)
	}
}

func TestWirePassesMSCLITemperatureToModelRequests(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MSCLI_PROVIDER", "openai-completion")
	t.Setenv("MSCLI_API_KEY", "token")
	t.Setenv("MSCLI_MODEL", "gpt-4o-mini")
	t.Setenv("MSCLI_TEMPERATURE", "0.25")

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	provider := &captureStreamProvider{}
	origBuildProvider := buildProvider
	buildProvider = func(resolved llm.ResolvedConfig) (llm.Provider, error) {
		return provider, nil
	}
	defer func() { buildProvider = origBuildProvider }()

	app, err := Wire(BootstrapConfig{})
	if err != nil {
		t.Fatalf("Wire() error = %v", err)
	}

	_, err = app.Engine.Run(loop.Task{
		ID:          "wire-temperature",
		Description: "ping",
	})
	if err != nil {
		t.Fatalf("Engine.Run() error = %v", err)
	}

	if provider.lastReq == nil {
		t.Fatal("expected provider to receive completion request")
	}
	if provider.lastReq.Temperature == nil {
		t.Fatal("provider.lastReq.Temperature = nil, want value")
	}
	if got, want := *provider.lastReq.Temperature, float32(0.25); got != want {
		t.Fatalf("provider.lastReq.Temperature = %v, want %v", got, want)
	}
}
