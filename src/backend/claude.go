package backend

import (
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
	Role string `json:"role"`
	// Content is either a plain string (text-only message) or a
	// []claudeContentBlock when attachments are present.
	Content any `json:"content"`
}

// claudeContentBlock is one element of a structured message content array.
type claudeContentBlock struct {
	Type   string             `json:"type"`             // "text", "image", or "document"
	Text   string             `json:"text,omitempty"`   // set for "text" blocks
	Source *claudeBlockSource `json:"source,omitempty"` // set for "image"/"document" blocks
}

// claudeBlockSource describes the data for an image or document block.
type claudeBlockSource struct {
	Type      string `json:"type"`       // "base64" or "text"
	MediaType string `json:"media_type"` // e.g. "image/png", "application/pdf"
	Data      string `json:"data"`       // base64 bytes, or raw text for a "text" source
}

// buildClaudeContent returns the message content: a plain string when there are
// no attachments (unchanged behavior), or a content-block array that places the
// encoded attachments before the user's instruction.
func buildClaudeContent(req Request) any {
	if len(req.Attachments) == 0 {
		return req.UserMessage
	}
	blocks := make([]claudeContentBlock, 0, len(req.Attachments)+1)
	for _, a := range req.Attachments {
		switch a.Kind {
		case "image":
			blocks = append(blocks, claudeContentBlock{
				Type:   "image",
				Source: &claudeBlockSource{Type: "base64", MediaType: a.MediaType, Data: a.Data},
			})
		case "document":
			blocks = append(blocks, claudeContentBlock{
				Type:   "document",
				Source: &claudeBlockSource{Type: "base64", MediaType: a.MediaType, Data: a.Data},
			})
		case "text":
			blocks = append(blocks, claudeContentBlock{
				Type:   "document",
				Source: &claudeBlockSource{Type: "text", MediaType: "text/plain", Data: a.Data},
			})
		}
	}
	blocks = append(blocks, claudeContentBlock{Type: "text", Text: req.UserMessage})
	return blocks
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
			{Role: "user", Content: buildClaudeContent(req)},
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

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	// Response headers have arrived; the first-byte timeout no longer applies.
	if req.OnResponseStart != nil {
		req.OnResponseStart()
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", parseAPIError(c.Name(), resp.StatusCode, respBody)
	}

	return readClaudeStream(resp.Body, c.Name())
}

// readClaudeStream consumes an Anthropic Messages SSE stream and returns the
// concatenated text from all `content_block_delta` text deltas.
func readClaudeStream(r io.Reader, backendName string) (string, error) {
	return readSSEStream(r, func(data string, sb *strings.Builder) error {
		var ev claudeStreamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			// Malformed event line — skip.
			return nil
		}
		if ev.Error != nil {
			return &APIError{
				Backend: backendName,
				Type:    ev.Error.Type,
				Message: ev.Error.Message,
			}
		}
		if ev.Type == "content_block_delta" && ev.Delta.Type == "text_delta" {
			sb.WriteString(ev.Delta.Text)
		}
		return nil
	})
}
