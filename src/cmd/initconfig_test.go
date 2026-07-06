package cmd

import (
	"os"
	"path/filepath"
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

func TestDefaultConfigTemplateParses(t *testing.T) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(defaultConfigTemplate), &doc); err != nil {
		t.Fatalf("template is not valid YAML: %v", err)
	}

	if got := doc["default_backend"]; got != "claude" {
		t.Errorf("default_backend = %v, want claude", got)
	}

	output, _ := doc["output"].(map[string]any)
	if got := output["mode"]; got != "streaming" {
		t.Errorf("output.mode = %v, want streaming", got)
	}
	if got := output["markdown"]; got != true {
		t.Errorf("output.markdown = %v, want true", got)
	}

	attach, _ := doc["attach"].(map[string]any)
	if got := attach["max_bytes"]; got != 5242880 {
		t.Errorf("attach.max_bytes = %v, want 5242880", got)
	}
}

func TestRunInitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	noPrompt := func(string) bool { t.Fatal("prompt should not be called for a new file"); return false }
	if err := runInit(path, false, noPrompt); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
	content, _ := os.ReadFile(path)
	if string(content) != defaultConfigTemplate {
		t.Error("written content does not match the template")
	}
}

func TestRunInitCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "underdash", "config.yaml")

	if err := runInit(path, true, nil); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created in nested dir: %v", err)
	}
}

func TestRunInitEmptyPath(t *testing.T) {
	if err := runInit("", false, nil); err == nil {
		t.Fatal("empty path should return an error")
	}
}

func TestRunInitOverwrite(t *testing.T) {
	write := func(t *testing.T, path string) {
		if err := os.WriteFile(path, []byte("old: content\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("autoYes overwrites without prompting", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		write(t, path)

		noPrompt := func(string) bool { t.Fatal("prompt should not be called with autoYes"); return false }
		if err := runInit(path, true, noPrompt); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		content, _ := os.ReadFile(path)
		if string(content) != defaultConfigTemplate {
			t.Error("autoYes should overwrite with the template")
		}
	})

	t.Run("prompt no leaves file unchanged", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		write(t, path)

		if err := runInit(path, false, func(string) bool { return false }); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		content, _ := os.ReadFile(path)
		if string(content) != "old: content\n" {
			t.Error("declining should leave the existing file untouched")
		}
	})

	t.Run("prompt yes overwrites", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		write(t, path)

		if err := runInit(path, false, func(string) bool { return true }); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		content, _ := os.ReadFile(path)
		if string(content) != defaultConfigTemplate {
			t.Error("accepting should overwrite with the template")
		}
	})
}
