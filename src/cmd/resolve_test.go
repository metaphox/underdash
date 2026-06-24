package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// TestResolveBackend exercises --backend / config resolution across the M2
// backend set, asserting the resolved backend's identity.
func TestResolveBackend(t *testing.T) {
	// Keep env-var key fallbacks from interfering with the keyless cases.
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	newCmd := func(backendFlag string) *cobra.Command {
		c := &cobra.Command{}
		c.Flags().String("backend", "", "")
		if backendFlag != "" {
			if err := c.Flags().Set("backend", backendFlag); err != nil {
				t.Fatal(err)
			}
		}
		return c
	}

	tests := []struct {
		name     string
		flag     string
		setup    func()
		wantName string
		wantErr  bool
	}{
		{
			name:     "stdout via flag",
			flag:     "stdout",
			setup:    func() {},
			wantName: "stdout",
		},
		{
			name:     "claude via flag with config key",
			flag:     "claude",
			setup:    func() { viper.Set("backends.claude.api_key", "sk-test") },
			wantName: "claude",
		},
		{
			name:     "openai via flag with config key",
			flag:     "openai",
			setup:    func() { viper.Set("backends.openai.api_key", "sk-test") },
			wantName: "openai",
		},
		{
			name:     "local via flag with endpoint, no key",
			flag:     "local",
			setup:    func() { viper.Set("backends.local.endpoint", "http://localhost:11434/v1") },
			wantName: "local",
		},
		{
			name:     "http via flag with endpoint",
			flag:     "http",
			setup:    func() { viper.Set("backends.http.endpoint", "http://localhost:8080/v1") },
			wantName: "http",
		},
		{
			name:     "default_backend from config when no flag",
			flag:     "",
			setup:    func() { viper.Set("default_backend", "stdout") },
			wantName: "stdout",
		},
		{
			name:    "openai missing key errors",
			flag:    "openai",
			setup:   func() {},
			wantErr: true,
		},
		{
			name:    "local missing endpoint errors",
			flag:    "local",
			setup:   func() {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			tt.setup()
			t.Cleanup(viper.Reset)

			be, err := resolveBackend(newCmd(tt.flag))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveBackend(%q) should error", tt.flag)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveBackend(%q) unexpected error: %v", tt.flag, err)
			}
			if be.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", be.Name(), tt.wantName)
			}
		})
	}
}
