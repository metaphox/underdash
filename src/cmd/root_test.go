package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// --- Finding #7: world-readable config warning ---

func TestCheckConfigPermissions_WorldReadable(t *testing.T) {
	// Create a temp config file with api_key and world-readable permissions.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("backends:\n  claude:\n    api_key: sk-secret-key\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	warning := checkConfigPermissions(cfgPath)
	if warning == "" {
		t.Error("expected a warning for world-readable config file containing api_key")
	}
}

func TestCheckConfigPermissions_RestrictedPerms(t *testing.T) {
	// Create a temp config with restricted permissions.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("backends:\n  claude:\n    api_key: sk-secret-key\n")
	if err := os.WriteFile(cfgPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	warning := checkConfigPermissions(cfgPath)
	if warning != "" {
		t.Errorf("unexpected warning for restricted config: %s", warning)
	}
}

func TestCheckConfigPermissions_NoAPIKey(t *testing.T) {
	// A world-readable config with no api_key should not warn.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte("default_backend: claude\nbackends:\n  claude:\n    env_key: ANTHROPIC_API_KEY\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	warning := checkConfigPermissions(cfgPath)
	if warning != "" {
		t.Errorf("unexpected warning for config without api_key: %s", warning)
	}
}

// --- Finding #2: --dry-run should show generated command, not prompt internals ---
// (Integration-level test for the flag behavior — tests the contract)

func TestDryRunFlag_Defined(t *testing.T) {
	// Verify --dry-run and --no-exec flags both exist.
	f := rootCmd.Flags().Lookup("dry-run")
	if f == nil {
		t.Error("--dry-run flag not defined")
	}
	f = rootCmd.Flags().Lookup("no-exec")
	if f == nil {
		t.Error("--no-exec flag not defined")
	}
}

func TestFlagErrorHint(t *testing.T) {
	// Unknown flag → hint with both the spelling and the "-- prompt" suggestion.
	msg, ok := flagErrorHint(errors.New("unknown flag: --these"))
	if !ok {
		t.Fatal("expected flagErrorHint to handle unknown flag error")
	}
	if !strings.Contains(msg, "check the spelling") {
		t.Errorf("missing spelling hint, got:\n%s", msg)
	}
	if !strings.Contains(msg, "_ -- these ...") {
		t.Errorf("missing dash-separator suggestion, got:\n%s", msg)
	}

	// Shorthand error → hint with the spelling and the "-- prompt" suggestion.
	msg, ok = flagErrorHint(errors.New("unknown shorthand flag: 'x' in -x"))
	if !ok {
		t.Fatal("expected flagErrorHint to handle unknown shorthand flag error")
	}
	if !strings.Contains(msg, "check the spelling") {
		t.Errorf("missing spelling hint, got:\n%s", msg)
	}
	if !strings.Contains(msg, "_ -- -x ...") {
		t.Errorf("missing dash-prefix suggestion, got:\n%s", msg)
	}

	// Non-flag errors pass through untouched.
	if _, ok := flagErrorHint(errors.New("network timeout")); ok {
		t.Error("flagErrorHint should not claim unrelated errors")
	}
}

func TestNormalizeLeadingArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "underdash", Args: cobra.ArbitraryArgs}
	registerRootFlags(cmd)
	flags := cmd.Flags()

	tests := []struct {
		name string
		argv []string
		want []string
	}{
		{
			name: "leading negative number becomes prompt",
			argv: []string{"-1", "times", "-1"},
			want: []string{"--", "-1", "times", "-1"},
		},
		{
			name: "unknown letter shorthand left for pflag to reject",
			argv: []string{"-x", "plus", "y"},
			want: []string{"-x", "plus", "y"},
		},
		{
			name: "bool flag before negative number is skipped",
			argv: []string{"-n", "-1", "times"},
			want: []string{"-n", "--", "-1", "times"},
		},
		{
			name: "explicit -- is left untouched",
			argv: []string{"--", "-1", "foo"},
			want: []string{"--", "-1", "foo"},
		},
		{
			name: "long flag then bareword prompt unchanged",
			argv: []string{"--dry-run", "tar", "dir"},
			want: []string{"--dry-run", "tar", "dir"},
		},
		{
			name: "negative number after a bareword is left to interspersed=false",
			argv: []string{"tar", "-1", "dir"},
			want: []string{"tar", "-1", "dir"},
		},
		{
			name: "value-taking flag stops the scan",
			argv: []string{"--backend", "claude", "-1", "x"},
			want: []string{"--backend", "claude", "-1", "x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLeadingArgs(flags, tc.argv)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("normalizeLeadingArgs(%v) = %v, want %v", tc.argv, got, tc.want)
			}
		})
	}
}

// --- "--" as flag prefix and as prompt separator ---
//
// Flags are recognized only before the prompt starts (SetInterspersed(false)),
// and a standalone "--" forces everything after it into the prompt verbatim.

func TestPromptArgParsing(t *testing.T) {
	tests := []struct {
		name       string
		argv       []string
		wantDryRun bool
		wantPrompt string
	}{
		{
			name:       "flag before prompt is parsed",
			argv:       []string{"--dry-run", "tar", "the", "dir"},
			wantDryRun: true,
			wantPrompt: "tar the dir",
		},
		{
			name:       "flag-looking token inside prompt is kept",
			argv:       []string{"tar", "the", "--dry-run", "dir"},
			wantDryRun: false,
			wantPrompt: "tar the --dry-run dir",
		},
		{
			name:       "dash-dash forces rest into prompt",
			argv:       []string{"--", "--dry-run", "anything"},
			wantDryRun: false,
			wantPrompt: "--dry-run anything",
		},
		{
			name:       "flag then dash-dash then dashy prompt",
			argv:       []string{"--dry-run", "--", "--weird", "prompt"},
			wantDryRun: true,
			wantPrompt: "--weird prompt",
		},
		{
			name:       "shorthand flag before prompt",
			argv:       []string{"-n", "list", "files"},
			wantDryRun: true,
			wantPrompt: "list files",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "underdash", Args: cobra.ArbitraryArgs}
			registerRootFlags(cmd)

			if err := cmd.ParseFlags(tc.argv); err != nil {
				t.Fatalf("ParseFlags(%v): %v", tc.argv, err)
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if dryRun != tc.wantDryRun {
				t.Errorf("dry-run = %v, want %v", dryRun, tc.wantDryRun)
			}

			gotPrompt := strings.Join(cmd.Flags().Args(), " ")
			if gotPrompt != tc.wantPrompt {
				t.Errorf("prompt = %q, want %q", gotPrompt, tc.wantPrompt)
			}
		})
	}
}
