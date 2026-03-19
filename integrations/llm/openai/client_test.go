package openai

import "testing"

func TestClientZeroValueSafeMethods(t *testing.T) {
	var client Client

	if got := client.Name(); got != "openai" {
		t.Fatalf("Name() = %q, want %q", got, "openai")
	}
	if !client.SupportsTools() {
		t.Fatal("SupportsTools() = false, want true")
	}

	models := client.AvailableModels()
	if len(models) == 0 {
		t.Fatal("AvailableModels() returned no models")
	}
	if models[0].Provider != "openai" {
		t.Fatalf("models[0].Provider = %q, want %q", models[0].Provider, "openai")
	}
}
