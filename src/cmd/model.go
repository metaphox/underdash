package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	spin.Start("Discovering available models", "", time.Time{})
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

	fmt.Fprintln(out, "Select a model (press Enter for the first):")
	for i, id := range ranked {
		fmt.Fprintf(out, "  %d) %s\n", i+1, id)
	}
	fmt.Fprint(out, "> ")

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
// preserving existing comments and untouched lines. When the target backend
// already exists in the file, it edits just the one model line (in place, or
// inserted after the backend's other keys); otherwise it falls back to building
// the missing structure via a node tree. The file is created if absent.
func persistModel(path, backendName, model string) error {
	raw, err := os.ReadFile(path)
	if err == nil && len(bytes.TrimSpace(raw)) > 0 {
		patched, ok, perr := patchModelLine(raw, backendName, model)
		if perr != nil {
			return perr
		}
		if ok {
			return writeConfigFile(path, patched)
		}
	}
	return rebuildWithModel(path, raw, backendName, model)
}

// patchModelLine surgically sets backends.<name>.model by editing a single line
// of the source, leaving every other byte untouched. It only applies when the
// backend already exists as a mapping with a `type` key; otherwise it returns
// ok=false so the caller can build the missing structure instead.
func patchModelLine(raw []byte, backendName, model string) ([]byte, bool, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, false, fmt.Errorf("parse existing config: %w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, false, nil
	}
	doc := root.Content[0]

	backends, ok := yamlMappingValue(doc, "backends")
	if !ok || backends.Kind != yaml.MappingNode {
		return nil, false, nil
	}
	entry, ok := yamlMappingValue(backends, backendName)
	if !ok || entry.Kind != yaml.MappingNode || len(entry.Content) == 0 {
		return nil, false, nil
	}
	// Defer to the rebuild path when `type` is missing, so it can add both keys.
	if _, ok := yamlMappingValue(entry, "type"); !ok {
		return nil, false, nil
	}

	lines := strings.Split(string(raw), "\n")

	// Replace an existing model line in place. Swap just the value token by its
	// source position so the prefix and any inline comment (and its spacing)
	// stay byte-identical; fall back to rebuilding the line if the source text
	// doesn't match the plain value (e.g. a quoted scalar).
	if keyNode, valNode := yamlMappingEntry(entry, "model"); keyNode != nil {
		i := keyNode.Line - 1
		if i < 0 || i >= len(lines) {
			return nil, false, nil
		}
		line := lines[i]
		start := valNode.Column - 1
		end := start + len(valNode.Value)
		if start >= 0 && end <= len(line) && line[start:end] == valNode.Value {
			lines[i] = line[:start] + model + line[end:]
		} else {
			rebuilt := leadingSpaces(line) + "model: " + model
			if valNode.LineComment != "" {
				rebuilt += " " + valNode.LineComment
			}
			lines[i] = rebuilt
		}
		return []byte(strings.Join(lines, "\n")), true, nil
	}

	// No model key yet — insert one after the backend's last existing key,
	// matching that key's indentation.
	indent := entry.Content[0].Column - 1
	last := 0
	for _, n := range entry.Content {
		if n.Line > last {
			last = n.Line
		}
	}
	if indent < 0 || last <= 0 || last > len(lines) {
		return nil, false, nil
	}
	newLine := strings.Repeat(" ", indent) + "model: " + model
	lines = append(lines[:last], append([]string{newLine}, lines[last:]...)...)
	return []byte(strings.Join(lines, "\n")), true, nil
}

// rebuildWithModel sets backends.<name>.model (and type, if absent) by editing a
// node tree and re-encoding. Used for a fresh/empty file or one that lacks the
// backend, where there is no existing structure to preserve byte-for-byte.
func rebuildWithModel(path string, raw []byte, backendName, model string) error {
	var root yaml.Node
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := yaml.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	}

	doc := yamlDocMapping(&root)
	backends := yamlEnsureMapping(doc, "backends")
	entry := yamlEnsureMapping(backends, backendName)
	if _, ok := yamlMappingValue(entry, "type"); !ok {
		yamlSetScalar(entry, "type", backendName)
	}
	yamlSetScalar(entry, "model", model)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return writeConfigFile(path, buf.Bytes())
}

// writeConfigFile writes data to path (0600), creating the parent directory.
func writeConfigFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// leadingSpaces returns the run of leading spaces of s.
func leadingSpaces(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " "))]
}

// yamlDocMapping returns the mapping node at the document root, creating the
// document/mapping scaffolding when root is empty or not a mapping.
func yamlDocMapping(root *yaml.Node) *yaml.Node {
	if root.Kind == 0 {
		root.Kind = yaml.DocumentNode
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		root.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	return root.Content[0]
}

// yamlMappingValue returns the value node for key in a mapping node.
func yamlMappingValue(m *yaml.Node, key string) (*yaml.Node, bool) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], true
		}
	}
	return nil, false
}

// yamlMappingEntry returns the key and value nodes for key in a mapping node, or
// (nil, nil) when absent.
func yamlMappingEntry(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

// yamlEnsureMapping returns the mapping value for key, creating an empty mapping
// entry when the key is absent or its value is not a mapping.
func yamlEnsureMapping(m *yaml.Node, key string) *yaml.Node {
	if v, ok := yamlMappingValue(m, key); ok && v.Kind == yaml.MappingNode {
		return v
	} else if ok {
		v.Kind = yaml.MappingNode
		v.Tag = "!!map"
		v.Value = ""
		v.Content = nil
		return v
	}
	valNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		valNode,
	)
	return valNode
}

// yamlSetScalar sets key to a string value in a mapping node, updating the
// existing value node in place (keeping its comments) or appending a new entry.
func yamlSetScalar(m *yaml.Node, key, value string) {
	if v, ok := yamlMappingValue(m, key); ok {
		v.Kind = yaml.ScalarNode
		v.Tag = "!!str"
		v.Value = value
		v.Content = nil
		return
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}
