package cmd

import (
	"path/filepath"
	"testing"
)

func TestIsRemoteBackend(t *testing.T) {
	remote := map[string]bool{
		"claude": true, "openai": true, "http": true,
		"local": false, "stdout": false, "": false,
	}
	for name, want := range remote {
		if got := isRemoteBackend(name); got != want {
			t.Errorf("isRemoteBackend(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestEnsureConsentAt(t *testing.T) {
	noPrompt := func(string) bool { t.Fatal("prompt should not be called"); return false }

	t.Run("non-remote skips entirely", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "consent")
		if err := ensureConsentAt(path, "local", false, noPrompt); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if hasConsent(path) {
			t.Error("should not write consent for a local backend")
		}
	})

	t.Run("autoYes records without prompting", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "consent")
		if err := ensureConsentAt(path, "claude", true, noPrompt); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if !hasConsent(path) {
			t.Error("autoYes should persist consent")
		}
	})

	t.Run("already consented is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "consent")
		if err := saveConsent(path); err != nil {
			t.Fatal(err)
		}
		if err := ensureConsentAt(path, "openai", false, noPrompt); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})

	t.Run("prompt yes records consent", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "consent")
		if err := ensureConsentAt(path, "claude", false, func(string) bool { return true }); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if !hasConsent(path) {
			t.Error("a yes answer should persist consent")
		}
	})

	t.Run("prompt no aborts without recording", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "consent")
		err := ensureConsentAt(path, "claude", false, func(string) bool { return false })
		if err == nil {
			t.Fatal("a no answer should return an error")
		}
		if hasConsent(path) {
			t.Error("declined consent must not be persisted")
		}
	})
}
