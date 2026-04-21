package context

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
)

type compactTestProvider struct {
	response llm.CompletionResponse
	err      error
	calls    int
	lastReq  *llm.CompletionRequest
}

func (p *compactTestProvider) Name() string { return "compact-test" }

func (p *compactTestProvider) Complete(ctx stdctx.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	p.calls++
	p.lastReq = req
	if p.err != nil {
		return nil, p.err
	}
	resp := p.response
	return &resp, nil
}

func (p *compactTestProvider) CompleteStream(ctx stdctx.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	return nil, errors.New("streaming is not implemented")
}

func (p *compactTestProvider) SupportsTools() bool { return true }

func (p *compactTestProvider) AvailableModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "compact-test-model", Provider: p.Name()}}
}

func TestCompactUsesLLMSummaryAndTrajectoryReference(t *testing.T) {
	t.Setenv(envCompactMode, compactModeLLM)
	provider := &compactTestProvider{
		response: llm.CompletionResponse{
			Content: "<analysis>draft notes that should be stripped</analysis><summary>1. Primary Request and Intent:\n   preserve the important work.</summary>",
		},
	}
	mgr := NewManager(ManagerConfig{
		ContextWindow:       1000,
		ReserveTokens:       100,
		CompactionThreshold: 0.9,
		CompactProvider:     provider,
		TrajectoryPath:      "/tmp/mscli/trajectory.jsonl",
	})

	for i := 0; i < 3; i++ {
		if err := mgr.AddMessage(llm.NewUserMessage(strings.Repeat("x", 800))); err != nil {
			t.Fatalf("AddMessage #%d failed: %v", i+1, err)
		}
	}
	before := mgr.TokenUsage().Current

	if err := mgr.Compact(); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if provider.lastReq == nil {
		t.Fatal("provider request was not captured")
	}
	if len(provider.lastReq.Tools) != 0 {
		t.Fatalf("compact request tools = %d, want 0", len(provider.lastReq.Tools))
	}
	if got, want := len(provider.lastReq.Messages), 5; got != want {
		t.Fatalf("compact request messages = %d, want %d", got, want)
	}
	if got := provider.lastReq.Messages[0].Content; got != compactSummarySystemPrompt {
		t.Fatalf("compact system prompt = %q, want %q", got, compactSummarySystemPrompt)
	}
	if got := provider.lastReq.Messages[1].Role; got != "user" {
		t.Fatalf("compact request first history role = %q, want user", got)
	}
	if got := provider.lastReq.Messages[1].Content; !strings.Contains(got, strings.Repeat("x", 800)) {
		t.Fatalf("compact request first history content was not preserved")
	}
	lastPrompt := provider.lastReq.Messages[len(provider.lastReq.Messages)-1]
	if got := lastPrompt.Role; got != "user" {
		t.Fatalf("compact request final prompt role = %q, want user", got)
	}
	if got := lastPrompt.Content; !strings.Contains(got, "Your summary should include the following sections:") {
		t.Fatalf("compact prompt missing summary structure: %q", got)
	}
	if got := lastPrompt.Content; !strings.Contains(got, "Tool calls will be REJECTED and will waste your only turn") {
		t.Fatalf("compact prompt missing tool rejection warning: %q", got)
	}
	if got := lastPrompt.Content; !strings.Contains(got, "REMINDER: Do NOT call any tools.") {
		t.Fatalf("compact prompt missing final reminder: %q", got)
	}
	if strings.Contains(lastPrompt.Content, "Conversation to summarize") {
		t.Fatalf("compact prompt should not embed rendered conversation: %q", lastPrompt.Content)
	}

	msgs := mgr.GetNonSystemMessages()
	if len(msgs) != 1 {
		t.Fatalf("messages after compact = %d, want 1", len(msgs))
	}
	content := msgs[0].Content
	if strings.Contains(content, "draft notes") {
		t.Fatalf("compact summary still contains analysis block: %q", content)
	}
	if !strings.Contains(content, "Summary:\n1. Primary Request and Intent:") {
		t.Fatalf("compact summary content = %q, want formatted summary", content)
	}
	if !strings.Contains(content, "Reference: the full trajectory is available at: /tmp/mscli/trajectory.jsonl") {
		t.Fatalf("compact summary content = %q, want trajectory reference", content)
	}
	if got := mgr.TokenUsage().Current; got >= before {
		t.Fatalf("token usage after compact = %d, want less than before %d", got, before)
	}
}

