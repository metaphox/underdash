package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// isRemoteBackend reports whether a backend transmits local context to a third
// party, and so requires first-run acknowledgment. local and stdout do not.
func isRemoteBackend(name string) bool {
	switch name {
	case "claude", "openai", "http":
		return true
	}
	return false
}

// consentPath returns the first-run acknowledgment marker file path.
func consentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "underdash", "consent"), nil
}

func hasConsent(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func saveConsent(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("acknowledged\n"), 0o644)
}

// ensureConsent gates a remote backend on a persisted first-run acknowledgment,
// prompting the user once. autoYes (-y) accepts without prompting.
func ensureConsent(backendName string, autoYes bool) error {
	path, err := consentPath()
	if err != nil {
		return nil // can't resolve home; don't block the user
	}
	return ensureConsentAt(path, backendName, autoYes, promptYesNo)
}

// ensureConsentAt is the testable core of ensureConsent with an injectable
// marker path and prompt function.
func ensureConsentAt(path, backendName string, autoYes bool, prompt func(string) bool) error {
	if !isRemoteBackend(backendName) {
		return nil
	}
	if hasConsent(path) {
		return nil
	}
	if !autoYes {
		fmt.Fprintln(os.Stderr, disclosureText)
		fmt.Fprintln(os.Stderr)
		if !prompt(fmt.Sprintf("Send this context to the %q backend? [y/N] ", backendName)) {
			return fmt.Errorf("aborted: data disclosure not acknowledged")
		}
	}
	return saveConsent(path)
}

// promptYesNo asks a yes/no question on stderr and reads the answer from stdin.
func promptYesNo(question string) bool {
	fmt.Fprint(os.Stderr, question)
	var answer string
	_, _ = fmt.Fscanln(os.Stdin, &answer)
	switch answer {
	case "y", "Y", "yes", "Yes", "YES":
		return true
	}
	return false
}
