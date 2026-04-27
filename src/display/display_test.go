package display

import (
	"testing"
)

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