func TestAddMessageAutoCompactUsesLLMSummary(t *testing.T) {
	t.Setenv(envCompactMode, compactModeLLM)
	provider := &compactTestProvider{
		response: llm.CompletionResponse{Content: "<summary>Current Work:\n   continue the active request.</summary>"},
	}
	var snapshotTrigger CompactTrigger
	var snapshotMessages int
	mgr := NewManager(ManagerConfig{
		ContextWindow:       300,
		ReserveTokens:       30,
		CompactionThreshold: 0.5,
		CompactProvider:     provider,
		TrajectoryPath:      "/tmp/mscli/trajectory.jsonl",
		PreCompactSnapshot: func(snapshot CompactSnapshot) error {
			snapshotTrigger = snapshot.Trigger
			snapshotMessages = len(snapshot.Messages)
			return nil
		},
	})

	if err := mgr.AddMessage(llm.NewUserMessage(strings.Repeat("first ", 80))); err != nil {
		t.Fatalf("AddMessage first failed: %v", err)
	}
	if err := mgr.AddMessage(llm.NewAssistantMessage(strings.Repeat("second ", 80))); err != nil {
		t.Fatalf("AddMessage second failed: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if snapshotTrigger != CompactTriggerAuto {
		t.Fatalf("pre-compact snapshot trigger = %q, want %q", snapshotTrigger, CompactTriggerAuto)
	}
	if snapshotMessages != 2 {
		t.Fatalf("pre-compact snapshot messages = %d, want 2", snapshotMessages)
	}
	msgs := mgr.GetNonSystemMessages()
	if len(msgs) != 1 {
		t.Fatalf("messages after auto compact = %d, want 1", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "continue the active request") {
		t.Fatalf("auto compact summary = %q, want llm summary", msgs[0].Content)
	}
}

func TestAddMessageAutoCompactReinjectsActivatedSkillAfterLLMSummary(t *testing.T) {
	t.Setenv(envCompactMode, compactModeLLM)
	provider := &compactTestProvider{
		response: llm.CompletionResponse{Content: "<summary>Current Work:\n   continue with the loaded skill.</summary>"},
	}
	mgr := NewManager(ManagerConfig{
		ContextWindow:       600,
		ReserveTokens:       60,
		CompactionThreshold: 0.8,
		CompactProvider:     provider,
	})

	toolCallID := "call_load_demo"
	skillCall := loadSkillToolCallMessage(t, toolCallID, "demo-skill")
	skillCall.Content = "assistant text before load_skill should not be re-injected"
	skillContent := "demo skill instructions\n" + strings.Repeat("follow the workflow. ", 20)
	if err := mgr.AddMessage(llm.NewUserMessage(strings.Repeat("prior context ", 60))); err != nil {
		t.Fatalf("AddMessage prior context failed: %v", err)
	}
	if err := mgr.AddMessage(skillCall); err != nil {
		t.Fatalf("AddMessage skill call failed: %v", err)
	}
	if err := mgr.AddMessage(llm.NewToolMessage(toolCallID, skillContent)); err != nil {
		t.Fatalf("AddMessage skill result failed: %v", err)
	}
	if err := mgr.AddMessage(llm.NewAssistantMessage(strings.Repeat("assistant context ", 60))); err != nil {
		t.Fatalf("AddMessage triggering assistant failed: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	msgs := mgr.GetNonSystemMessages()
	if got, want := len(msgs), 2; got != want {
		t.Fatalf("messages after auto compact = %d, want %d: %#v", got, want, msgs)
	}
	if !strings.Contains(msgs[0].Content, "continue with the loaded skill") {
		t.Fatalf("first compacted message = %q, want llm summary", msgs[0].Content)
	}
	if got := msgs[1].Role; got != "user" {
		t.Fatalf("re-injected skill message role = %q, want user", got)
	}
	if len(msgs[1].ToolCalls) != 0 || msgs[1].ToolCallID != "" {
		t.Fatalf("re-injected skill message should not contain tool call state: %#v", msgs[1])
	}
	if !strings.Contains(msgs[1].Content, `[Invoked Skill: demo-skill]`) {
		t.Fatalf("re-injected skill message missing direct marker: %q", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, strings.TrimSpace(skillContent)) {
		t.Fatalf("re-injected skill message missing original skill content: %q", msgs[1].Content)
	}
	if strings.Contains(msgs[1].Content, skillCall.Content) {
		t.Fatalf("re-injected skill message leaked old assistant content: %q", msgs[1].Content)
	}
}

func TestAddMessageAutoCompactPreservesDirectSkillInjectionAfterLLMSummary(t *testing.T) {
	t.Setenv(envCompactMode, compactModeLLM)
	provider := &compactTestProvider{
		response: llm.CompletionResponse{Content: "<summary>Current Work:\n   continue after the next compact.</summary>"},
	}
	mgr := NewManager(ManagerConfig{
		ContextWindow:       900,
		ReserveTokens:       90,
		CompactionThreshold: 0.5,
		CompactProvider:     provider,
	})

	skillMsg := invokedSkillMessage("demo-skill", "direct skill instructions\n"+strings.Repeat("keep using this skill. ", 20))
	if err := mgr.AddMessage(llm.NewUserMessage(strings.Repeat("old context ", 60))); err != nil {
		t.Fatalf("AddMessage old context failed: %v", err)
	}
	if err := mgr.AddMessage(skillMsg); err != nil {
		t.Fatalf("AddMessage skill injection failed: %v", err)
	}
	if err := mgr.AddMessage(llm.NewAssistantMessage(strings.Repeat("assistant context ", 60))); err != nil {
		t.Fatalf("AddMessage triggering assistant failed: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	msgs := mgr.GetNonSystemMessages()
	if got, want := len(msgs), 2; got != want {
		t.Fatalf("messages after auto compact = %d, want %d: %#v", got, want, msgs)
	}
	if !strings.Contains(msgs[0].Content, "continue after the next compact") {
		t.Fatalf("first compacted message = %q, want llm summary", msgs[0].Content)
	}
	if got := msgs[1].Content; got != skillMsg.Content {
		t.Fatalf("direct skill injection after compact = %q, want %q", got, skillMsg.Content)
	}
}

func TestAddUserMessageAutoCompactSummarizesExistingContextThenAppendsUser(t *testing.T) {
	t.Setenv(envCompactMode, compactModeLLM)
	provider := &compactTestProvider{
		response: llm.CompletionResponse{Content: "<summary>Current Work:\n   summarized previous context.</summary>"},
	}
	mgr := NewManager(ManagerConfig{
		ContextWindow:       200,
		ReserveTokens:       20,
		CompactionThreshold: 0.7,
		CompactProvider:     provider,
	})

	oldUser := strings.Repeat("u", 200)
	oldAssistant := strings.Repeat("a", 200)
	activeUser := strings.Repeat("active", 24)
	if err := mgr.AddMessage(llm.NewUserMessage(oldUser)); err != nil {
		t.Fatalf("AddMessage old user failed: %v", err)
	}
	if err := mgr.AddMessage(llm.NewAssistantMessage(oldAssistant)); err != nil {
		t.Fatalf("AddMessage old assistant failed: %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls before triggering user = %d, want 0", provider.calls)
	}

	if err := mgr.AddMessage(llm.NewUserMessage(activeUser)); err != nil {
		t.Fatalf("AddMessage active user failed: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if provider.lastReq == nil {
		t.Fatal("provider request was not captured")
	}
	if got, want := len(provider.lastReq.Messages), 4; got != want {
		t.Fatalf("compact request messages = %d, want %d", got, want)
	}
	if got := provider.lastReq.Messages[1].Content; got != oldUser {
		t.Fatalf("compact request first history content = %q, want old user", got)
	}
	if got := provider.lastReq.Messages[2].Content; got != oldAssistant {
		t.Fatalf("compact request second history content = %q, want old assistant", got)
	}
	for i, msg := range provider.lastReq.Messages {
		if strings.Contains(msg.Content, activeUser) {
			t.Fatalf("compact request message %d included active user content", i)
		}
	}

	msgs := mgr.GetNonSystemMessages()
	if got, want := len(msgs), 2; got != want {
		t.Fatalf("messages after active user auto compact = %d, want %d", got, want)
	}
	if got := msgs[1].Content; got != activeUser {
		t.Fatalf("last message after auto compact = %q, want active user", got)
	}
	if strings.Contains(msgs[0].Content, activeUser) {
		t.Fatalf("summary message should not contain active user content: %q", msgs[0].Content)
	}
}

type dumpingCompactProvider struct {
	calls int
}

func (p *dumpingCompactProvider) Name() string { return "dumping-compact-test" }

func (p *dumpingCompactProvider) Complete(ctx stdctx.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	p.calls++
	resp, err := llm.DoJSON(ctx, fakeCompactHTTPClient{}, http.MethodPost, "https://compact.example/v1/chat/completions", nil, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if _, err := io.ReadAll(resp.Body); err != nil {
		return nil, err
	}
	return &llm.CompletionResponse{Content: "<summary>debug compact response</summary>"}, nil
}

func (p *dumpingCompactProvider) CompleteStream(ctx stdctx.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	return nil, errors.New("streaming is not implemented")
}

func (p *dumpingCompactProvider) SupportsTools() bool { return true }

func (p *dumpingCompactProvider) AvailableModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "dumping-compact-test-model", Provider: p.Name()}}
}

type fakeCompactHTTPClient struct{}

func (fakeCompactHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"content":"ok"}`)),
		Request:    req,
	}, nil
}

func TestCompactLLMSummaryUsesDebugDumper(t *testing.T) {
	t.Setenv(envCompactMode, compactModeLLM)
	debugDir := t.TempDir()
	provider := &dumpingCompactProvider{}
	mgr := NewManager(ManagerConfig{
		ContextWindow:       1000,
		ReserveTokens:       100,
		CompactionThreshold: 0.9,
		CompactProvider:     provider,
		DebugDumper:         llm.NewDebugDumper(debugDir),
	})

	for i := 0; i < 3; i++ {
		if err := mgr.AddMessage(llm.NewUserMessage(strings.Repeat("x", 800))); err != nil {
			t.Fatalf("AddMessage #%d failed: %v", i+1, err)
		}
	}
	if err := mgr.Compact(); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	requests, err := filepath.Glob(filepath.Join(debugDir, "llm_*.request.http"))
	if err != nil {
		t.Fatalf("glob requests: %v", err)
	}
	responses, err := filepath.Glob(filepath.Join(debugDir, "llm_*.response.http"))
	if err != nil {
		t.Fatalf("glob responses: %v", err)
	}
	if len(requests) != 1 || len(responses) != 1 {
		t.Fatalf("debug dumps requests=%d responses=%d, want 1 each", len(requests), len(responses))
	}
	requestDump, err := os.ReadFile(requests[0])
	if err != nil {
		t.Fatalf("read request dump: %v", err)
	}
	if !strings.Contains(string(requestDump), "Primary Request and Intent") {
		t.Fatalf("request dump missing compact prompt:\n%s", string(requestDump))
	}
	responseDump, err := os.ReadFile(responses[0])
	if err != nil {
		t.Fatalf("read response dump: %v", err)
	}
	if !strings.Contains(string(responseDump), `{"content":"ok"}`) {
		t.Fatalf("response dump missing body:\n%s", string(responseDump))
	}
}

func TestCompactFallsBackWhenLLMSummaryFails(t *testing.T) {
	t.Setenv(envCompactMode, compactModeLLM)
	provider := &compactTestProvider{err: errors.New("provider unavailable")}
	mgr := NewManager(ManagerConfig{
		ContextWindow:       1000,
		ReserveTokens:       100,
		CompactionThreshold: 0.9,
		EnableSmartCompact:  false,
		CompactProvider:     provider,
	})

	for i := 0; i < 3; i++ {
		if err := mgr.AddMessage(llm.NewUserMessage(strings.Repeat("x", 500))); err != nil {
			t.Fatalf("AddMessage #%d failed: %v", i+1, err)
		}
	}
	before := mgr.TokenUsage().Current

	if err := mgr.Compact(); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if got := mgr.TokenUsage().Current; got > before/2 {
		t.Fatalf("token usage after fallback compact = %d, want <= %d", got, before/2)
	}
	if got := len(mgr.GetNonSystemMessages()); got >= 3 {
		t.Fatalf("message count after fallback compact = %d, want fewer than 3", got)
	}
}

func TestCompactModePrioritySkipsLLMSummary(t *testing.T) {
	t.Setenv(envCompactMode, compactModePriority)
	provider := &compactTestProvider{
		response: llm.CompletionResponse{Content: "<summary>should not be used</summary>"},
	}
	mgr := NewManager(ManagerConfig{
		ContextWindow:       1000,
		ReserveTokens:       100,
		CompactionThreshold: 0.9,
		EnableSmartCompact:  false,
		CompactProvider:     provider,
	})

	for i := 0; i < 3; i++ {
		if err := mgr.AddMessage(llm.NewUserMessage(strings.Repeat("x", 500))); err != nil {
			t.Fatalf("AddMessage #%d failed: %v", i+1, err)
		}
	}
	if err := mgr.Compact(); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if content := strings.Join(messageContents(mgr.GetNonSystemMessages()), "\n"); strings.Contains(content, "should not be used") {
		t.Fatalf("priority compact used llm summary: %q", content)
	}
}

func messageContents(messages []llm.Message) []string {
	contents := make([]string, len(messages))
	for i, msg := range messages {
		contents[i] = msg.Content
	}
	return contents
}

func loadSkillToolCallMessage(t *testing.T, toolCallID, skillName string) llm.Message {
	t.Helper()
	args, err := json.Marshal(map[string]string{"name": skillName})
	if err != nil {
		t.Fatalf("marshal load_skill args: %v", err)
	}
	return llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{
			{
				ID:   toolCallID,
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "load_skill",
					Arguments: json.RawMessage(args),
				},
			},
		},
	}
}
