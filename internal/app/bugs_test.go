package app

import (
	"strings"
	"testing"
	"time"

	issuepkg "github.com/mindspore-lab/mindspore-cli/internal/issues"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

type fakeDockIssueStore struct {
	fakeAppIssueStore
	dock *issuepkg.DockData
}

func (f *fakeDockIssueStore) DockSummary() (*issuepkg.DockData, error) {
	return f.dock, nil
}

func TestCmdDockStreamsRawANSI(t *testing.T) {
	store := &fakeDockIssueStore{
		dock: &issuepkg.DockData{
			OpenCount:   21,
			OnlineCount: 5,
			ReadyIssues: []issuepkg.Issue{
				{ID: 43, Key: "ISSUE-43", Title: "abnormal display for contents of /project", Kind: issuepkg.KindBug, Status: "ready"},
			},
			RecentFeed: []issuepkg.Activity{
				{
					Actor:     "xinwen",
					Text:      "reported issue: abnormal display for contents of /now",
					CreatedAt: time.Date(2026, 4, 3, 3, 22, 0, 0, time.FixedZone("CST", 8*3600)),
				},
			},
		},
	}

	app := &Application{
		EventCh:      make(chan model.Event, 4),
		issueService: issuepkg.NewService(store),
	}

	app.cmdNow()

	ev := drainUntilEventType(t, app, model.AgentReply)
	if !ev.RawANSI {
		t.Fatal("expected /now output to be marked RawANSI")
	}
	for _, want := range []string{
		"DASHBOARD",
		"open issues",
		"online (24h)",
		"abnormal display for contents of /project",
		"reported issue: abnormal display for contents of /now",
	} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("expected /now output to contain %q, got:\n%s", want, ev.Message)
		}
	}
}
