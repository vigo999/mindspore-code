package app

import "testing"

func TestListBuiltinModelPresetsIncludesComingSoon(t *testing.T) {
	presets := listBuiltinModelPresets()
	if len(presets) < 4 {
		t.Fatalf("expected at least 4 presets (2 active + 2 coming soon), got %d", len(presets))
	}

	active := 0
	comingSoon := 0
	for _, p := range presets {
		if p.ComingSoon {
			comingSoon++
		} else {
			active++
		}
	}
	if active < 2 {
		t.Errorf("expected at least 2 active presets, got %d", active)
	}
	if comingSoon < 2 {
		t.Errorf("expected at least 2 coming-soon presets, got %d", comingSoon)
	}
}

func TestResolveBuiltinModelPresetSkipsComingSoon(t *testing.T) {
	if _, ok := resolveBuiltinModelPreset("glm-4.7"); ok {
		t.Error("expected coming-soon preset to not resolve")
	}
	if _, ok := resolveBuiltinModelPreset("kimi-k2.5-free"); !ok {
		t.Error("expected active preset to resolve")
	}
	if _, ok := resolveBuiltinModelPreset("deepseek-v3"); !ok {
		t.Error("expected deepseek preset to resolve")
	}
}
