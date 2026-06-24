package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"1024", 1024},
		{"10MB", 10 << 20},
		{"10mb", 10 << 20},
		{"512KB", 512 << 10},
		{"2GB", 2 << 30},
		{"garbage", 0},
	}
	for _, tt := range tests {
		if got := parseSize(tt.in); got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	if got := expandHome("~/x/y"); got != filepath.Join(home, "x/y") {
		t.Errorf("expandHome(~/x/y) = %q", got)
	}
	if got := expandHome("/abs/path"); got != "/abs/path" {
		t.Errorf("expandHome(/abs/path) = %q, want unchanged", got)
	}
	if got := expandHome(""); got != "" {
		t.Errorf("expandHome(\"\") = %q, want empty", got)
	}
}

func TestAuditConfigDefaultPath(t *testing.T) {
	// When enabled with no explicit path, a sensible default under the state dir
	// is used.
	cfg := auditConfigFrom(true, "", "")
	if cfg.Path == "" {
		t.Error("enabled audit with no path should get a default path")
	}
	if !filepath.IsAbs(cfg.Path) {
		t.Errorf("default audit path should be absolute, got %q", cfg.Path)
	}
}
