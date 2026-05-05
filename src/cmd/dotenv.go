package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnv loads a .env file from the directory of the running binary, if
// present. Existing environment variables are preserved (not overridden).
// Failures — missing file, unreadable file, parse errors on individual lines
// — are silently ignored: the .env is a convenience, not a requirement, and
// a malformed line must not suppress valid lines around it.
func loadDotEnv() {
	path, ok := binaryDotEnvPath()
	if !ok {
		return
	}
	loadDotEnvFrom(path)
}

func loadDotEnvFrom(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := parseDotEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value) // only fails for an invalid key, which parseDotEnvLine rejects
	}
}

// parseDotEnvLine parses a single .env line. Returns (key, value, true) for a
// valid KEY=VALUE pair. Blank lines, comment lines, and malformed lines return
// ok=false and are meant to be skipped silently.
func parseDotEnvLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}

	// Optional `export ` prefix.
	trimmed = strings.TrimPrefix(trimmed, "export ")
	trimmed = strings.TrimLeft(trimmed, " \t")

	idx := strings.IndexByte(trimmed, '=')
	if idx <= 0 {
		return "", "", false
	}

	key := strings.TrimSpace(trimmed[:idx])
	value := strings.TrimSpace(trimmed[idx+1:])
	if key == "" || !isValidKey(key) {
		return "", "", false
	}

	// Strip matching surrounding quotes.
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, true
}

// isValidKey checks that key looks like a shell-compatible identifier.
func isValidKey(key string) bool {
	for i, r := range key {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
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
