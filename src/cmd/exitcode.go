package cmd

import (
	"context"
	"errors"
	osexec "os/exec"
	"strings"
)

// Stable exit codes — part of the tool's contract (see SPEC §Error Surfaces).
const (
	exitOK        = 0
	exitError     = 1
	exitUsage     = 2
	exitCancelled = 130
)

// exitCodeFor maps an error to the process exit code.
func exitCodeFor(err error) int {
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, context.Canceled):
		return exitCancelled
	case isExitError(err):
		var ee *osexec.ExitError
		errors.As(err, &ee)
		return ee.ExitCode()
	case isUsageError(err):
		return exitUsage
	default:
		return exitError
	}
}

// isExitError reports whether err wraps a failed command's *exec.ExitError.
func isExitError(err error) bool {
	var ee *osexec.ExitError
	return errors.As(err, &ee)
}

// isUsageError reports whether err is a flag/usage error (exit code 2).
func isUsageError(err error) bool {
	if _, ok := flagErrorHint(err); ok {
		return true
	}
	msg := err.Error()
	for _, p := range []string{
		"unknown flag",
		"unknown shorthand flag",
		"invalid argument",
		"flag needs an argument",
	} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}
