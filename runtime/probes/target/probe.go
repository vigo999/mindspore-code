// Package target defines the interface for remote target readiness probes.
package target

import (
	"context"

	"github.com/vigo999/mindspore-code/internal/train"
	"github.com/vigo999/mindspore-code/runtime/probes"
)

// Probe checks remote training target readiness.
type Probe interface {
	Run(ctx context.Context, target train.TrainTarget) ([]probes.Result, error)
}
