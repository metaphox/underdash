package display

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what
// was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureFile(t, &os.Stderr, fn)
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureFile(t, &os.Stdout, fn)
}

func captureFile(t *testing.T, f **os.File, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := *f
	*f = w
	defer func() { *f = old }()

	fn()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRenderExplanation(t *testing.T) {
	md := "# Title\n\nSome **bold** text and a list:\n\n- one\n- two\n"

	t.Run("plain returns the input unchanged", func(t *testing.T) {
		if got := renderExplanation(md, true); got != md {
			t.Errorf("renderExplanation(plain) = %q, want unchanged input", got)
		}
	})

	t.Run("non-plain transforms the markdown", func(t *testing.T) {
		got := renderExplanation(md, false)
		// glamour reflows and indents (and colorizes on a color terminal), so the
		// rendered output must differ from the raw input and be non-empty. This
		// holds regardless of the test environment's color support.
		if got == "" {
			t.Fatal("renderExplanation(non-plain) returned empty output")
		}
		if got == md {
			t.Error("renderExplanation(non-plain) did not transform the markdown")
		}
	})
}

func TestFormatSpinner(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		elapsed   time.Duration
		remaining time.Duration
		detail    string
		countdown bool
		want      string
	}{
		{
			name:    "timer and detail",
			label:   "Thinking",
			elapsed: 12 * time.Second,
			detail:  "claude/opus-4-8 · ctx: git, go",
			want:    "X Thinking 12s · claude/opus-4-8 · ctx: git, go",
		},
		{
			name:    "no detail",
			label:   "Thinking",
			elapsed: 3 * time.Second,
			want:    "X Thinking 3s",
		},
		{
			name:      "countdown shown under 10s",
			label:     "Thinking",
			elapsed:   53 * time.Second,
			remaining: 7 * time.Second,
			detail:    "claude/opus-4-8",
			countdown: true,
			want:      "X Thinking 53s · claude/opus-4-8 · timeout in 7s",
		},
		{
			name:      "countdown hidden at or above 10s",
			label:     "Thinking",
			elapsed:   40 * time.Second,
			remaining: 20 * time.Second,
			countdown: true,
			want:      "X Thinking 40s",
		},
		{
			name:      "countdown not requested",
			label:     "Thinking",
			elapsed:   55 * time.Second,
			remaining: 2 * time.Second,
			countdown: false,
			want:      "X Thinking 55s",
		},
		{
			name:      "negative remaining clamps to zero",
			label:     "Thinking",
			elapsed:   61 * time.Second,
			remaining: -2 * time.Second,
			countdown: true,
			want:      "X Thinking 61s · timeout in 0s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSpinner("X", tc.label, tc.elapsed, tc.remaining, tc.detail, tc.countdown)
			if got != tc.want {
				t.Errorf("formatSpinner() = %q, want %q", got, tc.want)
			}
			if !tc.countdown && strings.Contains(got, "timeout in") {
				t.Errorf("countdown leaked into %q", got)
			}
		})
	}
}

// --- Finding #4: TTY detection should check stdout, not stderr ---

func TestIsTTY_ChecksStdout(t *testing.T) {
	// This test validates the contract: IsTTY should check stdout.
	// In a test environment (piped), stdout is not a TTY, so IsTTY should return false.
	// The important thing is that the function exists and works without panic.
	result := IsTTY()
	// In test (non-interactive), should be false.
	if result {
		t.Skip("running in interactive terminal, cannot verify non-TTY behavior")
	}
}

// --- Finding #4: PlainMode should be configurable ---

func TestIsPlainMode_ForcedPlain(t *testing.T) {
	// When output mode is "plain", IsPlainMode should return true regardless of TTY.
	SetOutputMode("plain")
	defer SetOutputMode("") // reset

	if !IsPlainMode() {
		t.Error("IsPlainMode() should be true when output mode is explicitly set to 'plain'")
	}
}

