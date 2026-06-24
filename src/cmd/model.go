package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	yaml "go.yaml.in/yaml/v3"

	"metaphox/underdash/backend"
	"metaphox/underdash/display"
)

// resolveModelFor discovers an available model for the backend: it lists the
// provider's models, ranks them, and picks one (interactively on a TTY, or the
// top-ranked default otherwise). A spinner covers the network call.
func resolveModelFor(ctx context.Context, lister backend.ModelLister, backendType string, interactive bool) (string, error) {
	spin := display.NewSpinner()
	spin.Start("Discovering available models...")
	ids, err := lister.ListModels(ctx)
	spin.Stop()
	if err != nil {
		return "", err
	}

	ranked := backend.RankModels(backendType, ids)
	if len(ranked) == 0 {
		return "", fmt.Errorf("no usable models offered by the %s endpoint", backendType)
	}
	return chooseModel(ranked, interactive, os.Stdin, os.Stderr), nil
}

// chooseModel returns the selected model. Non-interactive (piped / -y) auto-picks
// the top-ranked model; interactive presents a numbered menu defaulting to it.
func chooseModel(ranked []string, interactive bool, in io.Reader, out io.Writer) string {
	if len(ranked) == 0 {
		return ""
	}
	if !interactive || len(ranked) == 1 {
		return ranked[0]
	}

	_, _ = fmt.Fprintln(out, "Select a model (press Enter for the first):")
	for i, id := range ranked {
		_, _ = fmt.Fprintf(out, "  %d) %s\n", i+1, id)
	}
	_, _ = fmt.Fprint(out, "> ")

	line, _ := bufio.NewReader(in).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return ranked[0]
	}
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(ranked) {
		return ranked[n-1]
	}
	return ranked[0]
}

// configFilePath resolves where to persist config: an explicit --config, then
// the loaded config file, then the default location.
func configFilePath() string {
	if cfgFile != "" {
		return cfgFile
	}
	if used := viper.ConfigFileUsed(); used != "" {
		return used
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "underdash", "config.yaml")
}

// configWritable reports whether the config file at path can be created/written,
// creating the parent directory (and an empty file) as needed.
func configWritable(path string) bool {
	if path == "" {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// persistModel writes backends.<name>.model into the YAML config at path,
// preserving any existing content. The file is created if absent.
func persistModel(path, backendName, model string) error {
	doc := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
		if doc == nil {
			doc = map[string]any{}
		}
	}

	backends, _ := doc["backends"].(map[string]any)
	if backends == nil {
		backends = map[string]any{}
		doc["backends"] = backends
	}
	entry, _ := backends[backendName].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
		backends[backendName] = entry
	}
	if _, ok := entry["type"]; !ok {
		entry["type"] = backendName
	}
	entry["model"] = model

	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}
