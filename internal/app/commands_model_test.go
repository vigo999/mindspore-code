package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindspore-lab/mindspore-cli/configs"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestCmdModel_UnprefixedKeepsProvider(t *testing.T) {
	app := newModelCommandTestApp()
	app.Config.Model.Provider = "anthropic"
	app.Config.Model.Model = "claude-3-5-sonnet"

	app.cmdModel([]string{"claude-3-5-haiku"})

	drainUntilEventType(t, app, model.AgentThinking)
	drainUntilEventType(t, app, model.ModelUpdate)
	drainUntilEventType(t, app, model.AgentReply)

	if got, want := app.Config.Model.Provider, "anthropic"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.Model, "claude-3-5-haiku"; got != want {
		t.Fatalf("model = %q, want %q", got, want)
	}
}

func TestCmdModel_PrefixedUpdatesProviderAndModel(t *testing.T) {
	app := newModelCommandTestApp()
	app.Config.Model.Provider = "openai-completion"
	app.Config.Model.Model = "gpt-4o-mini"

	app.cmdModel([]string{"anthropic:claude-3-5-sonnet"})

	drainUntilEventType(t, app, model.AgentThinking)
	drainUntilEventType(t, app, model.ModelUpdate)
	drainUntilEventType(t, app, model.AgentReply)

	if got, want := app.Config.Model.Provider, "anthropic"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.Model, "claude-3-5-sonnet"; got != want {
		t.Fatalf("model = %q, want %q", got, want)
	}
}

func TestCmdModel_ModelUpdateCarriesContextWindow(t *testing.T) {
	app := newModelCommandTestApp()
	app.Config.Context.Window = 200000

	app.cmdModel([]string{"gpt-4o"})

	drainUntilEventType(t, app, model.AgentThinking)
	ev := drainUntilEventType(t, app, model.ModelUpdate)

	if got, want := ev.CtxMax, 200000; got != want {
		t.Fatalf("model update ctx max = %d, want %d", got, want)
	}
}

func TestCmdModel_InvalidPrefixNoMutation(t *testing.T) {
	app := newModelCommandTestApp()
	app.Config.Model.Provider = "openai-completion"
	app.Config.Model.Model = "gpt-4o-mini"

	app.cmdModel([]string{"invalid:gpt-4o"})

	ev := drainUntilEventType(t, app, model.AgentReply)
	if !strings.Contains(ev.Message, "Unsupported provider prefix") {
		t.Fatalf("unexpected message: %q", ev.Message)
	}

	if got, want := app.Config.Model.Provider, "openai-completion"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.Model, "gpt-4o-mini"; got != want {
		t.Fatalf("model = %q, want %q", got, want)
	}
}

func TestCmdModel_NoArgsShowsModelPicker(t *testing.T) {
	app := newModelCommandTestApp()
	t.Setenv("HOME", t.TempDir())
	if err := saveCredentials(&credentials{
		ServerURL: "https://mscli.dev",
		Token:     "user-token",
		User:      "alice",
		Role:      "user",
	}); err != nil {
		t.Fatalf("saveCredentials() error = %v", err)
	}

	app.cmdModel(nil)

	ev := drainUntilEventType(t, app, model.ModelPickerOpen)
	if ev.Popup == nil {
		t.Fatal("ModelPickerOpen popup = nil, want SelectionPopup")
	}
	if len(ev.Popup.Options) == 0 {
		t.Fatal("expected model picker options")
	}
	if got, want := ev.Popup.ActionID, "model_picker"; got != want {
		t.Fatalf("popup.ActionID = %q, want %q", got, want)
	}
}

func TestCmdModel_BuiltinPresetRequiresLogin(t *testing.T) {
	app := newModelCommandTestApp()
	t.Setenv("HOME", t.TempDir())

	app.cmdModel([]string{"kimi-k2.5-free"})

	drainUntilEventType(t, app, model.AgentThinking)
	ev := drainUntilEventType(t, app, model.ToolError)
	if !strings.Contains(ev.Message, "not logged in") {
		t.Fatalf("tool error = %q, want login requirement", ev.Message)
	}
}

