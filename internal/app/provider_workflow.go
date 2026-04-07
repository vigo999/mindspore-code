package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

type providerWorkflowState struct {
	catalog    *providerCatalog
	authState  *providerAuthState
	modelState *modelSelectionState
	loggedIn   bool
}

type logicalModelSelectionResult struct {
	ProviderLabel string
}

type providerCatalogLoadMode int

const (
	providerCatalogLoadCacheFirst providerCatalogLoadMode = iota
	providerCatalogLoadBlocking
)

func (a *Application) loadProviderWorkflowState(mode providerCatalogLoadMode) (*providerWorkflowState, error) {
	appCfg, err := loadAppConfig()
	if err != nil {
		return nil, err
	}
	var catalog *providerCatalog
	switch mode {
	case providerCatalogLoadBlocking:
		catalog, err = loadProviderCatalogBlocking(appCfg.ExtraProviders)
	default:
		catalog, err = loadProviderCatalog(nil, appCfg.ExtraProviders)
	}
	if err != nil {
		return nil, err
	}
	authState, err := loadProviderAuthState()
	if err != nil {
		return nil, err
	}
	modelState, err := loadModelSelectionState()
	if err != nil {
		return nil, err
	}
	return &providerWorkflowState{
		catalog:    catalog,
		authState:  authState,
		modelState: modelState,
		loggedIn:   isLoggedIn(),
	}, nil
}

func (a *Application) emitConnectPopup(canEscape bool) {
	state, err := a.loadProviderWorkflowState(providerCatalogLoadCacheFirst)
	if err != nil {
		a.emitToolError("connect", "Failed to load provider catalog: %v", err)
		return
	}

	options := buildConnectProviderOptions(state.catalog, state.loggedIn)

	a.EventCh <- model.Event{
		Type: model.ModelSetupOpen,
		SetupPopup: &model.SetupPopup{
			Title:         "Connect Provider",
			InputLabel:    "API key",
			Screen:        model.SetupScreenPresetPicker,
			PresetOptions: options,
			PresetSelected: (&model.SetupPopup{
				PresetOptions: options,
			}).FirstSelectablePreset(),
			CanEscape:  canEscape,
			BackCloses: true,
		},
	}
}

func (a *Application) emitModelPicker() {
	state, err := a.loadProviderWorkflowState(providerCatalogLoadCacheFirst)
	if err != nil {
		a.emitToolError("model", "Failed to load models: %v", err)
		return
	}
	a.emitModelPickerWithState(state)
}

func (a *Application) emitModelPickerWithState(state *providerWorkflowState) {
	if state == nil {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "No models available. Run /connect to configure a provider.",
		}
		return
	}

	options := buildModelPickerOptions(state.catalog, state.authState, state.modelState, state.loggedIn)
	if len(options) == 0 {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "No models available. Run /connect to configure a provider.",
		}
		return
	}

	selected := 0
	if state.modelState != nil && state.modelState.Active != nil {
		activeID := state.modelState.Active.key()
		for i, opt := range options {
			if opt.ID == activeID {
				selected = i
				break
			}
		}
	} else {
		selected = firstSelectableOptionIndex(options)
	}

	a.EventCh <- model.Event{
		Type: model.ModelPickerOpen,
		Popup: &model.SelectionPopup{
			Title:       "Select model",
			Options:     options,
			Selected:    selected,
			ActionID:    "model_picker",
			SearchQuery: "",
		},
	}
}

func buildConnectProviderOptions(catalog *providerCatalog, loggedIn bool) []model.SelectionOption {
	if catalog == nil {
		return nil
	}
	popularProviders, otherProviders := partitionConnectProviders(catalog.Providers, loggedIn)
	options := make([]model.SelectionOption, 0, len(catalog.Providers)+4)
	if len(popularProviders) > 0 {
		options = append(options, model.SelectionOption{ID: "__header__popular", Label: "Popular", Header: true, Disabled: true})
	}
	for _, provider := range popularProviders {
		options = append(options, connectProviderOption(provider, loggedIn))
	}
	if len(otherProviders) > 0 {
		if len(options) > 0 {
			options = append(options, model.SelectionOption{ID: "__separator__connect", Separator: true, Disabled: true})
		}
		options = append(options, model.SelectionOption{ID: "__header__other", Label: "Other", Header: true, Disabled: true})
	}
	for _, provider := range otherProviders {
		options = append(options, connectProviderOption(provider, loggedIn))
	}
	return options
}

func connectProviderOption(provider providerCatalogEntry, loggedIn bool) model.SelectionOption {
	opt := model.SelectionOption{
		ID:            provider.ID,
		Label:         provider.Label,
		RequiresInput: provider.Protocol != "mindspore-cli-free",
	}
	if provider.Protocol == "mindspore-cli-free" {
		if loggedIn {
			opt.Desc = "(Recommended)"
			opt.RequiresInput = false
		} else {
			opt.Desc = "(require login)"
			opt.Disabled = true
			opt.RequiresInput = false
		}
		return opt
	}
	return opt
}

