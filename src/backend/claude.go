package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultClaudeEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion      = "2023-06-01"
)

// ClaudeBackend calls the Anthropic Messages API via raw HTTP using SSE streaming.
type ClaudeBackend struct {
	APIKey   string
	Model    string
	Endpoint string
}

// Name returns the backend identifier used for configuration and selection.
func (c *ClaudeBackend) Name() string { return "claude" }

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
	Stream    bool            `json:"stream"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeStreamEvent is the union of SSE event payloads we care about.
// Unused fields from the API (usage, index, etc.) are ignored.
type claudeStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Send sends a request to Claude and returns the raw model response text.
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
		Stream: true,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup //nolint:errcheck // best-effort cleanup

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		switch resp.StatusCode {
		case 401:
			return "", fmt.Errorf("authentication failed: invalid API key")
		case 429:
			return "", fmt.Errorf("rate limited: too many requests")
		default:
			return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
		}
	}

	return readClaudeStream(resp.Body)
}

// readClaudeStream consumes an Anthropic Messages SSE stream and returns the
// concatenated text from all `content_block_delta` text deltas.
func readClaudeStream(r io.Reader) (string, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(r)
	// SSE lines can be long; bump the buffer.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" || data == "[DONE]" {
			continue
		}

		var ev claudeStreamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			// Malformed event line — skip.
			continue
		}
		if ev.Error != nil {
			return "", fmt.Errorf("API error: %s: %s", ev.Error.Type, ev.Error.Message)
		}
		if ev.Type == "content_block_delta" && ev.Delta.Type == "text_delta" {
			sb.WriteString(ev.Delta.Text)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read stream: %w", err)
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("empty response from API")
	}
	return sb.String(), nil
}