func TestCmdModel_BuiltinPresetUsesServerCredentialAndRestoresOnSwitchBack(t *testing.T) {
	app := newModelCommandTestApp()
	app.Config.Model.Provider = "openai-completion"
	app.Config.Model.URL = "https://api.openai.com/v1"
	app.Config.Model.Model = "gpt-4o-mini"
	app.Config.Model.Key = "env-key"

	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/model-presets/kimi-k2.5-free/credential" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"api_key": "server-kimi-key"})
	}))
	t.Cleanup(srv.Close)

	t.Setenv("HOME", t.TempDir())
	cred := credentials{
		ServerURL: srv.URL,
		Token:     "user-token",
		User:      "alice",
		Role:      "user",
	}
	if err := saveCredentials(&cred); err != nil {
		t.Fatalf("saveCredentials() error = %v", err)
	}

	var resolved llm.ResolvedConfig
	origBuildProvider := buildProvider
	buildProvider = func(cfg llm.ResolvedConfig) (llm.Provider, error) {
		resolved = cfg
		return &blockingStreamProvider{started: make(chan struct{})}, nil
	}
	defer func() { buildProvider = origBuildProvider }()

	app.cmdModel([]string{"kimi-k2.5-free"})
	drainUntilEventType(t, app, model.AgentThinking)
	drainUntilEventType(t, app, model.ModelUpdate)
	drainUntilEventType(t, app, model.AgentReply)

	if got, want := capturedAuth, "Bearer user-token"; got != want {
		t.Fatalf("credential request auth = %q, want %q", got, want)
	}
	if got, want := string(resolved.Kind), "anthropic"; got != want {
		t.Fatalf("resolved provider = %q, want %q", got, want)
	}
	if got, want := resolved.BaseURL, "https://api.kimi.com/coding/"; got != want {
		t.Fatalf("resolved base url = %q, want %q", got, want)
	}
	if got, want := resolved.Model, "kimi-k2.5"; got != want {
		t.Fatalf("resolved model = %q, want %q", got, want)
	}
	if got, want := resolved.APIKey, "server-kimi-key"; got != want {
		t.Fatalf("resolved key = %q, want %q", got, want)
	}

	app.cmdModel([]string{"gpt-4o"})
	drainUntilEventType(t, app, model.AgentThinking)
	drainUntilEventType(t, app, model.ModelUpdate)
	drainUntilEventType(t, app, model.AgentReply)

	if got, want := app.Config.Model.Provider, "openai-completion"; got != want {
		t.Fatalf("provider after restore = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.URL, "https://api.openai.com/v1"; got != want {
		t.Fatalf("url after restore = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.Key, "env-key"; got != want {
		t.Fatalf("key after restore = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.Model, "gpt-4o"; got != want {
		t.Fatalf("model after switch = %q, want %q", got, want)
	}
}

func TestCmdModel_LogicalProviderSelectionWorksWithoutCache(t *testing.T) {
	app := newModelCommandTestApp()
	home := t.TempDir()
	t.Setenv("HOME", home)

	origAuthPath := authStatePathOverride
	authStatePathOverride = filepath.Join(home, ".mscli", "auth.json")
	t.Cleanup(func() { authStatePathOverride = origAuthPath })
	origModelPath := modelStatePathOverride
	modelStatePathOverride = filepath.Join(home, ".mscli", "model.json")
	t.Cleanup(func() { modelStatePathOverride = origModelPath })
	origCache := modelsDevCachePathOverride
	modelsDevCachePathOverride = filepath.Join(home, ".mscli", "cached", "models-dev-api.json")
	t.Cleanup(func() { modelsDevCachePathOverride = origCache })
	origURL := modelsDevAPIURL
	server := newModelsDevTestServer(`{
		"openrouter": {
			"id": "openrouter",
			"name": "OpenRouter",
			"api": "https://openrouter.ai/api/v1",
			"npm": "@openrouter/ai-sdk-provider",
			"models": {"openai/gpt-4o-mini": {"id": "openai/gpt-4o-mini", "name": "GPT-4o mini"}}
		}
	}`)
	defer server.Close()
	modelsDevAPIURL = server.URL
	t.Cleanup(func() { modelsDevAPIURL = origURL })

	resetModelsDevProviderCacheForTest()
	t.Cleanup(resetModelsDevProviderCacheForTest)

	if err := saveProviderAuthState(&providerAuthState{
		Providers: map[string]providerAuthEntry{
			"openrouter": {ProviderID: "openrouter", APIKey: "sk-openrouter"},
		},
	}); err != nil {
		t.Fatalf("saveProviderAuthState() error = %v", err)
	}

	app.cmdModel([]string{"openrouter:openai/gpt-4o-mini"})

	drainUntilEventType(t, app, model.ModelUpdate)
	drainUntilEventType(t, app, model.AgentReply)

	if got, want := app.Config.Model.Provider, "openai-completion"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
	if got, want := app.Config.Model.Model, "openai/gpt-4o-mini"; got != want {
		t.Fatalf("model = %q, want %q", got, want)
	}
}

func TestCmdModel_LogicalProviderSelectionPreservesUnderlyingError(t *testing.T) {
	app := newModelCommandTestApp()
	home := t.TempDir()
	t.Setenv("HOME", home)

	origAuthPath := authStatePathOverride
	authStatePathOverride = filepath.Join(home, ".mscli", "auth.json")
	t.Cleanup(func() { authStatePathOverride = origAuthPath })
	origCache := modelsDevCachePathOverride
	modelsDevCachePathOverride = filepath.Join(home, ".mscli", "cached", "models-dev-api.json")
	t.Cleanup(func() { modelsDevCachePathOverride = origCache })
	origURL := modelsDevAPIURL
	server := newModelsDevTestServer(`{
		"openrouter": {
			"id": "openrouter",
			"name": "OpenRouter",
			"api": "https://openrouter.ai/api/v1",
			"npm": "@openrouter/ai-sdk-provider",
			"models": {"openai/gpt-4o-mini": {"id": "openai/gpt-4o-mini", "name": "GPT-4o mini"}}
		}
	}`)
	defer server.Close()
	modelsDevAPIURL = server.URL
	t.Cleanup(func() { modelsDevAPIURL = origURL })

	resetModelsDevProviderCacheForTest()
	t.Cleanup(resetModelsDevProviderCacheForTest)

	app.cmdModel([]string{"openrouter:openai/gpt-4o-mini"})

	ev := drainUntilEventType(t, app, model.AgentReply)
	if !strings.Contains(ev.Message, "not connected") {
		t.Fatalf("message = %q, want underlying connection error", ev.Message)
	}
}

func TestCmdConnect_NoArgsUsesCachedCatalogWithoutBlocking(t *testing.T) {
	app := newModelCommandTestApp()
	home := t.TempDir()
	t.Setenv("HOME", home)

	origCache := modelsDevCachePathOverride
	modelsDevCachePathOverride = filepath.Join(home, ".mscli", "cached", "models-dev-api.json")
	t.Cleanup(func() { modelsDevCachePathOverride = origCache })
	origURL := modelsDevAPIURL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"openrouter": {
				"id": "openrouter",
				"name": "OpenRouter",
				"npm": "@openrouter/ai-sdk-provider",
				"models": {}
			}
		}`))
	}))
	defer server.Close()
	modelsDevAPIURL = server.URL
	t.Cleanup(func() { modelsDevAPIURL = origURL })

	resetModelsDevProviderCacheForTest()
	t.Cleanup(resetModelsDevProviderCacheForTest)

	cached := `{
		"openrouter": {
			"id": "openrouter",
			"name": "OpenRouter",
			"npm": "@openrouter/ai-sdk-provider",
			"models": {}
		}
	}`
	if err := writeModelsDevCache([]byte(cached)); err != nil {
		t.Fatalf("writeModelsDevCache() error = %v", err)
	}

	start := time.Now()
	app.cmdConnect(nil)
	if elapsed := time.Since(start); elapsed >= 500*time.Millisecond {
		t.Fatalf("cmdConnect(nil) took %v, want under 500ms", elapsed)
	}

	ev := drainUntilEventType(t, app, model.ModelSetupOpen)
	if ev.SetupPopup == nil {
		t.Fatal("ModelSetupOpen popup = nil")
	}
}

func TestCmdConnect_NoArgsShowsProviderPickerWithFreeLoginHint(t *testing.T) {
	app := newModelCommandTestApp()
	home := t.TempDir()
	t.Setenv("HOME", home)
	origCache := modelsDevCachePathOverride
	modelsDevCachePathOverride = filepath.Join(home, ".mscli", "cached", "models-dev-api.json")
	t.Cleanup(func() { modelsDevCachePathOverride = origCache })
	origURL := modelsDevAPIURL
	server := newModelsDevTestServer(`{
		"anthropic": {"id": "anthropic", "name": "Anthropic", "npm": "@ai-sdk/anthropic", "models": {}},
		"openai": {"id": "openai", "name": "OpenAI", "npm": "@ai-sdk/openai", "models": {}},
		"kimi-for-coding": {"id": "kimi-for-coding", "name": "Kimi for Coding", "npm": "@ai-sdk/openai-compatible", "models": {}},
		"deepseek": {"id": "deepseek", "name": "DeepSeek", "npm": "@ai-sdk/openai-compatible", "models": {}},
		"openrouter": {"id": "openrouter", "name": "OpenRouter", "npm": "@openrouter/ai-sdk-provider", "models": {}}
	}`)
	defer server.Close()
	modelsDevAPIURL = server.URL
	t.Cleanup(func() { modelsDevAPIURL = origURL })
	resetModelsDevProviderCacheForTest()
	t.Cleanup(resetModelsDevProviderCacheForTest)
	if err := writeModelsDevCache([]byte(`{
		"anthropic": {"id": "anthropic", "name": "Anthropic", "npm": "@ai-sdk/anthropic", "models": {}},
		"openai": {"id": "openai", "name": "OpenAI", "npm": "@ai-sdk/openai", "models": {}},
		"kimi-for-coding": {"id": "kimi-for-coding", "name": "Kimi for Coding", "npm": "@ai-sdk/openai-compatible", "models": {}},
		"deepseek": {"id": "deepseek", "name": "DeepSeek", "npm": "@ai-sdk/openai-compatible", "models": {}},
		"openrouter": {"id": "openrouter", "name": "OpenRouter", "npm": "@openrouter/ai-sdk-provider", "models": {}}
	}`)); err != nil {
		t.Fatalf("writeModelsDevCache() error = %v", err)
	}

	app.cmdConnect(nil)

	ev := drainUntilEventType(t, app, model.ModelSetupOpen)
	if ev.SetupPopup == nil {
		t.Fatal("ModelSetupOpen popup = nil, want SetupPopup")
	}
	if got, want := ev.SetupPopup.Title, "Connect Provider"; got != want {
		t.Fatalf("popup.Title = %q, want %q", got, want)
	}
	if len(ev.SetupPopup.PresetOptions) == 0 {
		t.Fatal("expected provider options")
	}
	if got, want := ev.SetupPopup.PresetOptions[0].Label, "Popular"; got != want {
		t.Fatalf("first option label = %q, want %q", got, want)
	}
	if got, want := ev.SetupPopup.PresetOptions[1].ID, "anthropic"; got != want {
		t.Fatalf("second option ID = %q, want %q", got, want)
	}
	if got, want := ev.SetupPopup.PresetOptions[5].Separator, true; got != want {
		t.Fatalf("separator before other = %v, want %v", got, want)
	}
	if got, want := ev.SetupPopup.PresetOptions[6].Label, "Other"; got != want {
		t.Fatalf("other header label = %q, want %q", got, want)
	}
	firstOther := ev.SetupPopup.PresetOptions[7]
	if got, want := firstOther.ID, mindsporeCLIFreeProviderID; got != want {
		t.Fatalf("first other provider ID = %q, want %q", got, want)
	}
	if !firstOther.Disabled {
		t.Fatal("expected free provider to be disabled before login")
	}
	if !strings.Contains(firstOther.Desc, "require login") {
		t.Fatalf("firstOther.Desc = %q, want require login hint", firstOther.Desc)
	}
}

func TestCmdConnect_LoggedInShowsFreeAsRecommendedFirstPopularItem(t *testing.T) {
	app := newModelCommandTestApp()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := saveCredentials(&credentials{
		ServerURL: "https://mscli.dev",
		Token:     "user-token",
		User:      "alice",
		Role:      "user",
	}); err != nil {
		t.Fatalf("saveCredentials() error = %v", err)
	}
	origCache := modelsDevCachePathOverride
	modelsDevCachePathOverride = filepath.Join(home, ".mscli", "cached", "models-dev-api.json")
	t.Cleanup(func() { modelsDevCachePathOverride = origCache })
	origURL := modelsDevAPIURL
	server := newModelsDevTestServer(`{
		"anthropic": {"id": "anthropic", "name": "Anthropic", "npm": "@ai-sdk/anthropic", "models": {}},
		"openai": {"id": "openai", "name": "OpenAI", "npm": "@ai-sdk/openai", "models": {}}
	}`)
	defer server.Close()
	modelsDevAPIURL = server.URL
	t.Cleanup(func() { modelsDevAPIURL = origURL })
	resetModelsDevProviderCacheForTest()
	t.Cleanup(resetModelsDevProviderCacheForTest)
	if err := writeModelsDevCache([]byte(`{
		"anthropic": {"id": "anthropic", "name": "Anthropic", "npm": "@ai-sdk/anthropic", "models": {}},
		"openai": {"id": "openai", "name": "OpenAI", "npm": "@ai-sdk/openai", "models": {}}
	}`)); err != nil {
		t.Fatalf("writeModelsDevCache() error = %v", err)
	}

	app.cmdConnect(nil)

	ev := drainUntilEventType(t, app, model.ModelSetupOpen)
	if ev.SetupPopup == nil {
		t.Fatal("ModelSetupOpen popup = nil, want SetupPopup")
	}
	if got, want := ev.SetupPopup.PresetOptions[0].Label, "Popular"; got != want {
		t.Fatalf("popular header label = %q, want %q", got, want)
	}
	firstPopular := ev.SetupPopup.PresetOptions[1]
	if got, want := firstPopular.ID, mindsporeCLIFreeProviderID; got != want {
		t.Fatalf("first popular ID = %q, want %q", got, want)
	}
	if firstPopular.Disabled {
		t.Fatal("expected free provider enabled after login")
	}
	if !strings.Contains(firstPopular.Desc, "Recommended") {
		t.Fatalf("firstPopular.Desc = %q, want Recommended", firstPopular.Desc)
	}
}

func TestCmdConnect_ConfiguredProviderStillRequiresAPIKeyInput(t *testing.T) {
	app := newModelCommandTestApp()
	home := t.TempDir()
	t.Setenv("HOME", home)
	origAuthPath := authStatePathOverride
	authStatePathOverride = filepath.Join(home, ".mscli", "auth.json")
	t.Cleanup(func() { authStatePathOverride = origAuthPath })
	origCache := modelsDevCachePathOverride
	modelsDevCachePathOverride = filepath.Join(home, ".mscli", "cached", "models-dev-api.json")
	t.Cleanup(func() { modelsDevCachePathOverride = origCache })
	origURL := modelsDevAPIURL
	server := newModelsDevTestServer(`{
		"openrouter": {
			"id": "openrouter",
			"name": "OpenRouter",
			"api": "https://openrouter.ai/api/v1",
			"npm": "@openrouter/ai-sdk-provider",
			"models": {"openai/gpt-4o-mini": {"id": "openai/gpt-4o-mini", "name": "GPT-4o mini"}}
		}
	}`)
	defer server.Close()
	modelsDevAPIURL = server.URL
	t.Cleanup(func() { modelsDevAPIURL = origURL })
	resetModelsDevProviderCacheForTest()
	t.Cleanup(resetModelsDevProviderCacheForTest)
	if err := writeModelsDevCache([]byte(`{
		"openrouter": {
			"id": "openrouter",
			"name": "OpenRouter",
			"api": "https://openrouter.ai/api/v1",
			"npm": "@openrouter/ai-sdk-provider",
			"models": {"openai/gpt-4o-mini": {"id": "openai/gpt-4o-mini", "name": "GPT-4o mini"}}
		}
	}`)); err != nil {
		t.Fatalf("writeModelsDevCache() error = %v", err)
	}
	if err := saveProviderAuthState(&providerAuthState{
		Providers: map[string]providerAuthEntry{
			"openrouter": {ProviderID: "openrouter", APIKey: "sk-old"},
		},
	}); err != nil {
		t.Fatalf("saveProviderAuthState() error = %v", err)
	}

	app.cmdConnect(nil)

	ev := drainUntilEventType(t, app, model.ModelSetupOpen)
	if ev.SetupPopup == nil {
		t.Fatal("ModelSetupOpen popup = nil")
	}
	var openrouter model.SelectionOption
	found := false
	for _, opt := range ev.SetupPopup.PresetOptions {
		if opt.ID == "openrouter" {
			openrouter = opt
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected openrouter option")
	}
	if strings.Contains(openrouter.Desc, "connected") {
		t.Fatalf("openrouter.Desc = %q, want no connected hint", openrouter.Desc)
	}
	if !openrouter.RequiresInput {
		t.Fatal("expected openrouter to require API key input even when already configured")
	}
}

func TestCmdConnect_WithAPIKeyPersistsAuthAndOpensProviderScopedModelPicker(t *testing.T) {
	app := newModelCommandTestApp()
	home := t.TempDir()
	t.Setenv("HOME", home)
	origCache := modelsDevCachePathOverride
	modelsDevCachePathOverride = filepath.Join(home, ".mscli", "cached", "models-dev-api.json")
	t.Cleanup(func() { modelsDevCachePathOverride = origCache })
	origURL := modelsDevAPIURL
	server := newModelsDevTestServer(`{
		"openrouter": {
			"id": "openrouter",
			"name": "OpenRouter",
			"api": "https://openrouter.ai/api/v1",
			"npm": "@openrouter/ai-sdk-provider",
			"models": {"openai/gpt-4o-mini": {"id": "openai/gpt-4o-mini", "name": "GPT-4o mini"}}
		}
	}`)
	defer server.Close()
	modelsDevAPIURL = server.URL
	t.Cleanup(func() { modelsDevAPIURL = origURL })

	app.cmdConnect([]string{"openrouter", "sk-openrouter"})

	drainUntilEventType(t, app, model.AgentThinking)
	ev := drainUntilEventType(t, app, model.ModelSetupClose)
	if got, want := ev.Type, model.ModelSetupClose; got != want {
		t.Fatalf("event type = %s, want %s", got, want)
	}
	picker := drainUntilEventType(t, app, model.ModelPickerOpen)
	if picker.Popup == nil {
		t.Fatal("ModelPickerOpen popup = nil")
	}
	if got, want := picker.Popup.ActionID, "connect_provider_model_picker:openrouter"; got != want {
		t.Fatalf("picker.Popup.ActionID = %q, want %q", got, want)
	}
	if len(picker.Popup.Options) != 1 {
		t.Fatalf("len(picker.Popup.Options) = %d, want 1 provider-scoped model option", len(picker.Popup.Options))
	}
	if got, want := picker.Popup.Options[0].ID, "openrouter:openai/gpt-4o-mini"; got != want {
		t.Fatalf("picker.Popup.Options[0].ID = %q, want %q", got, want)
	}
	authState, err := loadProviderAuthState()
	if err != nil {
		t.Fatalf("loadProviderAuthState() error = %v", err)
	}
	entry, ok := authState.Providers["openrouter"]
	if !ok {
		t.Fatalf("authState.Providers = %#v, want openrouter entry", authState.Providers)
	}
	if got, want := entry.APIKey, "sk-openrouter"; got != want {
		t.Fatalf("entry.APIKey = %q, want %q", got, want)
	}
}

func newModelCommandTestApp() *Application {
	cfg := configs.DefaultConfig()
	cfg.Model.Key = "test-key"
	cfg.Server.URL = "https://issues.example"
	return &Application{
		EventCh: make(chan model.Event, 16),
		Config:  cfg,
	}
}

func drainUntilEventType(t *testing.T, app *Application, target model.EventType) model.Event {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	for {
		select {
		case ev := <-app.EventCh:
			if ev.Type == target {
				return ev
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for event type %s", target)
		}
	}
}
