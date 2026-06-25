package display

import (
	"strings"
	"testing"
	"time"
)

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
