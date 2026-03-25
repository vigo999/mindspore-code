package configs

import "testing"

func TestLoadWithEnv_AutoTokenLimitsByModel(t *testing.T) {
	clearEnv(t)
	t.Setenv("MSCLI_MODEL", "gpt-5")

	cfg, err := LoadWithEnv()
	if err != nil {
		t.Fatalf("LoadWithEnv() error = %v", err)
	}

	if got, want := cfg.Model.MaxTokens, 128000; got != want {
		t.Fatalf("model.max_tokens = %d, want %d", got, want)
	}
	if got, want := cfg.Context.Window, 400000; got != want {
		t.Fatalf("context.window = %d, want %d", got, want)
	}
}

func TestLoadWithEnv_AutoTokenLimitsByModelSeries(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		wantModelTokens int
		wantWindow      int
	}{
		{name: "gpt-5.4", model: "gpt-5.4", wantModelTokens: 128000, wantWindow: 1050000},
		{name: "gpt-5.3", model: "gpt-5.3", wantModelTokens: 128000, wantWindow: 400000},
		{name: "claude-4.6-sonnet", model: "claude-sonnet-4.6", wantModelTokens: 64000, wantWindow: 1000000},
		{name: "claude-4.5-haiku", model: "claude-haiku-4.5", wantModelTokens: 64000, wantWindow: 200000},
		{name: "kimi-k2", model: "kimi-k2", wantModelTokens: 32000, wantWindow: 128000},
		{name: "namespaced-kimi-k2.5", model: "moonshotai/kimi-k2.5", wantModelTokens: 32768, wantWindow: 256000},
		{name: "deepseek-reasoner", model: "deepseek-reasoner", wantModelTokens: 64000, wantWindow: 128000},
		{name: "qwen3", model: "qwen3-max", wantModelTokens: 65536, wantWindow: 262144},
		{name: "qwen3.5", model: "qwen3.5-plus", wantModelTokens: 65536, wantWindow: 1000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("MSCLI_MODEL", tt.model)

			cfg, err := LoadWithEnv()
			if err != nil {
				t.Fatalf("LoadWithEnv() error = %v", err)
			}

			if got := cfg.Model.MaxTokens; got != tt.wantModelTokens {
				t.Fatalf("model.max_tokens = %d, want %d", got, tt.wantModelTokens)
			}
			if got := cfg.Context.Window; got != tt.wantWindow {
				t.Fatalf("context.window = %d, want %d", got, tt.wantWindow)
			}
		})
	}
}

func TestLoadWithEnv_EnvOverridesAutoTokenLimits(t *testing.T) {
	clearEnv(t)
	t.Setenv("MSCLI_MODEL", "gpt-5")
	t.Setenv("MSCLI_MAX_TOKENS", "8192")
	t.Setenv("MSCLI_CONTEXT_WINDOW", "16000")

	cfg, err := LoadWithEnv()
	if err != nil {
		t.Fatalf("LoadWithEnv() error = %v", err)
	}

	if got, want := cfg.Model.MaxTokens, 8192; got != want {
		t.Fatalf("model.max_tokens = %d, want %d", got, want)
	}
	if got, want := cfg.Context.Window, 16000; got != want {
		t.Fatalf("context.window = %d, want %d", got, want)
	}
}

func TestApplyModelTokenDefaults_CustomModelProfiles(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.Model = "my-inhouse-model-v2"
	cfg.ModelProfiles = map[string]ModelTokenProfile{
		"my-inhouse-model": {
			ModelMaxTokens: 7777,
			ContextWindow:  55555,
		},
	}

	defaults := DefaultConfig()
	applyModelTokenDefaults(cfg, defaults.Model.MaxTokens, defaults.Context.Window)

	if got, want := cfg.Model.MaxTokens, 7777; got != want {
		t.Fatalf("model.max_tokens = %d, want %d", got, want)
	}
	if got, want := cfg.Context.Window, 55555; got != want {
		t.Fatalf("context.window = %d, want %d", got, want)
	}
}

func TestRefreshModelTokenDefaults_UpdatesAutoValuesOnModelSwitch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.Model = "gpt-4o-mini"

	previousModel := cfg.Model.Model

	cfg.Model.Model = "gpt-5.4"
	RefreshModelTokenDefaults(cfg, previousModel)

	if got, want := cfg.Model.MaxTokens, 128000; got != want {
		t.Fatalf("model.max_tokens = %d, want %d", got, want)
	}
	if got, want := cfg.Context.Window, 1050000; got != want {
		t.Fatalf("context.window = %d, want %d", got, want)
	}
}

func TestRefreshModelTokenDefaults_PreservesExplicitOverridesOnModelSwitch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.Model = "gpt-5"
	applyModelTokenDefaults(cfg, DefaultConfig().Model.MaxTokens, DefaultConfig().Context.Window)
	cfg.Model.MaxTokens = 7777
	cfg.Context.Window = 55555

	cfg.Model.Model = "gpt-5.4"
	RefreshModelTokenDefaults(cfg, "gpt-5")

	if got, want := cfg.Model.MaxTokens, 7777; got != want {
		t.Fatalf("model.max_tokens = %d, want %d", got, want)
	}
	if got, want := cfg.Context.Window, 55555; got != want {
		t.Fatalf("context.window = %d, want %d", got, want)
	}
}
