package panels

import (
	"regexp"
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/ui/model"
)

var testANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestRenderMessagesRendersAgentMarkdown(t *testing.T) {
	state := model.NewState("test", ".", "", "demo-model", 4096)
	state = state.WithMessage(model.Message{
		Kind: model.MsgAgent,
		Content: "# Title\n\n- item one\n1. item two\n\n`inline`\n\n```go\nfmt.Println(\"hi\")\n```" +
			"\n\n[docs](https://example.com)",
	})

	rendered := RenderMessages(state, "", 80)
	plain := testANSIPattern.ReplaceAllString(rendered, "")

	if strings.Contains(plain, "# Title") {
		t.Fatalf("expected heading markers to be removed, got:\n%s", plain)
	}
	if strings.Contains(plain, "- item one") {
		t.Fatalf("expected bullet markers to be rendered, got:\n%s", plain)
	}
	if strings.Contains(plain, "```") {
		t.Fatalf("expected code fences to be removed, got:\n%s", plain)
	}
	for _, want := range []string{"Title", "• item one", "1. item two", "inline", "fmt.Println(\"hi\")", "docs (https://example.com)"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in rendered output, got:\n%s", want, plain)
		}
	}
}