func buildProviderScopedModelPicker(provider providerCatalogEntry) *model.SelectionPopup {
	options := make([]model.SelectionOption, 0, len(provider.Models))
	for _, m := range provider.Models {
		options = append(options, model.SelectionOption{
			ID:    modelRef{ProviderID: provider.ID, ModelID: m.ID}.key(),
			Label: m.Label,
		})
	}
	return &model.SelectionPopup{
		Title:       "Select model",
		Options:     options,
		Selected:    firstSelectableOptionIndex(options),
		ActionID:    "connect_provider_model_picker:" + provider.ID,
		SearchQuery: "",
	}
}

func firstSelectableOptionIndex(options []model.SelectionOption) int {
	for i, opt := range options {
		if opt.Selectable() {
			return i
		}
	}
	return 0
}

func buildModelPickerOptions(catalog *providerCatalog, authState *providerAuthState, modelState *modelSelectionState, loggedIn bool) []model.SelectionOption {
	if catalog == nil {
		return nil
	}
	usable := usableProviders(catalog, authState, loggedIn)
	options := make([]model.SelectionOption, 0)
	const recentDisplayLimit = 5

	addModelRefs := func(title string, refs []modelRef, limit int, showProvider bool) {
		if len(refs) == 0 {
			return
		}

		group := make([]model.SelectionOption, 0, len(refs)+1)
		group = append(group, model.SelectionOption{ID: "__header__" + title, Label: title, Header: true, Disabled: true})
		count := 0
		for _, ref := range refs {
			provider, ok := findProviderInList(usable, ref.ProviderID)
			if !ok {
				continue
			}
			label := modelLabelForRef(provider, ref.ModelID)
			desc := ""
			if showProvider {
				desc = "· " + provider.Label
			}
			group = append(group, model.SelectionOption{
				ID:    ref.key(),
				Label: label,
				Desc:  desc,
			})
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
		if count == 0 {
			return
		}
		if len(options) > 0 {
			options = append(options, model.SelectionOption{ID: "__separator__" + title, Separator: true, Disabled: true})
		}
		options = append(options, group...)
	}

	if modelState != nil {
		addModelRefs("Recent", modelState.Recents, recentDisplayLimit, true)
		addModelRefs("Favorites", modelState.Favorites, 0, false)
	}

	for _, provider := range usable {
		if len(options) > 0 {
			options = append(options, model.SelectionOption{ID: "__separator__provider:" + provider.ID, Separator: true, Disabled: true})
		}
		options = append(options, model.SelectionOption{ID: "__header__provider:" + provider.ID, Label: provider.Label, Header: true, Disabled: true})
		for _, m := range provider.Models {
			options = append(options, model.SelectionOption{
				ID:    modelRef{ProviderID: provider.ID, ModelID: m.ID}.key(),
				Label: m.Label,
			})
		}
	}
	return options
}

func partitionConnectProviders(providers []providerCatalogEntry, loggedIn bool) ([]providerCatalogEntry, []providerCatalogEntry) {
	popular := make([]providerCatalogEntry, 0, 5)
	other := make([]providerCatalogEntry, 0, len(providers))

	providerByID := make(map[string]providerCatalogEntry, len(providers))
	for _, provider := range providers {
		providerByID[provider.ID] = provider
	}

	if loggedIn {
		if provider, ok := providerByID[mindsporeCLIFreeProviderID]; ok {
			popular = append(popular, provider)
			delete(providerByID, mindsporeCLIFreeProviderID)
		}
	}

	for _, id := range connectPopularProviderIDs {
		if provider, ok := providerByID[id]; ok {
			popular = append(popular, provider)
			delete(providerByID, id)
		}
	}

	if !loggedIn {
		if provider, ok := providerByID[mindsporeCLIFreeProviderID]; ok {
			other = append(other, provider)
			delete(providerByID, mindsporeCLIFreeProviderID)
		}
	}

	rest := make([]providerCatalogEntry, 0, len(providerByID))
	for _, provider := range providerByID {
		rest = append(rest, provider)
	}
	sort.Slice(rest, func(i, j int) bool {
		return strings.ToLower(rest[i].Label) < strings.ToLower(rest[j].Label)
	})
	other = append(other, rest...)
	return popular, other
}

func usableProviders(catalog *providerCatalog, authState *providerAuthState, loggedIn bool) []providerCatalogEntry {
	if catalog == nil {
		return nil
	}
	out := make([]providerCatalogEntry, 0, len(catalog.Providers))
	for _, provider := range catalog.Providers {
		if provider.Protocol == "mindspore-cli-free" {
			if loggedIn {
				out = append(out, provider)
			}
			continue
		}
		if authState != nil {
			if entry, ok := authState.Providers[provider.ID]; ok && strings.TrimSpace(entry.APIKey) != "" {
				out = append(out, provider)
			}
		}
	}
	return out
}

func findProviderInList(providers []providerCatalogEntry, id string) (providerCatalogEntry, bool) {
	for _, provider := range providers {
		if provider.ID == normalizedProviderID(id) {
			return provider, true
		}
	}
	return providerCatalogEntry{}, false
}

func modelLabelForRef(provider providerCatalogEntry, modelID string) string {
	for _, m := range provider.Models {
		if m.ID == strings.TrimSpace(modelID) {
			return m.Label
		}
	}
	return strings.TrimSpace(modelID)
}

func providerDisplayLabel(catalog *providerCatalog, providerID string) string {
	if catalog != nil {
		if provider, ok := catalog.Provider(providerID); ok && strings.TrimSpace(provider.Label) != "" {
			return strings.TrimSpace(provider.Label)
		}
	}
	return runtimeProviderDisplayLabel(providerID)
}

func runtimeProviderDisplayLabel(providerID string) string {
	switch normalizedProviderID(providerID) {
	case "openai", "openai-completion":
		return "OpenAI"
	case "openai-responses":
		return "OpenAI Responses"
	case "anthropic":
		return "Anthropic"
	case mindsporeCLIFreeProviderID:
		return "MindSpore CLI Free"
	default:
		if label := canonicalProviderLabel(providerID, ""); label != "" {
			return label
		}
		return strings.TrimSpace(providerID)
	}
}

func (a *Application) connectProvider(providerID, apiKey string) error {
	state, err := a.loadProviderWorkflowState(providerCatalogLoadCacheFirst)
	if err != nil {
		return err
	}
	provider, ok := state.catalog.Provider(providerID)
	if !ok {
		state, err = a.loadProviderWorkflowState(providerCatalogLoadBlocking)
		if err != nil {
			return err
		}
		provider, ok = state.catalog.Provider(providerID)
	}
	if !ok {
		return fmt.Errorf("unknown provider %q", providerID)
	}

	if provider.Protocol == "mindspore-cli-free" {
		if !state.loggedIn {
			return fmt.Errorf("MindSpore CLI Free requires /login first")
		}
		a.EventCh <- model.Event{Type: model.ModelSetupClose}
		a.emitModelPickerWithState(state)
		return nil
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("provider %s requires api key", provider.Label)
	}
	state.authState.Providers[provider.ID] = providerAuthEntry{
		ProviderID: provider.ID,
		APIKey:     apiKey,
	}
	if err := saveProviderAuthState(state.authState); err != nil {
		return err
	}

	a.EventCh <- model.Event{Type: model.ModelSetupClose}
	a.EventCh <- model.Event{
		Type:  model.ModelPickerOpen,
		Popup: buildProviderScopedModelPicker(provider),
	}
	return nil
}

func (a *Application) activateLogicalModelSelection(providerID, modelID string) (logicalModelSelectionResult, error) {
	state, err := a.loadProviderWorkflowState(providerCatalogLoadCacheFirst)
	if err != nil {
		return logicalModelSelectionResult{}, err
	}
	if _, ok := state.catalog.Provider(providerID); !ok {
		state, err = a.loadProviderWorkflowState(providerCatalogLoadBlocking)
		if err != nil {
			return logicalModelSelectionResult{}, err
		}
	}

	resolved, presetID, err := resolveRuntimeSelection(state.catalog, state.authState, modelRef{
		ProviderID: providerID,
		ModelID:    modelID,
	})
	if err != nil {
		return logicalModelSelectionResult{}, err
	}

	a.restoreModelConfigFromPreset()
	a.Config.Model.URL = resolved.URL
	if err := a.SetProvider(resolved.Provider, resolved.Model, resolved.Key); err != nil {
		return logicalModelSelectionResult{}, err
	}

	a.activeModelPresetID = presetID
	if a.activeModelPresetID == "" {
		a.activeModelPresetID = ""
	}

	if state.modelState == nil {
		state.modelState = emptyModelSelectionState()
	}
	selected := modelRef{ProviderID: providerID, ModelID: modelID}.normalized()
	state.modelState.Active = &selected
	state.modelState.Recents = append([]modelRef{selected}, state.modelState.Recents...)
	state.modelState.normalize()
	if err := saveModelSelectionState(state.modelState); err != nil {
		return logicalModelSelectionResult{}, err
	}
	return logicalModelSelectionResult{
		ProviderLabel: providerDisplayLabel(state.catalog, providerID),
	}, nil
}
