package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type openAIResponsesClient struct {
	name       string
	baseURL    string
	model      string
	headers    map[string]string
	httpClient HTTPClient
	codec      *openAIResponsesCodec
}

// NewOpenAIResponsesProvider builds the OpenAI Responses API provider implementation.
func NewOpenAIResponsesProvider(cfg ResolvedConfig) (Provider, error) {
	return newOpenAIResponsesClient(cfg, string(ProviderOpenAIResponses), nil)
}

func newOpenAIResponsesClient(cfg ResolvedConfig, name string, httpClient HTTPClient) (*openAIResponsesClient, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("key is required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	headers := copyHeaders(cfg.Headers)
	authHeaderName := strings.TrimSpace(cfg.AuthHeaderName)
	if authHeaderName == "" {
		authHeaderName = "Authorization"
	}
	ensureAuthHeader(headers, authHeaderName, "Bearer "+apiKey)

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = openAIDefaultTimeout
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	if strings.TrimSpace(name) == "" {
		name = string(cfg.Kind)
	}

	return &openAIResponsesClient{
		name:       name,
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      strings.TrimSpace(cfg.Model),
		headers:    headers,
		httpClient: httpClient,
		codec:      newOpenAIResponsesCodec(cfg.Model),
	}, nil
}

func (c *openAIResponsesClient) Name() string {
	return c.name
}

func (c *openAIResponsesClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	body, err := c.codec.encodeRequest(req, false, PreviousResponseIDFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	resp, err := DoJSON(ctx, c.httpClient, http.MethodPost, c.baseURL+"/responses", c.headers, body)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseOpenAIError(resp)
	}

	var decoded openAIResponsesResponseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.codec.decodeCompletionResponse(decoded.ResponseOrSelf()), nil
}

func (c *openAIResponsesClient) CompleteStream(ctx context.Context, req *CompletionRequest) (StreamIterator, error) {
	body, err := c.codec.encodeRequest(req, true, PreviousResponseIDFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	resp, err := DoJSON(ctx, c.httpClient, http.MethodPost, c.baseURL+"/responses", c.headers, body)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseOpenAIError(resp)
	}

	return c.codec.newStreamIterator(resp.Body), nil
}

func (c *openAIResponsesClient) SupportsTools() bool {
	return true
}

func (c *openAIResponsesClient) AvailableModels() []ModelInfo {
	return []ModelInfo{
		{ID: "gpt-4.1", Provider: c.name, MaxTokens: 1047576},
		{ID: "gpt-4.1-mini", Provider: c.name, MaxTokens: 1047576},
		{ID: "gpt-4o", Provider: c.name, MaxTokens: 128000},
		{ID: "gpt-4o-mini", Provider: c.name, MaxTokens: 128000},
	}
}
