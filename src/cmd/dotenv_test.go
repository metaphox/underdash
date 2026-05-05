package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnvFrom_LoadsKeys(t *testing.T) {
	key := testEnvKey(t, "loads")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(key+"=from_dotenv\n"), 0600); err != nil {
		t.Fatal(err)
	}

	loadDotEnvFrom(path)

	if got := os.Getenv(key); got != "from_dotenv" {
		t.Errorf("expected %s=from_dotenv, got %q", key, got)
	}
}

func TestLoadDotEnvFrom_DoesNotOverrideExisting(t *testing.T) {
	key := testEnvKey(t, "override")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(key+"=from_dotenv\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(key, "from_shell")

	loadDotEnvFrom(path)

	if got := os.Getenv(key); got != "from_shell" {
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
	keyBefore := testEnvKey(t, "good_before")
	keyAfter := testEnvKey(t, "good_after")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	contents := keyBefore + "=alpha\n" +
		"THIS LINE HAS NO EQUALS\n" +
		"=value_with_no_key\n" +
		"123BAD=numeric_first_char\n" +
		"# a comment\n" +
		"\n" +
		keyAfter + "=\"bravo\"\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	loadDotEnvFrom(path)

	if got := os.Getenv(keyBefore); got != "alpha" {
		t.Errorf("%s: want %q, got %q", keyBefore, "alpha", got)
	}
	if got := os.Getenv(keyAfter); got != "bravo" {
		t.Errorf("%s: want %q (quotes stripped), got %q", keyAfter, "bravo", got)
	}
}

func TestLoadDotEnvFrom_HandlesExportPrefix(t *testing.T) {
	key := testEnvKey(t, "exported")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("export "+key+"=ok\n"), 0600); err != nil {
		t.Fatal(err)
	}

	loadDotEnvFrom(path)

	if got := os.Getenv(key); got != "ok" {
		t.Errorf("expected exported var to load, got %q", got)
	}
}

func testEnvKey(t *testing.T, suffix string) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	name = strings.ToUpper(name)
	return "UNDERDASH_" + name + "_" + strings.ToUpper(suffix)
}