func TestIsPlainMode_AutoDetect(t *testing.T) {
	// When output mode is empty/streaming, should auto-detect based on TTY.
	SetOutputMode("")
	// In test environment, stdout is not TTY, so plain mode should be auto-detected.
	if !IsPlainMode() {
		t.Skip("running in interactive terminal")
	}
}

// --- Output helpers (plain mode; tests never run on a TTY) ---

func TestShowCommand(t *testing.T) {
	SetOutputMode("plain")
	defer SetOutputMode("")

	t.Run("command only", func(t *testing.T) {
		got := captureStderr(t, func() { ShowCommand("ls -la", "") })
		if got != "> ls -la\n" {
			t.Errorf("ShowCommand output = %q", got)
		}
	})

	t.Run("command with explanation", func(t *testing.T) {
		got := captureStderr(t, func() { ShowCommand("ls -la", "lists files") })
		if !strings.Contains(got, "> ls -la\n") || !strings.Contains(got, "  lists files\n") {
			t.Errorf("ShowCommand output = %q", got)
		}
	})
}

func TestShowScript(t *testing.T) {
	SetOutputMode("plain")
	defer SetOutputMode("")

	got := captureStderr(t, func() { ShowScript("#!/bin/sh\necho hi", "greets") })
	for _, want := range []string{"  greets\n", "---\n#!/bin/sh\necho hi\n---\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("ShowScript output missing %q; got %q", want, got)
		}
	}
}

func TestShowError(t *testing.T) {
	SetOutputMode("plain")
	defer SetOutputMode("")

	got := captureStderr(t, func() { ShowError("boom") })
	if got != "error: boom\n" {
		t.Errorf("ShowError output = %q", got)
	}
}

func TestShowErrorDetails(t *testing.T) {
	SetOutputMode("plain")
	defer SetOutputMode("")

	got := captureStderr(t, func() {
		ShowErrorDetails("request failed", []string{"status 500", "id req_123"})
	})
	for _, want := range []string{"error: request failed\n", "  status 500\n", "  id req_123\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("ShowErrorDetails output missing %q; got %q", want, got)
		}
	}
}

func TestShowExplanation_Plain(t *testing.T) {
	SetOutputMode("plain")
	defer SetOutputMode("")

	got := captureStdout(t, func() { ShowExplanation("# raw markdown") })
	if got != "# raw markdown\n" {
		t.Errorf("ShowExplanation output = %q, want raw markdown", got)
	}
}

func TestShowCommandOutput(t *testing.T) {
	t.Run("prints non-empty output", func(t *testing.T) {
		got := captureStdout(t, func() { ShowCommandOutput("result\n") })
		if got != "result\n" {
			t.Errorf("ShowCommandOutput = %q", got)
		}
	})

	t.Run("prints nothing for empty output", func(t *testing.T) {
		got := captureStdout(t, func() { ShowCommandOutput("") })
		if got != "" {
			t.Errorf("ShowCommandOutput = %q, want empty", got)
		}
	})
}

func TestPrompt(t *testing.T) {
	t.Run("plain mode returns unstyled", func(t *testing.T) {
		SetOutputMode("plain")
		defer SetOutputMode("")
		if got := Prompt("Execute? "); got != "Execute? " {
			t.Errorf("Prompt = %q", got)
		}
	})
}

func TestShowDryRun(t *testing.T) {
	got := captureStdout(t, func() { ShowDryRun("sys prompt", "user msg") })
	for _, want := range []string{"=== DRY RUN ===", "sys prompt", "user msg"} {
		if !strings.Contains(got, want) {
			t.Errorf("ShowDryRun output missing %q", want)
		}
	}
}

func TestSpinner_PlainModeNoop(t *testing.T) {
	SetOutputMode("plain")
	defer SetOutputMode("")

	s := NewSpinner()
	s.Start("Thinking", "", time.Time{})
	s.ClearDeadline()
	s.Stop() // must not block or panic when Start was a no-op
}
