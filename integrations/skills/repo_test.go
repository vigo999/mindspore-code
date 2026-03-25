package skills

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRemoteCommitUsesThreeSecondTimeout(t *testing.T) {
	syncer := NewRepoSync(RepoSyncConfig{
		HomeDir: t.TempDir(),
	})
	syncer.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected remote commit request to include a deadline")
			}

			remaining := time.Until(deadline)
			tolerance := 500 * time.Millisecond
			if remaining < defaultRemoteHEADTimeout-tolerance || remaining > defaultRemoteHEADTimeout+tolerance {
				t.Fatalf("expected remote commit timeout near %v, got %v", defaultRemoteHEADTimeout, remaining)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"object":{"sha":"abc123"}}`)),
			}, nil
		}),
	}

	commit, err := syncer.remoteCommit()
	if err != nil {
		t.Fatalf("remoteCommit returned error: %v", err)
	}
	if commit != "abc123" {
		t.Fatalf("expected commit %q, got %q", "abc123", commit)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
