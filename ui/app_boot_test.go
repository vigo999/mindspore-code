package ui

import "testing"

func TestInitStartsDeferredChecksDuringBoot(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	if !app.bootActive {
		t.Fatal("expected boot splash to be enabled by default")
	}

	_ = app.Init()

	select {
	case got := <-userCh:
		if got != bootReadyToken {
			t.Fatalf("boot token = %q, want %q", got, bootReadyToken)
		}
	default:
		t.Fatal("expected Init to send bootReadyToken")
	}
}

func TestNewReplaySkipsBootSplash(t *testing.T) {
	app := NewReplay(nil, nil, "test", ".", "", "demo-model", 4096)
	if app.bootActive {
		t.Fatal("expected replay app to skip boot splash")
	}
}
