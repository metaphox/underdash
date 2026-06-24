// Package backend provides LLM backend implementations.
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
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai backend requires an API key (set backends.openai.api_key in config or OPENAI_API_KEY env var)")
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = defaultOpenAIEndpoint
		}
		model := cfg.Model
		if model == "" {
			model = defaultOpenAIModel
		}
		return &OpenAIBackend{name: "openai", APIKey: cfg.APIKey, Model: model, Endpoint: endpoint}, nil
	case "local", "http":
		// Both are OpenAI-compatible HTTP endpoints the user supplies; "local"
		// is the conventional name for a localhost model server. The endpoint is
		// required; the API key is optional (many local servers need none).
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("%s backend requires an endpoint (set backends.%s.endpoint in config, e.g. http://localhost:11434/v1)", cfg.Type, cfg.Type)
		}
		return &OpenAIBackend{name: cfg.Type, APIKey: cfg.APIKey, Model: cfg.Model, Endpoint: cfg.Endpoint}, nil
	default:
		return nil, fmt.Errorf("unknown backend type: %q", cfg.Type)
	}
}
