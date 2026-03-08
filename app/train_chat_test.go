package main

import (
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/ui/model"
)

func TestExtractTrainChatActions(t *testing.T) {
	message := "Updated the training script and validated the config.\n" + trainChatRetryMarker + "\n"
	cleaned, retry, stop := extractTrainChatActions(message)

	if !retry {
		t.Fatalf("expected retry marker to be detected")
	}
	if stop {
		t.Fatalf("did not expect stop marker")
	}
	if cleaned != "Updated the training script and validated the config." {
		t.Fatalf("unexpected cleaned message: %q", cleaned)
	}
}

func TestTrainChatContextSummaryIncludesLatestMetrics(t *testing.T) {
	app := &Application{}

	app.recordTrainUpdate(model.TrainUpdate{
		Kind:  model.TrainUpdateOpen,
		RunID: "run-123",
		Hosts: []string{"gpuA"},
	})
	app.recordTrainUpdate(model.TrainUpdate{
		Kind:   model.TrainUpdateStage,
		Stage:  "dashboard",
		Status: string(model.TrainStageRunning),
	})
	app.recordTrainUpdate(model.TrainUpdate{
		Kind:          model.TrainUpdateMetric,
		Host:          "gpuA",
		Stage:         "dashboard",
		Step:          80,
		TotalStep:     120,
		Loss:          0.4321,
		Throughput:    512.7,
		GradNorm:      0.88,
		Model:         "qwen2.5-7b",
		HasStep:       true,
		HasTotalStep:  true,
		HasLoss:       true,
		HasThroughput: true,
		HasGradNorm:   true,
		HasModel:      true,
	})
	app.recordTrainUpdate(model.TrainUpdate{
		Kind:    model.TrainUpdateLog,
		Host:    "gpuA",
		Message: "loss stabilized after lr decay",
	})

	summary := app.trainChatContextSummary()
	for _, want := range []string{
		"run_id: run-123",
		"status: running",
		"current_stage: dashboard",
		"gpuA",
		"step=80/120",
		"loss=0.43210",
		"loss stabilized after lr decay",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got %q", want, summary)
		}
	}
}
