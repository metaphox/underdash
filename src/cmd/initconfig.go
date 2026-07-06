package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultConfigTemplate is the annotated config written by --init. Every
// overridable key is present with its in-code default; comments explain each
// one. List-typed policies and alternate backends are shown commented so the
// file works out of the box for the default claude backend while showing how to
// extend it. Keep this aligned with the config section in README.md.
const defaultConfigTemplate = `# Underdash configuration.
# Precedence: CLI flags > environment variables (UNDERDASH_*) > this file.

# Which backend to use when --backend is not given.
default_backend: claude

# Configure one or more backends under backends.<name>.
backends:
  claude:
    type: claude
    env_key: ANTHROPIC_API_KEY   # env var holding the API key
    # api_key: sk-ant-...        # or paste the key here (chmod 600 the file)
    # api_key_file: .keys         # or read it from a file (raw key, or NAME=value pairs); relative to this config
    # model:                     # omit to auto-discover and pin the best model
  # openai:
  #   type: openai
  #   env_key: OPENAI_API_KEY
  # local:
  #   type: local
  #   endpoint: http://localhost:11434/v1   # any OpenAI-compatible endpoint
  #   model: llama3

# Command execution policy — glob patterns override the built-in risk rules.
execution:
  auto_run: []    # always run without asking, e.g. ["docker ps *", "kubectl get *"]
  confirm: []     # always ask first, e.g. ["git push *"]
  deny: []        # never run, even with -y, e.g. ["rm -rf /*"]

# Output rendering.
output:
  mode: streaming   # streaming or plain
  markdown: true    # render explanations as Markdown

# Extra local context sent to the model (off by default).
context:
  history: false        # include recent shell history in the prompt
  history_lines: 20     # how many history lines when enabled

# Audit log of invocations (off by default).
audit:
  enabled: false
  path: ~/.local/state/underdash/audit.jsonl
  max_size: ""          # e.g. 10MB or 1GB; empty means no rotation

# File attachments (@file syntax).
attach:
  max_bytes: 5242880    # per-file cap, 5 MiB
`

// runInit writes the default config template to path, prompting before
// overwriting an existing file unless autoYes is set. It is the cobra-independent
// core of the --init flag; prompt is injected for testability.
func runInit(path string, autoYes bool, prompt func(string) bool) error {
	if path == "" {
		return fmt.Errorf("could not resolve config file path")
	}

	if _, err := os.Stat(path); err == nil && !autoYes {
		if !prompt(fmt.Sprintf("Config already exists at %s. Overwrite? [y/N] ", path)) {
			fmt.Fprintln(os.Stderr, "init cancelled")
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o600); err != nil {
		return err
	}

	fmt.Printf("wrote default config to %s\n", path)
	return nil
}
