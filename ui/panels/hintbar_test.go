package panels

import (
	"strings"
	"testing"
)

func TestRenderHintBar_SlashCandidatesVertical(t *testing.T) {
	out := RenderHintBar(80, []string{
		"/model  use <provider>/<model> | list | show",
		"/clear",
	}, 0)

	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected divider + header + vertical items, got %d lines", len(lines))
	}
	if !strings.Contains(out, "slash commands") {
		t.Fatalf("expected slash header, got %q", out)
	}
	if !strings.Contains(out, "› ") {
		t.Fatalf("expected selected marker, got %q", out)
	}
}
