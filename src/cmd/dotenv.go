package cmd

import (
	"os"
	"path/filepath"

	"github.com/subosito/gotenv"
)

// loadDotEnv loads a .env file from the directory of the running binary, if
// present. Existing environment variables are preserved (not overridden).
// Failures — missing file, unreadable file, parse errors — are silently
// ignored: the .env is a convenience, not a requirement.
func loadDotEnv() {
	path, ok := binaryDotEnvPath()
	if !ok {
		return
	}
	loadDotEnvFrom(path)
}

func loadDotEnvFrom(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	_ = gotenv.Load(path)
}

func binaryDotEnvPath() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	// Follow symlinks so we find the .env next to the real binary even when
	// the executable is invoked through a symlink (e.g., aliased as `_`).
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), ".env"), true
}
