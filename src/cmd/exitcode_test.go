package cmd

import (
	"context"
	"errors"
	"fmt"
	osexec "os/exec"
	"testing"
)

func TestExitCodeFor(t *testing.T) {
	// A real command that exits 3, to obtain a genuine *exec.ExitError.
	exitErr := osexec.Command("sh", "-c", "exit 3").Run()
	wrapped := fmt.Errorf("command exited with error: %w", exitErr)

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"cancelled", fmt.Errorf("send: %w", context.Canceled), 130},
		{"unknown flag is usage", errors.New("unknown flag: --nope"), 2},
		{"flag needs argument is usage", errors.New("flag needs an argument: --output"), 2},
		{"command exit passthrough", wrapped, 3},
		{"generic", errors.New("backend exploded"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFor(tt.err); got != tt.want {
				t.Errorf("exitCodeFor(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
