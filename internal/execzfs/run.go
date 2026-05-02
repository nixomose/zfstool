package execzfs

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

var (
	// ZpoolCmd and ZfsCmd can be overridden for tests.
	ZpoolCmd = "zpool"
	ZfsCmd   = "zfs"
)

// ErrNotFound indicates zfs/zpool binary missing or permission denied.
var ErrNotFound = errors.New("zfs or zpool command failed (not installed or no permission)")

// RunZpool executes zpool with args; ctx controls timeout.
func RunZpool(ctx context.Context, args ...string) ([]byte, error) {
	return run(ctx, ZpoolCmd, args...)
}

// RunZfs executes zfs with args.
func RunZfs(ctx context.Context, args ...string) ([]byte, error) {
	return run(ctx, ZfsCmd, args...)
}

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if strings.Contains(stderr.String(), "command not found") || errors.Is(err, exec.ErrNotFound) {
			return nil, ErrNotFound
		}
		return out.Bytes(), &RunError{
			Cmd:    name,
			Args:   args,
			Stderr: stderr.String(),
			Err:    err,
		}
	}
	return out.Bytes(), nil
}

// RunError wraps a failed command.
type RunError struct {
	Cmd    string
	Args   []string
	Stderr string
	Err    error
}

func (e *RunError) Error() string {
	return e.Cmd + " " + strings.Join(e.Args, " ") + ": " + e.Err.Error() + ": " + e.Stderr
}

func (e *RunError) Unwrap() error { return e.Err }
