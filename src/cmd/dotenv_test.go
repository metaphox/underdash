package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvFrom_LoadsKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("UNDERDASH_TEST_KEY=from_dotenv\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UNDERDASH_TEST_KEY", "")
	os.Unsetenv("UNDERDASH_TEST_KEY")

	loadDotEnvFrom(path)

	if got := os.Getenv("UNDERDASH_TEST_KEY"); got != "from_dotenv" {
		t.Errorf("expected UNDERDASH_TEST_KEY=from_dotenv, got %q", got)
	}
}

func TestLoadDotEnvFrom_DoesNotOverrideExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("UNDERDASH_TEST_KEY=from_dotenv\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UNDERDASH_TEST_KEY", "from_shell")

	loadDotEnvFrom(path)

	if got := os.Getenv("UNDERDASH_TEST_KEY"); got != "from_shell" {
		t.Errorf("expected shell value to win, got %q", got)
	}
}

func TestLoadDotEnvFrom_MissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env") // does not exist

	// Should not panic or error.
	loadDotEnvFrom(path)
}
