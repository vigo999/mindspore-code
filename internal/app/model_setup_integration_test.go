package app

import (
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/configs"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestModelCommand_OpensModelPickerWithState(t *testing.T) {
	eventCh := make(chan model.Event, 64)
	t.Setenv("HOME", t.TempDir())
	if err := saveCredentials(&credentials{
		ServerURL: "https://mscli.dev",
		Token:     "user-token",
		User:      "alice",
		Role:      "user",
	}); err != nil {
		t.Fatalf("saveCredentials() error = %v", err)
	}
	if err := saveModelSelectionState(&modelSelectionState{
		Active: &modelRef{
			ProviderID: mindsporeCLIFreeProviderID,
			ModelID:    "kimi-k2.5",
		},
	}); err != nil {
		t.Fatalf("saveModelSelectionState() error = %v", err)
	}
	app := &Application{
		EventCh:             eventCh,
		Config:              configs.DefaultConfig(),
		activeModelPresetID: "kimi-k2.5-free",
		llmReady:            true,
	}

	app.cmdModel(nil)

	var popup *model.SelectionPopup
	for len(eventCh) > 0 {
		ev := <-eventCh
		if ev.Type == model.ModelPickerOpen {
			popup = ev.Popup
		}
	}
	if popup == nil {
		t.Fatal("expected model picker to open")
	}
	if got, want := popup.ActionID, "model_picker"; got != want {
		t.Errorf("popup.ActionID = %q, want %q", got, want)
	}
	if len(popup.Options) == 0 {
		t.Fatal("expected model options")
	}
	foundFree := false
	for _, opt := range popup.Options {
		if strings.Contains(opt.ID, "mindspore-cli-free:kimi-k2.5") {
			foundFree = true
			break
		}
	}
	if !foundFree {
		t.Errorf("expected free model option, got %#v", popup.Options)
	}
}
