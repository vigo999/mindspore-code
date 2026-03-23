package llm

import "time"

// ProviderKind identifies a supported LLM provider mode.
type ProviderKind string

const (
	// ProviderOpenAICompletion represents the OpenAI Chat Completions API.
	ProviderOpenAICompletion ProviderKind = "openai-completion"
	// ProviderOpenAIResponses represents the OpenAI Responses API.
	ProviderOpenAIResponses ProviderKind = "openai-responses"
	// ProviderAnthropic represents the Anthropic Messages API.
	ProviderAnthropic ProviderKind = "anthropic"
)

// ResolvedConfig is the fully resolved provider configuration.
type ResolvedConfig struct {
	Kind           ProviderKind
	Model          string
	BaseURL        string
	APIKey         string
	AuthHeaderName string
	Headers        map[string]string
	Timeout        time.Duration
}

// ResolveOptions controls provider config precedence for explicit runtime inputs.
type ResolveOptions struct {
	PreferConfigAPIKey  bool
	PreferConfigBaseURL bool
}
