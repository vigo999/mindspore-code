package app

import (
	"testing"

	"github.com/vigo999/mindspore-code/ui/model"
)

func TestSyncSharedSkillsEmitsSkillsNote(t *testing.T) {
	app := &Application{
		EventCh:       make(chan model.Event, 4),
		skillsHomeDir: "", // empty → syncSharedSkills returns early
	}

	// Should not panic on empty skillsHomeDir.
	app.syncSharedSkills()

	select {
	case ev := <-app.EventCh:
		t.Fatalf("unexpected event on empty skillsHomeDir: %+v", ev)
	default:
		// Expected: no events emitted.
	}
}

func TestEmitSkillsNote(t *testing.T) {
	app := &Application{
		EventCh: make(chan model.Event, 2),
	}

	app.emitSkillsNote("mindspore-skills v1.0.0 ready")

	select {
	case ev := <-app.EventCh:
		if ev.Type != model.SkillsNoteUpdate {
			t.Fatalf("event type = %s, want %s", ev.Type, model.SkillsNoteUpdate)
		}
		if ev.Message != "mindspore-skills v1.0.0 ready" {
			t.Fatalf("event message = %q, want %q", ev.Message, "mindspore-skills v1.0.0 ready")
		}
	default:
		t.Fatal("expected skills note event")
	}
}

func TestEmitSkillsNoteIgnoresEmpty(t *testing.T) {
	app := &Application{
		EventCh: make(chan model.Event, 2),
	}

	app.emitSkillsNote("")
	app.emitSkillsNote("   ")

	select {
	case ev := <-app.EventCh:
		t.Fatalf("unexpected event: %+v", ev)
	default:
		// Expected: no events.
	}
}
