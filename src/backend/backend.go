package backend

import (
	"context"
	"fmt"
)

// Request is the payload sent to any backend.
type Request struct {
	SystemPrompt string
	UserMessage  string
	MaxTokens    int
}

// Backend sends a prompt and returns the raw response text.
type Backend interface {
	Send(ctx context.Context, req Request) (string, error)
	Name() string
}

// Config holds backend-specific configuration read from Viper.
type Config struct {
	Type     string
	Model    string
	Endpoint string
	APIKey   string
}

// New creates a backend by type name.
func New(cfg Config) (Backend, error) {
	switch cfg.Type {
	case "stdout":
		return &StdoutBackend{}, nil
	case "claude":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("claude backend requires an API key (set backends.claude.api_key in config or ANTHROPIC_API_KEY env var)")
		}
		model := cfg.Model
		if model == "" {
			model = "claude-sonnet-4-20250514"
		}
		return &ClaudeBackend{
			APIKey:   cfg.APIKey,
			Model:    model,
			Endpoint: cfg.Endpoint,
		}, nil
	case "openai":
		return nil, fmt.Errorf("openai backend is not yet implemented (coming in v0.2)")
	case "local":
		return nil, fmt.Errorf("local backend is not yet implemented (coming in v0.2)")
	case "http":
		return nil, fmt.Errorf("http backend is not yet implemented (coming in v0.2)")
	default:
		return nil, fmt.Errorf("unknown backend type: %q", cfg.Type)
	}
}
