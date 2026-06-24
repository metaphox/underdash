package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"metaphox/underdash/backend"
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
			name: "claude via flag with config key",
			flag: "claude",
			// model set so resolution doesn't trigger network discovery.
			setup: func() {
				viper.Set("backends.claude.api_key", "sk-test")
				viper.Set("backends.claude.model", "claude-sonnet-4-5")
			},
			wantName: "claude",
		},
		{
			name: "openai via flag with config key",
			flag: "openai",
			setup: func() {
				viper.Set("backends.openai.api_key", "sk-test")
				viper.Set("backends.openai.model", "gpt-4o")
			},
			wantName: "openai",
		},
		{
			name: "local via flag with endpoint, no key",
			flag: "local",
			setup: func() {
				viper.Set("backends.local.endpoint", "http://localhost:11434/v1")
				viper.Set("backends.local.model", "llama3")
			},
			wantName: "local",
		},
		{
			name: "http via flag with endpoint",
			flag: "http",
			setup: func() {
				viper.Set("backends.http.endpoint", "http://localhost:8080/v1")
				viper.Set("backends.http.model", "local-model")
			},
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

			be, _, err := resolveBackend(context.Background(), newCmd(tt.flag))
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

// TestResolveBackend_DiscoversAndPersists exercises model discovery end-to-end
// against a mock OpenAI-compatible endpoint: no model configured → list → auto-pick
// (non-TTY) → persist to the config file.
func TestResolveBackend_DiscoversAndPersists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4o"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("backends.local.endpoint", srv.URL+"/v1") // no model configured

	tmpCfg := filepath.Join(t.TempDir(), "config.yaml")
	oldCfg := cfgFile
	cfgFile = tmpCfg
	t.Cleanup(func() { cfgFile = oldCfg })

	cmd := &cobra.Command{}
	cmd.Flags().String("backend", "", "")
	cmd.Flags().Bool("yes", false, "")
	if err := cmd.Flags().Set("backend", "local"); err != nil {
		t.Fatal(err)
	}

	be, cfg, err := resolveBackend(context.Background(), cmd)
	if err != nil {
		t.Fatalf("resolveBackend: %v", err)
	}
	// Auto-pick (non-TTY in tests) prefers gpt-4o over gpt-4o-mini.
	if cfg.Model != "gpt-4o" {
		t.Errorf("discovered model = %q, want gpt-4o", cfg.Model)
	}
	if got := be.(*backend.OpenAIBackend); got == nil {
		t.Fatal("expected *OpenAIBackend")
	}
	// The choice must have been persisted to the config file.
	if got := readModel(t, tmpCfg, "local"); got != "gpt-4o" {
		t.Errorf("persisted model = %q, want gpt-4o", got)
	}
}

// TestMaybeSelfHeal verifies a model-not-found 404 triggers re-discovery, a
// rebuilt backend with the replacement model, and a config rewrite.
func TestMaybeSelfHeal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	viper.Reset()
	t.Cleanup(viper.Reset)
	tmpCfg := filepath.Join(t.TempDir(), "config.yaml")
	oldCfg := cfgFile
	cfgFile = tmpCfg
	t.Cleanup(func() { cfgFile = oldCfg })

	cfg := backend.Config{Type: "local", Endpoint: srv.URL + "/v1", Model: "retired-model"}
	be, err := backend.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("backend", "", "")
	_ = cmd.Flags().Set("backend", "local")

	sendErr := &backend.APIError{StatusCode: 404, Type: "model_not_found", Message: "no such model"}
	newBe, healed := maybeSelfHeal(context.Background(), cmd, &cfg, be, sendErr)
	if !healed {
		t.Fatal("expected self-heal to trigger")
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("cfg.Model = %q, want gpt-4o", cfg.Model)
	}
	if m := newBe.(*backend.OpenAIBackend).Model; m != "gpt-4o" {
		t.Errorf("rebuilt backend model = %q, want gpt-4o", m)
	}
	if got := readModel(t, tmpCfg, "local"); got != "gpt-4o" {
		t.Errorf("persisted model = %q, want gpt-4o", got)
	}

	// A non-model error must not self-heal.
	if _, healed := maybeSelfHeal(context.Background(), cmd, &cfg, be, context.Canceled); healed {
		t.Error("non-model error should not self-heal")
	}
}
