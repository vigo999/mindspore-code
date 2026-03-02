package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Writer writes structured runtime events.
type Writer interface {
	Write(eventType string, payload any) error
}

type jsonlEvent struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   any    `json:"payload"`
}

type JSONLWriter struct {
	path string
	mu   sync.Mutex
}

func NewJSONLWriter(path string) (*JSONLWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return &JSONLWriter{path: path}, nil
}

func (w *JSONLWriter) Write(eventType string, payload any) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(jsonlEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      eventType,
		Payload:   payload,
	})
}

type NoopWriter struct{}

func NewNoopWriter() *NoopWriter {
	return &NoopWriter{}
}

func (w *NoopWriter) Write(string, any) error {
	return nil
}
