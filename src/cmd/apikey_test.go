package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadAPIKeyFileRawKey(t *testing.T) {
	dir := t.TempDir()

	t.Run("bare key", func(t *testing.T) {
		path := writeFile(t, dir, "raw", "sk-ant-abc123")
		got, err := readAPIKeyFile(path, "ANTHROPIC_API_KEY", "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-ant-abc123" {
			t.Errorf("got %q, want sk-ant-abc123", got)
		}
	})

	t.Run("surrounding whitespace is trimmed", func(t *testing.T) {
		path := writeFile(t, dir, "raw-ws", "\n  sk-ant-xyz \n")
		got, err := readAPIKeyFile(path, "ANTHROPIC_API_KEY", "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-ant-xyz" {
			t.Errorf("got %q, want sk-ant-xyz", got)
		}
	})

	t.Run("key with dashes is not mistaken for a pair", func(t *testing.T) {
		// A raw key whose left side has a dash is not a valid identifier, so it
		// must not be parsed as NAME=value.
		path := writeFile(t, dir, "dashy", "sk-ant-a=b-c")
		got, err := readAPIKeyFile(path, "ANTHROPIC_API_KEY", "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-ant-a=b-c" {
			t.Errorf("got %q, want sk-ant-a=b-c", got)
		}
	})
}

func TestReadAPIKeyFilePairs(t *testing.T) {
	dir := t.TempDir()
	content := "# shared key file\n" +
		"ANTHROPIC_API_KEY=sk-ant-multi\n" +
		"export OPENAI_API_KEY=\"sk-openai-multi\"\n"

	t.Run("selects the claude entry", func(t *testing.T) {
		path := writeFile(t, dir, "pairs", content)
		got, err := readAPIKeyFile(path, "ANTHROPIC_API_KEY", "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-ant-multi" {
			t.Errorf("got %q, want sk-ant-multi", got)
		}
	})

	t.Run("selects the openai entry (export + quotes)", func(t *testing.T) {
		path := writeFile(t, dir, "pairs2", content)
		got, err := readAPIKeyFile(path, "OPENAI_API_KEY", "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-openai-multi" {
			t.Errorf("got %q, want sk-openai-multi", got)
		}
	})

	t.Run("missing entry errors", func(t *testing.T) {
		path := writeFile(t, dir, "pairs3", content)
		if _, err := readAPIKeyFile(path, "MISSING_KEY", ""); err == nil {
			t.Fatal("expected an error for an absent entry")
		}
	})

	t.Run("pairs present but no env_key errors", func(t *testing.T) {
		path := writeFile(t, dir, "pairs4", content)
		if _, err := readAPIKeyFile(path, "", ""); err == nil {
			t.Fatal("expected an error when no env_key selects an entry")
		}
	})
}

func TestReadAPIKeyFileErrors(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing file", func(t *testing.T) {
		if _, err := readAPIKeyFile(filepath.Join(dir, "nope"), "ANTHROPIC_API_KEY", ""); err == nil {
			t.Fatal("expected an error for a missing file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeFile(t, dir, "empty", "   \n\n")
		if _, err := readAPIKeyFile(path, "ANTHROPIC_API_KEY", ""); err == nil {
			t.Fatal("expected an error for an empty file")
		}
	})
}

func TestReadAPIKeyFileRelativePath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".keys", "sk-beside-config")

	t.Run("bare name resolves against configDir", func(t *testing.T) {
		got, err := readAPIKeyFile(".keys", "ANTHROPIC_API_KEY", dir)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-beside-config" {
			t.Errorf("got %q, want sk-beside-config", got)
		}
	})

	t.Run("absolute path ignores configDir", func(t *testing.T) {
		path := writeFile(t, dir, "abs", "sk-abs")
		got, err := readAPIKeyFile(path, "ANTHROPIC_API_KEY", "/some/other/dir")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-abs" {
			t.Errorf("got %q, want sk-abs", got)
		}
	})
}

func TestResolveAPIKeyPrecedence(t *testing.T) {
	dir := t.TempDir()
	keyFile := writeFile(t, dir, "key", "sk-from-file")
	const envVar = "UNDERDASH_TEST_APIKEY"

	t.Run("env var wins over file and inline", func(t *testing.T) {
		t.Setenv(envVar, "sk-from-env")
		got, err := resolveAPIKey("sk-inline", keyFile, envVar, "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-from-env" {
			t.Errorf("got %q, want sk-from-env", got)
		}
	})

	t.Run("file wins over inline when env unset", func(t *testing.T) {
		os.Unsetenv(envVar)
		got, err := resolveAPIKey("sk-inline", keyFile, envVar, "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-from-file" {
			t.Errorf("got %q, want sk-from-file", got)
		}
	})

	t.Run("inline used when no env and no file", func(t *testing.T) {
		os.Unsetenv(envVar)
		got, err := resolveAPIKey("sk-inline", "", envVar, "")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got != "sk-inline" {
			t.Errorf("got %q, want sk-inline", got)
		}
	})

	t.Run("broken file errors only when env cannot satisfy it", func(t *testing.T) {
		os.Unsetenv(envVar)
		if _, err := resolveAPIKey("sk-inline", filepath.Join(dir, "nope"), envVar, ""); err == nil {
			t.Fatal("expected an error for an unreadable api_key_file")
		}
	})

	t.Run("env present short-circuits a broken file", func(t *testing.T) {
		t.Setenv(envVar, "sk-from-env")
		got, err := resolveAPIKey("", filepath.Join(dir, "nope"), envVar, "")
		if err != nil {
			t.Fatalf("env should satisfy the key before the file is read: %v", err)
		}
		if got != "sk-from-env" {
			t.Errorf("got %q, want sk-from-env", got)
		}
	})
}
