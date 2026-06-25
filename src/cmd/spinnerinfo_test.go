package cmd

import (
	"context"
	"testing"

	"metaphox/underdash/backend"
	"metaphox/underdash/sysinfo"
)

// namedBackend is a minimal backend.Backend used only to exercise modelLabel.
type namedBackend struct{ name string }

func (n namedBackend) Name() string { return n.name }
func (namedBackend) Send(context.Context, backend.Request) (string, error) {
	return "", nil
}

func TestModelLabel(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		model   string
		want    string
	}{
		{"claude trims redundant prefix", "claude", "claude-opus-4-8", "claude/opus-4-8"},
		{"openai keeps unprefixed model", "openai", "gpt-4o", "openai/gpt-4o"},
		{"local model", "local", "llama3", "local/llama3"},
		{"empty model shows backend only", "stdout", "", "stdout"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := modelLabel(namedBackend{name: tc.backend}, tc.model)
			if got != tc.want {
				t.Errorf("modelLabel(%q, %q) = %q, want %q", tc.backend, tc.model, got, tc.want)
			}
		})
	}
}

func TestContextSummary(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *sysinfo.SystemContext
		attCount int
		want     string
	}{
		{
			name: "empty when nothing gathered",
			ctx:  &sysinfo.SystemContext{},
			want: "",
		},
		{
			name: "full set in order",
			ctx: &sysinfo.SystemContext{
				InGitRepo:    true,
				ProjectType:  "go",
				PathTools:    make([]string, 14),
				ShellHistory: []string{"ls"},
			},
			attCount: 2,
			want:     "git, go, 14 tools, history, 2 files",
		},
		{
			name:     "single attachment is singular",
			ctx:      &sysinfo.SystemContext{PathTools: make([]string, 3)},
			attCount: 1,
			want:     "3 tools, 1 file",
		},
		{
			name: "tools only",
			ctx:  &sysinfo.SystemContext{PathTools: make([]string, 5)},
			want: "5 tools",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := contextSummary(tc.ctx, tc.attCount)
			if got != tc.want {
				t.Errorf("contextSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}
