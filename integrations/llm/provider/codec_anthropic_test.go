package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestAnthropicCodec_EncodeRequestMapsSystemToolUseAndToolResult(t *testing.T) {
	codec := newAnthropicCodec("claude-default")

	req, err := codec.encodeRequest(&llm.CompletionRequest{
		Messages: []llm.Message{
			llm.NewSystemMessage("system prompt one"),
			llm.NewSystemMessage("system prompt two"),
			llm.NewUserMessage("What is the weather?"),
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "toolu_weather",
						Type: "function",
						Function: llm.ToolCallFunc{
							Name:      "lookup_weather",
							Arguments: []byte(`{"city":"Shanghai"}`),
						},
					},
				},
			},
			llm.NewToolMessage("toolu_weather", `{"ok":true}`),
		},
		Tools: []llm.Tool{
			{
				Type: "function",
				Function: llm.ToolFunction{
					Name:        "lookup_weather",
					Description: "Look up the weather",
					Parameters: llm.ToolSchema{
						Type: "object",
						Properties: map[string]llm.Property{
							"city": {Type: "string"},
						},
						Required: []string{"city"},
					},
				},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("encodeRequest() error = %v", err)
	}

	if req.Model != "claude-default" {
		t.Fatalf("model = %q, want %q", req.Model, "claude-default")
	}
	if req.System != "system prompt one\n\nsystem prompt two" {
		t.Fatalf("system = %q", req.System)
	}
	if got := len(req.Messages); got != 3 {
		t.Fatalf("len(messages) = %d, want 3", got)
	}

	if req.Messages[0].Role != "user" {
		t.Fatalf("messages[0].role = %q, want user", req.Messages[0].Role)
	}
	if got := len(req.Messages[0].Content); got != 1 {
		t.Fatalf("len(messages[0].content) = %d, want 1", got)
	}
	if req.Messages[0].Content[0].Type != "text" || req.Messages[0].Content[0].Text != "What is the weather?" {
		t.Fatalf("messages[0].content[0] = %#v", req.Messages[0].Content[0])
	}

	if req.Messages[1].Role != "assistant" {
		t.Fatalf("messages[1].role = %q, want assistant", req.Messages[1].Role)
	}
	if got := len(req.Messages[1].Content); got != 1 {
		t.Fatalf("len(messages[1].content) = %d, want 1", got)
	}
	if req.Messages[1].Content[0].Type != "tool_use" {
		t.Fatalf("messages[1].content[0].type = %q, want tool_use", req.Messages[1].Content[0].Type)
	}
	if req.Messages[1].Content[0].ID != "toolu_weather" {
		t.Fatalf("tool_use id = %q, want %q", req.Messages[1].Content[0].ID, "toolu_weather")
	}
	if req.Messages[1].Content[0].Name != "lookup_weather" {
		t.Fatalf("tool_use name = %q, want %q", req.Messages[1].Content[0].Name, "lookup_weather")
	}
	if string(req.Messages[1].Content[0].Input) != `{"city":"Shanghai"}` {
		t.Fatalf("tool_use input = %q", string(req.Messages[1].Content[0].Input))
	}

	if req.Messages[2].Role != "user" {
		t.Fatalf("messages[2].role = %q, want user", req.Messages[2].Role)
	}
	if got := len(req.Messages[2].Content); got != 1 {
		t.Fatalf("len(messages[2].content) = %d, want 1", got)
	}
	if req.Messages[2].Content[0].Type != "tool_result" {
		t.Fatalf("messages[2].content[0].type = %q, want tool_result", req.Messages[2].Content[0].Type)
	}
	if req.Messages[2].Content[0].ToolUseID != "toolu_weather" {
		t.Fatalf("tool_result tool_use_id = %q, want %q", req.Messages[2].Content[0].ToolUseID, "toolu_weather")
	}
	if req.Messages[2].Content[0].Content != `{"ok":true}` {
		t.Fatalf("tool_result content = %q, want %q", req.Messages[2].Content[0].Content, `{"ok":true}`)
	}

	if got := len(req.Tools); got != 1 {
		t.Fatalf("len(tools) = %d, want 1", got)
	}
	if req.Tools[0].Name != "lookup_weather" {
		t.Fatalf("tool name = %q, want %q", req.Tools[0].Name, "lookup_weather")
	}
	if req.Stream {
		t.Fatal("stream = true, want false")
	}
}

