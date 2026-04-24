package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultClaudeEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion      = "2023-06-01"
	defaultTimeout        = 30 * time.Second
)

// ClaudeBackend calls the Anthropic Messages API via raw HTTP.
type ClaudeBackend struct {
	APIKey   string
	Model    string
	Endpoint string
}

func (c *ClaudeBackend) Name() string { return "claude" }

type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *ClaudeBackend) Send(ctx context.Context, req Request) (string, error) {
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = defaultClaudeEndpoint
	}

	body := claudeRequest{
		Model:     c.Model,
		MaxTokens: req.MaxTokens,
		System:    req.SystemPrompt,
		Messages: []claudeMessage{
			{Role: "user", Content: req.UserMessage},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 401:
			return "", fmt.Errorf("authentication failed: invalid API key")
		case 429:
			return "", fmt.Errorf("rate limited: too many requests")
		default:
			return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
		}
	}

	var cResp claudeResponse
	if err := json.Unmarshal(respBody, &cResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if cResp.Error != nil {
		return "", fmt.Errorf("API error: %s: %s", cResp.Error.Type, cResp.Error.Message)
	}

	if len(cResp.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return cResp.Content[0].Text, nil
}
