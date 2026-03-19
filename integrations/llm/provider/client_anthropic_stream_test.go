package provider

import (
	"io"
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestAnthropicStreamIterator_EmitsTextDeltasAndMappedStopReason(t *testing.T) {
	iter := newAnthropicCodec("claude-default").newStreamIterator(io.NopCloser(strings.NewReader(strings.TrimSpace(`
event: message_start
data: {"type":"message_start","message":{"id":"msg_1","model":"claude-3-5-sonnet","content":[]}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}
	`) + "\n")))

	chunk, err := iter.Next()
	if err != nil {
		t.Fatalf("Next() first error = %v", err)
	}
	if chunk.Content != "Hello" {
		t.Fatalf("first chunk content = %q, want %q", chunk.Content, "Hello")
	}

	chunk, err = iter.Next()
	if err != nil {
		t.Fatalf("Next() second error = %v", err)
	}
	if chunk.Content != " world" {
		t.Fatalf("second chunk content = %q, want %q", chunk.Content, " world")
	}

	chunk, err = iter.Next()
	if err != nil {
		t.Fatalf("Next() third error = %v", err)
	}
	if chunk.FinishReason != llm.FinishStop {
		t.Fatalf("finish_reason = %q, want %q", chunk.FinishReason, llm.FinishStop)
	}
	if chunk.Usage == nil || chunk.Usage.CompletionTokens != 2 {
		t.Fatalf("usage = %#v, want completion_tokens=2", chunk.Usage)
	}

	if _, err := iter.Next(); err != io.EOF {
		t.Fatalf("Next() final error = %v, want %v", err, io.EOF)
	}
}

func TestAnthropicStreamIterator_EmitsToolCallOnlyAfterBlockCompletion(t *testing.T) {
	iter := newAnthropicCodec("claude-default").newStreamIterator(io.NopCloser(strings.NewReader(strings.TrimSpace(`
event: message_start
data: {"type":"message_start","message":{"id":"msg_2","model":"claude-3-5-sonnet","content":[]}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_weather","name":"lookup_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"Shanghai\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":7}}

event: message_stop
data: {"type":"message_stop"}
	`) + "\n")))

	chunk, err := iter.Next()
	if err != nil {
		t.Fatalf("Next() tool chunk error = %v", err)
	}
	if chunk.Content != "" {
		t.Fatalf("tool chunk content = %q, want empty", chunk.Content)
	}
	if got := len(chunk.ToolCalls); got != 1 {
		t.Fatalf("len(tool_calls) = %d, want 1", got)
	}
	if chunk.ToolCalls[0].ID != "toolu_weather" {
		t.Fatalf("tool call id = %q, want %q", chunk.ToolCalls[0].ID, "toolu_weather")
	}
	if chunk.ToolCalls[0].Function.Name != "lookup_weather" {
		t.Fatalf("tool call name = %q, want %q", chunk.ToolCalls[0].Function.Name, "lookup_weather")
	}
	if string(chunk.ToolCalls[0].Function.Arguments) != `{"city":"Shanghai"}` {
		t.Fatalf("tool call args = %q", string(chunk.ToolCalls[0].Function.Arguments))
	}

	chunk, err = iter.Next()
	if err != nil {
		t.Fatalf("Next() finish chunk error = %v", err)
	}
	if chunk.FinishReason != llm.FinishToolCalls {
		t.Fatalf("finish_reason = %q, want %q", chunk.FinishReason, llm.FinishToolCalls)
	}

	if _, err := iter.Next(); err != io.EOF {
		t.Fatalf("Next() final error = %v, want %v", err, io.EOF)
	}
}

func TestAnthropicStreamIterator_MapsMaxTokensStopReason(t *testing.T) {
	iter := newAnthropicCodec("claude-default").newStreamIterator(io.NopCloser(strings.NewReader(strings.TrimSpace(`
event: message_start
data: {"type":"message_start","message":{"id":"msg_3","model":"claude-3-5-sonnet","content":[]}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":9}}

event: message_stop
data: {"type":"message_stop"}
	`) + "\n")))

	chunk, err := iter.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if chunk.FinishReason != llm.FinishLength {
		t.Fatalf("finish_reason = %q, want %q", chunk.FinishReason, llm.FinishLength)
	}
	if chunk.Usage == nil || chunk.Usage.CompletionTokens != 9 {
		t.Fatalf("usage = %#v, want completion_tokens=9", chunk.Usage)
	}
}
