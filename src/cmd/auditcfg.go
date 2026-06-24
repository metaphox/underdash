package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"metaphox/underdash/audit"
)

// auditConfig builds the audit.Config from the `audit` config section.
func auditConfig() audit.Config {
	return auditConfigFrom(
		viper.GetBool("audit.enabled"),
		viper.GetString("audit.path"),
		viper.GetString("audit.max_size"),
	)
}

// auditConfigFrom is the testable core of auditConfig. When enabled with no
// path, it defaults to the XDG state dir.
func auditConfigFrom(enabled bool, path, maxSize string) audit.Config {
	cfg := audit.Config{Enabled: enabled, MaxSize: parseSize(maxSize)}
	if enabled {
		if path == "" {
			path = defaultAuditPath()
		}
		cfg.Path = expandHome(path)
	}
	return cfg
}

// defaultAuditPath returns ~/.local/state/underdash/audit.jsonl.
func defaultAuditPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "underdash", "audit.jsonl")
}

// expandHome resolves a leading "~/" against the user's home directory.
func expandHome(path string) string {
	if path == "" || !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

// parseSize parses a byte count with an optional KB/MB/GB suffix (case
// insensitive). Returns 0 for empty or unparseable input.
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	upper := strings.ToUpper(s)

	var mult int64 = 1
	switch {
	case strings.HasSuffix(upper, "GB"):
		mult, upper = 1<<30, strings.TrimSuffix(upper, "GB")
	case strings.HasSuffix(upper, "MB"):
		mult, upper = 1<<20, strings.TrimSuffix(upper, "MB")
	case strings.HasSuffix(upper, "KB"):
		mult, upper = 1<<10, strings.TrimSuffix(upper, "KB")
	}

	n, err := strconv.ParseInt(strings.TrimSpace(upper), 10, 64)
	if err != nil {
		return 0
	}
	return n * mult
}