func TestAnthropicCodec_DecodeResponseMapsTextToolUseAndFinishReason(t *testing.T) {
	codec := newAnthropicCodec("claude-default")

	resp := codec.decodeCompletionResponse(anthropicMessagesResponse{
		ID:    "msg_123",
		Model: "claude-3-5-sonnet",
		Content: []anthropicContentBlock{
			{Type: "text", Text: "I can look that up."},
			{
				Type:  "tool_use",
				ID:    "toolu_weather",
				Name:  "lookup_weather",
				Input: json.RawMessage(`{"city":"Shanghai"}`),
			},
		},
		StopReason: "tool_use",
		Usage: anthropicUsage{
			InputTokens:  12,
			OutputTokens: 8,
		},
	})

	if resp == nil {
		t.Fatal("decodeCompletionResponse() returned nil")
	}
	if resp.ID != "msg_123" {
		t.Fatalf("id = %q, want %q", resp.ID, "msg_123")
	}
	if resp.Content != "I can look that up." {
		t.Fatalf("content = %q, want %q", resp.Content, "I can look that up.")
	}
	if resp.FinishReason != llm.FinishToolCalls {
		t.Fatalf("finish_reason = %q, want %q", resp.FinishReason, llm.FinishToolCalls)
	}
	if got := len(resp.ToolCalls); got != 1 {
		t.Fatalf("len(tool_calls) = %d, want 1", got)
	}
	if resp.ToolCalls[0].ID != "toolu_weather" {
		t.Fatalf("tool call id = %q, want %q", resp.ToolCalls[0].ID, "toolu_weather")
	}
	if resp.ToolCalls[0].Function.Name != "lookup_weather" {
		t.Fatalf("tool call name = %q, want %q", resp.ToolCalls[0].Function.Name, "lookup_weather")
	}
	if string(resp.ToolCalls[0].Function.Arguments) != `{"city":"Shanghai"}` {
		t.Fatalf("tool call args = %q", string(resp.ToolCalls[0].Function.Arguments))
	}
	if resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 8 || resp.Usage.TotalTokens != 20 {
		t.Fatalf("usage = %#v, want prompt=12 completion=8 total=20", resp.Usage)
	}
}

func TestAnthropicClient_CompletePostsMessagesAndDecodesResponse(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAPIKey string
	var seenVersion string
	var seenBody anthropicMessagesRequest

	client, err := newAnthropicClient(ResolvedConfig{
		Kind:    ProviderAnthropic,
		Model:   "claude-3-5-haiku",
		BaseURL: "https://example.invalid/v1",
		APIKey:  "test-key",
		Headers: map[string]string{
			"x-api-key":         "test-key",
			"anthropic-version": anthropicVersionHeader,
		},
	}, string(ProviderAnthropic), fakeHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			seenMethod = req.Method
			seenPath = req.URL.String()
			seenAPIKey = req.Header.Get("x-api-key")
			seenVersion = req.Header.Get("anthropic-version")

			if err := json.NewDecoder(req.Body).Decode(&seenBody); err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"id": "msg_456",
					"model": "claude-3-5-haiku",
					"content": [
						{"type":"text","text":"Done."},
						{"type":"tool_use","id":"toolu_time","name":"lookup_time","input":{"zone":"Asia/Shanghai"}}
					],
					"stop_reason": "tool_use",
					"usage": {"input_tokens": 4, "output_tokens": 6}
				}`)),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("newAnthropicClient() error = %v", err)
	}

	resp, err := client.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{
			llm.NewSystemMessage("be precise"),
			llm.NewUserMessage("What time is it?"),
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", seenMethod, http.MethodPost)
	}
	if seenPath != "https://example.invalid/v1/messages" {
		t.Fatalf("path = %q, want %q", seenPath, "https://example.invalid/v1/messages")
	}
	if seenAPIKey != "test-key" {
		t.Fatalf("x-api-key = %q, want %q", seenAPIKey, "test-key")
	}
	if seenVersion != anthropicVersionHeader {
		t.Fatalf("anthropic-version = %q, want %q", seenVersion, anthropicVersionHeader)
	}
	if seenBody.System != "be precise" {
		t.Fatalf("request system = %q, want %q", seenBody.System, "be precise")
	}
	if got := len(seenBody.Messages); got != 1 {
		t.Fatalf("len(request messages) = %d, want 1", got)
	}
	if !strings.EqualFold(resp.Model, "claude-3-5-haiku") {
		t.Fatalf("response model = %q, want %q", resp.Model, "claude-3-5-haiku")
	}
	if resp.Content != "Done." {
		t.Fatalf("response content = %q, want %q", resp.Content, "Done.")
	}
	if resp.FinishReason != llm.FinishToolCalls {
		t.Fatalf("response finish_reason = %q, want %q", resp.FinishReason, llm.FinishToolCalls)
	}
	if got := len(resp.ToolCalls); got != 1 {
		t.Fatalf("len(response tool_calls) = %d, want 1", got)
	}
	if resp.ToolCalls[0].Function.Name != "lookup_time" {
		t.Fatalf("response tool name = %q, want %q", resp.ToolCalls[0].Function.Name, "lookup_time")
	}
}
