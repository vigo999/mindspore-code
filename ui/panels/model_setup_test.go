package panels

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

var selectionPopupANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestRenderSetupPopupModeSelect(t *testing.T) {
	popup := &model.SetupPopup{
		Screen:       model.SetupScreenModeSelect,
		ModeSelected: 0,
		CanEscape:    true,
	}
	result := RenderSetupPopup(popup)
	if !strings.Contains(result, "mscli-provided") {
		t.Error("expected 'mscli-provided' in output")
	}
	if !strings.Contains(result, "your own model") {
		t.Error("expected 'your own model' in output")
	}
}

func TestRenderSetupPopupPresetPicker(t *testing.T) {
	popup := &model.SetupPopup{
		Screen: model.SetupScreenPresetPicker,
		PresetOptions: []model.SelectionOption{
			{ID: "kimi-k2.5-free", Label: "kimi-k2.5 [free]"},
			{ID: "deepseek-v3", Label: "deepseek-v3"},
			{ID: "glm-4.7", Label: "glm-4.7 (coming soon)", Disabled: true},
		},
		PresetSelected: 0,
		CanEscape:      true,
	}
	result := RenderSetupPopup(popup)
	if !strings.Contains(result, "kimi-k2.5 [free]") {
		t.Error("expected active preset label in output")
	}
	if !strings.Contains(result, "deepseek-v3") {
		t.Error("expected deepseek preset in output")
	}
	if !strings.Contains(result, "glm-4.7 (coming soon)") {
		t.Error("expected coming-soon preset in output")
	}
}

func TestRenderSetupPopupTokenInput(t *testing.T) {
	popup := &model.SetupPopup{
		Screen:     model.SetupScreenTokenInput,
		TokenValue: "sk-abc",
		CanEscape:  true,
		SelectedPreset: model.SelectionOption{
			ID:    "kimi-k2.5-free",
			Label: "kimi-k2.5 [free]",
		},
	}
	result := RenderSetupPopup(popup)
	if !strings.Contains(result, "Token") {
		t.Error("expected 'Token' label in output")
	}
}

func TestRenderSetupPopupEnvInfo(t *testing.T) {
	popup := &model.SetupPopup{
		Screen:    model.SetupScreenEnvInfo,
		CanEscape: true,
	}
	result := RenderSetupPopup(popup)
	if !strings.Contains(result, "MSCLI_PROVIDER") {
		t.Error("expected env var example in output")
	}
	if !strings.Contains(result, "MSCLI_API_KEY") {
		t.Error("expected MSCLI_API_KEY in output")
	}
}

func TestRenderSetupPopupPresetPickerUsesWindowedOptions(t *testing.T) {
	options := make([]model.SelectionOption, 0, 20)
	for i := 0; i < 20; i++ {
		options = append(options, model.SelectionOption{
			ID:    "provider-" + strings.Repeat("x", i%3+1),
			Label: fmt.Sprintf("provider option item-%02d", i),
		})
	}
	popup := &model.SetupPopup{
		Screen:         model.SetupScreenPresetPicker,
		PresetOptions:  options,
		PresetSelected: 15,
		CanEscape:      true,
	}
	result := RenderSetupPopup(popup)
	if !strings.Contains(result, "provider option item-15") {
		t.Fatalf("expected selected window item in output, got:\n%s", result)
	}
	if strings.Contains(result, "provider option item-00") {
		t.Fatalf("expected earliest option to be clipped from output, got:\n%s", result)
	}
}

func TestRenderSelectionPopupUsesWindowedOptions(t *testing.T) {
	options := make([]model.SelectionOption, 0, 20)
	for i := 0; i < 20; i++ {
		options = append(options, model.SelectionOption{
			ID:    "model-id",
			Label: fmt.Sprintf("model option item-%02d", i),
		})
	}
	result := RenderSelectionPopup(&model.SelectionPopup{
		Title:    "Select model",
		Options:  options,
		Selected: 14,
	})
	if !strings.Contains(result, "model option item-14") {
		t.Fatalf("expected selected window item in output, got:\n%s", result)
	}
	if strings.Contains(result, "model option item-00") {
		t.Fatalf("expected earliest option to be clipped from output, got:\n%s", result)
	}
}

func TestRenderSelectionPopupIncludesConnectProviderShortcut(t *testing.T) {
	result := RenderSelectionPopup(&model.SelectionPopup{
		Title:    "Select model",
		ActionID: "model_picker",
		Options: []model.SelectionOption{
			{ID: "__header__Recent", Label: "Recent", Header: true, Disabled: true},
			{ID: "mindspore-cli-free:kimi-k2.5", Label: "Kimi K2.5"},
		},
		Selected: 1,
	})
	if !strings.Contains(result, "Connect Provider ctrl+a") {
		t.Fatalf("expected connect provider shortcut in output, got:\n%s", result)
	}
}

func TestRenderSelectionPopupModelPickerFooterOnlyShowsConnectShortcut(t *testing.T) {
	result := RenderSelectionPopup(&model.SelectionPopup{
		Title:    "Select model",
		ActionID: "model_picker",
		Options: []model.SelectionOption{
			{ID: "__header__Recent", Label: "Recent", Header: true, Disabled: true},
			{ID: "mindspore-cli-free:kimi-k2.5", Label: "Kimi K2.5"},
		},
		Selected: 1,
	})
	if strings.Contains(result, "↑/↓ select · enter confirm · esc cancel") {
		t.Fatalf("expected model picker footer to omit nav hint, got:\n%s", result)
	}
	if strings.Contains(result, "↑/↓ scroll · enter confirm · esc cancel") {
		t.Fatalf("expected model picker footer to omit scroll hint, got:\n%s", result)
	}
	if !strings.Contains(result, "Connect Provider ctrl+a") {
		t.Fatalf("expected model picker footer to keep connect shortcut, got:\n%s", result)
	}
}

func TestRenderSelectionPopupModelPickerShowsProviderAfterRecentModel(t *testing.T) {
	result := RenderSelectionPopup(&model.SelectionPopup{
		Title:    "Select model",
		ActionID: "model_picker",
		Options: []model.SelectionOption{
			{ID: "__header__Recent", Label: "Recent", Header: true, Disabled: true},
			{ID: "openai:gpt-4.1", Label: "GPT-4.1", Desc: "· OpenAI"},
		},
		Selected: 1,
	})

	plain := selectionPopupANSIPattern.ReplaceAllString(result, "")
	if !strings.Contains(plain, "GPT-4.1 · OpenAI") {
		t.Fatalf("expected recent model to render provider suffix, got:\n%s", plain)
	}
}
