package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

type testProvider struct {
	name string
}

func (p *testProvider) Name() string { return p.name }
func (p *testProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, nil
}
func (p *testProvider) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	return nil, nil
}
func (p *testProvider) SupportsTools() bool              { return false }
func (p *testProvider) AvailableModels() []llm.ModelInfo { return nil }

func TestManagerBuild_CacheHitReturnsSameInstance(t *testing.T) {
	m := NewManager()

	var builds int
	if err := m.Register(ProviderOpenAICompatible, func(cfg ResolvedConfig) (llm.Provider, error) {
		builds++
		return &testProvider{name: "one"}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	cfg := ResolvedConfig{
		Kind:           ProviderOpenAICompatible,
		BaseURL:        "https://example.invalid/v1",
		Model:          "gpt-test",
		Timeout:        30 * time.Second,
		Headers:        map[string]string{"X-Test": "1"},
		AuthHeaderName: "Authorization",
		APIKey:         "secret",
	}

	first, err := m.Build(cfg)
	if err != nil {
		t.Fatalf("Build() first error = %v", err)
	}
	second, err := m.Build(cfg)
	if err != nil {
		t.Fatalf("Build() second error = %v", err)
	}

	if first != second {
		t.Fatalf("Build() provider instances differ: %p vs %p", first, second)
	}
	if builds != 1 {
		t.Fatalf("builder called %d times, want 1", builds)
	}
}

func TestManagerBuild_DifferentConfigProducesDifferentInstances(t *testing.T) {
	m := NewManager()

	var builds int
	if err := m.Register(ProviderOpenAICompatible, func(cfg ResolvedConfig) (llm.Provider, error) {
		builds++
		return &testProvider{name: cfg.Model}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	base := ResolvedConfig{
		Kind:           ProviderOpenAICompatible,
		BaseURL:        "https://example.invalid/v1",
		Model:          "gpt-test-a",
		Timeout:        30 * time.Second,
		Headers:        map[string]string{"X-Test": "1"},
		AuthHeaderName: "Authorization",
		APIKey:         "secret",
	}
	other := base
	other.Model = "gpt-test-b"

	first, err := m.Build(base)
	if err != nil {
		t.Fatalf("Build() first error = %v", err)
	}
	second, err := m.Build(other)
	if err != nil {
		t.Fatalf("Build() second error = %v", err)
	}

	if first == second {
		t.Fatal("Build() returned same instance for different configs")
	}
	if builds != 2 {
		t.Fatalf("builder called %d times, want 2", builds)
	}
}

func TestManagerBuild_UnregisteredKindReturnsError(t *testing.T) {
	m := NewManager()

	_, err := m.Build(ResolvedConfig{Kind: ProviderAnthropic})
	if err == nil {
		t.Fatal("Build() error = nil, want unregistered provider error")
	}
	if got := err.Error(); got == "" {
		t.Fatal("Build() error message is empty")
	}
	if errors.Is(err, errProviderNotImplemented) {
		t.Fatal("Build() returned not-implemented error for unregistered kind")
	}
}

func TestCacheKey_HeaderCanonicalizationDeterministic(t *testing.T) {
	base := ResolvedConfig{
		Kind:           ProviderOpenAICompatible,
		BaseURL:        "https://example.invalid/v1",
		Model:          "gpt-test",
		Timeout:        30 * time.Second,
		AuthHeaderName: "Authorization",
		APIKey:         "secret",
	}

	sameLogicalHeadersA := base
	sameLogicalHeadersA.Headers = map[string]string{
		"X-Trace-ID": "trace-123",
		"X-Feature":  "on",
	}
	sameLogicalHeadersB := base
	sameLogicalHeadersB.Headers = map[string]string{
		"x-feature":  "on",
		"x-trace-id": "trace-123",
	}

	if gotA, gotB := cacheKey(sameLogicalHeadersA), cacheKey(sameLogicalHeadersB); gotA != gotB {
		t.Fatalf("cacheKey() mismatch for equivalent headers: %q vs %q", gotA, gotB)
	}

	duplicateCaseVariant := base
	duplicateCaseVariant.Headers = map[string]string{
		"X-Trace-ID": "trace-123",
		"x-trace-id": "trace-999",
	}

	want := cacheKey(duplicateCaseVariant)
	for i := 0; i < 100; i++ {
		if got := cacheKey(duplicateCaseVariant); got != want {
			t.Fatalf("cacheKey() changed across calls with duplicate case-variant headers: got %q, want %q", got, want)
		}
	}
}

func TestRegistry_RegisterAndBuildSuccess(t *testing.T) {
	r := NewRegistry()

	var gotCfg ResolvedConfig
	if err := r.Register(ProviderOpenAICompatible, func(cfg ResolvedConfig) (llm.Provider, error) {
		gotCfg = cfg
		return &testProvider{name: "registry"}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	cfg := ResolvedConfig{Kind: ProviderOpenAICompatible, Model: "gpt-test"}
	got, err := r.Build(cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got == nil {
		t.Fatal("Build() provider = nil, want provider")
	}
	if gotCfg.Kind != cfg.Kind || gotCfg.Model != cfg.Model {
		t.Fatalf("builder received cfg = %#v, want %#v", gotCfg, cfg)
	}
}

func TestRegistry_RegisterErrors(t *testing.T) {
	t.Run("nil builder", func(t *testing.T) {
		r := NewRegistry()
		if err := r.Register(ProviderOpenAICompatible, nil); err == nil {
			t.Fatal("Register() error = nil, want nil builder error")
		}
	})

	t.Run("duplicate kind", func(t *testing.T) {
		r := NewRegistry()
		builder := func(cfg ResolvedConfig) (llm.Provider, error) {
			return &testProvider{name: "dup"}, nil
		}

		if err := r.Register(ProviderOpenAICompatible, builder); err != nil {
			t.Fatalf("Register() first error = %v", err)
		}
		if err := r.Register(ProviderOpenAICompatible, builder); err == nil {
			t.Fatal("Register() error = nil, want duplicate kind error")
		}
	})
}

type fakeHTTPClient struct {
	do func(*http.Request) (*http.Response, error)
}

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return f.do(req)
}

func TestDoJSON_BuildsAndSendsRequestWithHeaders(t *testing.T) {
	var seenMethod string
	var seenURL string
	var seenContentType string
	var seenAuth string
	var seenTrace string
	var seenBody string

	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		seenMethod = req.Method
		seenURL = req.URL.String()
		seenContentType = req.Header.Get("Content-Type")
		seenAuth = req.Header.Get("Authorization")
		seenTrace = req.Header.Get("X-Trace-ID")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		seenBody = string(body)

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	}}

	resp, err := DoJSON(context.Background(), client, http.MethodPost, "https://example.invalid/v1/chat", map[string]string{
		"Authorization": "Bearer token",
		"X-Trace-ID":    "trace-123",
	}, map[string]string{"model": "gpt-test"})
	if err != nil {
		t.Fatalf("DoJSON() error = %v", err)
	}
	if resp == nil {
		t.Fatal("DoJSON() response = nil, want response")
	}

	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", seenMethod, http.MethodPost)
	}
	if seenURL != "https://example.invalid/v1/chat" {
		t.Fatalf("url = %q, want %q", seenURL, "https://example.invalid/v1/chat")
	}
	if seenContentType != "application/json" {
		t.Fatalf("content-type = %q, want %q", seenContentType, "application/json")
	}
	if seenAuth != "Bearer token" {
		t.Fatalf("authorization = %q, want %q", seenAuth, "Bearer token")
	}
	if seenTrace != "trace-123" {
		t.Fatalf("x-trace-id = %q, want %q", seenTrace, "trace-123")
	}
	if want := `{"model":"gpt-test"}`; seenBody != want {
		t.Fatalf("body = %q, want %q", seenBody, want)
	}
}
