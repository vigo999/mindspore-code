package panels

import (
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/ui/model"
)

func TestRenderMessages_WrapsLongLines(t *testing.T) {
	msgs := []model.Message{
		{Kind: model.MsgAgent, Content: strings.Repeat("a", 200)},
	}

	out := RenderMessages(msgs, "", 40)
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped output to span multiple lines, got %d line", len(lines))
	}
}
