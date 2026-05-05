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

func TestLoadDotEnvFrom_PartialLoadOnBadLines(t *testing.T) {
	// Spec: parse errors are non-fatal; valid lines must still load.
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	contents := "UNDERDASH_GOOD_BEFORE=alpha\n" +
		"THIS LINE HAS NO EQUALS\n" +
		"=value_with_no_key\n" +
		"123BAD=numeric_first_char\n" +
		"# a comment\n" +
		"\n" +
		"UNDERDASH_GOOD_AFTER=\"bravo\"\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"UNDERDASH_GOOD_BEFORE", "UNDERDASH_GOOD_AFTER", "123BAD"} {
		os.Unsetenv(k)
	}

	loadDotEnvFrom(path)

	if got := os.Getenv("UNDERDASH_GOOD_BEFORE"); got != "alpha" {
		t.Errorf("UNDERDASH_GOOD_BEFORE: want %q, got %q", "alpha", got)
	}
	if got := os.Getenv("UNDERDASH_GOOD_AFTER"); got != "bravo" {
		t.Errorf("UNDERDASH_GOOD_AFTER: want %q (quotes stripped), got %q", "bravo", got)
	}
	os.Unsetenv("UNDERDASH_GOOD_BEFORE")
	os.Unsetenv("UNDERDASH_GOOD_AFTER")
}

func TestLoadDotEnvFrom_HandlesExportPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("export UNDERDASH_EXPORTED=ok\n"), 0600); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("UNDERDASH_EXPORTED")

	loadDotEnvFrom(path)

	if got := os.Getenv("UNDERDASH_EXPORTED"); got != "ok" {
		t.Errorf("expected exported var to load, got %q", got)
	}
	os.Unsetenv("UNDERDASH_EXPORTED")
}
