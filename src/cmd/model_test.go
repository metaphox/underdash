package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

func TestChooseModel(t *testing.T) {
	ranked := []string{"claude-sonnet-4-5", "claude-opus-4", "claude-haiku"}

	t.Run("non-interactive auto-picks first", func(t *testing.T) {
		got := chooseModel(ranked, false, strings.NewReader(""), &bytes.Buffer{})
		if got != "claude-sonnet-4-5" {
			t.Errorf("got %q, want first", got)
		}
	})

	t.Run("interactive picks the chosen number", func(t *testing.T) {
		got := chooseModel(ranked, true, strings.NewReader("2\n"), &bytes.Buffer{})
		if got != "claude-opus-4" {
			t.Errorf("got %q, want claude-opus-4", got)
		}
	})

	t.Run("interactive empty input takes default", func(t *testing.T) {
		got := chooseModel(ranked, true, strings.NewReader("\n"), &bytes.Buffer{})
		if got != "claude-sonnet-4-5" {
			t.Errorf("got %q, want default first", got)
		}
	})

	t.Run("empty list returns empty", func(t *testing.T) {
		if got := chooseModel(nil, false, strings.NewReader(""), &bytes.Buffer{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestPersistModel_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := persistModel(path, "claude", "claude-sonnet-4-5"); err != nil {
		t.Fatal(err)
	}

	if got := readModel(t, path, "claude"); got != "claude-sonnet-4-5" {
		t.Errorf("persisted model = %q", got)
	}
}

func TestPersistModel_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	existing := "default_backend: claude\nbackends:\n  claude:\n    api_key: sk-secret\n"
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := persistModel(path, "claude", "claude-opus-4"); err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	raw, _ := os.ReadFile(path)
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["default_backend"] != "claude" {
		t.Errorf("default_backend not preserved: %v", doc["default_backend"])
	}
	backends := doc["backends"].(map[string]any)
	claude := backends["claude"].(map[string]any)
	if claude["api_key"] != "sk-secret" {
		t.Errorf("api_key not preserved: %v", claude["api_key"])
	}
	if claude["model"] != "claude-opus-4" {
		t.Errorf("model not set: %v", claude["model"])
	}
}

func TestConfigWritable(t *testing.T) {
	dir := t.TempDir()
	if !configWritable(filepath.Join(dir, "sub", "config.yaml")) {
		t.Error("expected writable path under a temp dir")
	}

	// A path whose parent is a regular file cannot be created.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if configWritable(filepath.Join(blocker, "config.yaml")) {
		t.Error("expected non-writable path under a file")
	}
}

func readModel(t *testing.T, path, backendName string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	backends, ok := doc["backends"].(map[string]any)
	if !ok {
		return ""
	}
	be, ok := backends[backendName].(map[string]any)
	if !ok {
		return ""
	}
	m, _ := be["model"].(string)
	return m
}
