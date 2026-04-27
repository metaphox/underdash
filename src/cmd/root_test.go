package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Finding #7: world-readable config warning ---

func TestCheckConfigPermissions_WorldReadable(t *testing.T) {
	// Create a temp config file with api_key and world-readable permissions.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("backends:\n  claude:\n    api_key: sk-secret-key\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	warning := checkConfigPermissions(cfgPath)
	if warning == "" {
		t.Error("expected a warning for world-readable config file containing api_key")
	}
}

func TestCheckConfigPermissions_RestrictedPerms(t *testing.T) {
	// Create a temp config with restricted permissions.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("backends:\n  claude:\n    api_key: sk-secret-key\n")
	if err := os.WriteFile(cfgPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	warning := checkConfigPermissions(cfgPath)
	if warning != "" {
		t.Errorf("unexpected warning for restricted config: %s", warning)
	}
}

func TestCheckConfigPermissions_NoAPIKey(t *testing.T) {
	// A world-readable config with no api_key should not warn.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("default_backend: claude\nbackends:\n  claude:\n    env_key: ANTHROPIC_API_KEY\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	warning := checkConfigPermissions(cfgPath)
	if warning != "" {
		t.Errorf("unexpected warning for config without api_key: %s", warning)
	}
}

// --- Finding #2: --dry-run should show generated command, not prompt internals ---
// (Integration-level test for the flag behavior — tests the contract)

func TestDryRunFlag_Defined(t *testing.T) {
	// Verify --dry-run and --no-exec flags both exist.
	f := rootCmd.Flags().Lookup("dry-run")
	if f == nil {
		t.Error("--dry-run flag not defined")
	}
	f = rootCmd.Flags().Lookup("no-exec")
	if f == nil {
		t.Error("--no-exec flag not defined")
	}
}
