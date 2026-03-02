package executor

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type Result struct {
	Output   string
	ExitCode int
	Err      error
}

// BashRunner executes shell commands via "bash -lc".
type BashRunner struct {
	workDir string
	timeout time.Duration
}

func NewBashRunner(workDir string, timeout time.Duration) *BashRunner {
	return &BashRunner{
		workDir: workDir,
		timeout: timeout,
	}
}

func (r *BashRunner) Run(ctx context.Context, command string) Result {
	execCtx := ctx
	var cancel context.CancelFunc
	if r.timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(execCtx, "bash", "-lc", command)
	cmd.Dir = r.workDir

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err == nil {
		return Result{
			Output:   out.String(),
			ExitCode: 0,
		}
	}

	exitCode := -1
	if ee, ok := err.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
	}
	if execCtx.Err() != nil {
		err = execCtx.Err()
	}

	return Result{
		Output:   out.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}
