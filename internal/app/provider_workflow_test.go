package app

import (
	"testing"
)

func TestBuildModelPickerOptionsIncludesRecentGroup(t *testing.T) {
	catalog := &providerCatalog{
		Providers: []providerCatalogEntry{
			{
				ID:       "openai",
				Label:    "OpenAI",
				Protocol: "openai-chat",
				Models: []modelCatalogEntry{
					{ProviderID: "openai", ID: "gpt-4.1", Label: "GPT-4.1"},
					{ProviderID: "openai", ID: "gpt-4.1-mini", Label: "GPT-4.1 Mini"},
					{ProviderID: "openai", ID: "gpt-4o", Label: "GPT-4o"},
					{ProviderID: "openai", ID: "gpt-4o-mini", Label: "GPT-4o Mini"},
					{ProviderID: "openai", ID: "o3", Label: "o3"},
					{ProviderID: "openai", ID: "o4-mini", Label: "o4-mini"},
				},
			},
		},
	}
	authState := &providerAuthState{
		Providers: map[string]providerAuthEntry{
			"openai": {ProviderID: "openai", APIKey: "sk-test"},
		},
	}
	modelState := &modelSelectionState{
		Recents: []modelRef{
			{ProviderID: "missing", ModelID: "gone"},
			{ProviderID: "openai", ModelID: "gpt-4.1"},
			{ProviderID: "openai", ModelID: "gpt-4.1-mini"},
			{ProviderID: "openai", ModelID: "gpt-4o"},
			{ProviderID: "openai", ModelID: "gpt-4o-mini"},
			{ProviderID: "openai", ModelID: "o3"},
			{ProviderID: "openai", ModelID: "o4-mini"},
		},
	}

	options := buildModelPickerOptions(catalog, authState, modelState, false)

	wantIDs := []string{
		"__header__Recent",
		"openai:gpt-4.1",
		"openai:gpt-4.1-mini",
		"openai:gpt-4o",
		"openai:gpt-4o-mini",
		"openai:o3",
		"__separator__provider:openai",
		"__header__provider:openai",
	}
	if len(options) < len(wantIDs) {
		t.Fatalf("len(options) = %d, want at least %d", len(options), len(wantIDs))
	}
	for i, want := range wantIDs {
		if got := options[i].ID; got != want {
			t.Fatalf("options[%d].ID = %q, want %q", i, got, want)
		}
	}
	if got, want := options[1].Label, "GPT-4.1"; got != want {
		t.Fatalf("options[1].Label = %q, want %q", got, want)
	}
	if got, want := options[1].Desc, "· OpenAI"; got != want {
		t.Fatalf("options[1].Desc = %q, want %q", got, want)
	}
	if got := options[7].Desc; got != "" {
		t.Fatalf("options[7].Desc = %q, want empty provider-group entry desc", got)
	}
}

func TestPartitionConnectProvidersUsesConfiguredPopularOrder(t *testing.T) {
	providers := []providerCatalogEntry{
		{ID: "openrouter", Label: "OpenRouter"},
		{ID: "deepseek", Label: "DeepSeek"},
		{ID: "openai", Label: "OpenAI"},
		{ID: "kimi-for-coding", Label: "Kimi for Coding"},
		{ID: "anthropic", Label: "Anthropic"},
	}

	popular, other := partitionConnectProviders(providers, false)

	wantPopularIDs := []string{"anthropic", "openai", "kimi-for-coding", "deepseek"}
	if len(popular) != len(wantPopularIDs) {
		t.Fatalf("len(popular) = %d, want %d", len(popular), len(wantPopularIDs))
	}
	for i, want := range wantPopularIDs {
		if got := popular[i].ID; got != want {
			t.Fatalf("popular[%d].ID = %q, want %q", i, got, want)
		}
	}
	if len(other) != 1 || other[0].ID != "openrouter" {
		t.Fatalf("other = %#v, want only openrouter", other)
	}
}
