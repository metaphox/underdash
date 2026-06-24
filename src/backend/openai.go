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

const defaultOpenAIEndpoint = "https://api.openai.com/v1"

// OpenAIBackend calls an OpenAI-compatible Chat Completions API over SSE
// streaming. It backs the "openai", "local", and "http" backend types — they
// differ only in defaults and whether an API key is required (see New); the
// wire protocol is identical.
type OpenAIBackend struct {
	name     string // "openai", "local", or "http"
	APIKey   string
	Model    string
	Endpoint string // base URL (e.g. .../v1) or a full chat/completions URL
}

// Name returns the backend identifier used for configuration and selection.
func (o *OpenAIBackend) Name() string { return o.name }

type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens,omitempty"`
	Stream    bool            `json:"stream"`
}

type openAIMessage struct {
	Role string `json:"role"`
	// Content is either a plain string or a []openAIContentPart when image
	// attachments are present.
	Content any `json:"content"`
}

// openAIContentPart is one element of a multimodal message content array.
type openAIContentPart struct {
	Type     string          `json:"type"`                // "text" or "image_url"
	Text     string          `json:"text,omitempty"`      // set for "text" parts
	ImageURL *openAIImageURL `json:"image_url,omitempty"` // set for "image_url" parts
}

type openAIImageURL struct {
	URL string `json:"url"` // a data: URI carrying base64 image bytes
}

// buildUserContent returns the user message content: a plain string when there
// are no attachments, or a content-part array carrying image data URIs. Text
// attachments are inlined ahead of the instruction; document/PDF attachments are
// rejected, since the Chat Completions protocol cannot carry them.
func (o *OpenAIBackend) buildUserContent(req Request) (any, error) {
	if len(req.Attachments) == 0 {
		return req.UserMessage, nil
	}
	parts := make([]openAIContentPart, 0, len(req.Attachments)+1)
	var inlined strings.Builder
	for _, a := range req.Attachments {
		switch a.Kind {
		case "image":
			parts = append(parts, openAIContentPart{
				Type:     "image_url",
				ImageURL: &openAIImageURL{URL: "data:" + a.MediaType + ";base64," + a.Data},
			})
		case "text":
			fmt.Fprintf(&inlined, "Contents of %s:\n%s\n\n", a.Filename, a.Data)
		default: // "document" / PDF
			return nil, fmt.Errorf("the %s backend does not support %s attachments (%s); only images and text files are supported", o.name, a.Kind, a.Filename)
		}
	}
	parts = append(parts, openAIContentPart{Type: "text", Text: inlined.String() + req.UserMessage})
	return parts, nil
}

// openAIStreamEvent is the subset of a Chat Completions stream chunk we read.
type openAIStreamEvent struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Send sends a request to an OpenAI-compatible endpoint and returns the raw
// model response text.
func (o *OpenAIBackend) Send(ctx context.Context, req Request) (string, error) {
	userContent, err := o.buildUserContent(req)
	if err != nil {
		return "", err
	}

	body := openAIRequest{
		Model:     o.Model,
		MaxTokens: req.MaxTokens,
		Stream:    true,
		Messages: []openAIMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: userContent},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := resolveChatEndpoint(o.Endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	// Keyless local/http endpoints (e.g. Ollama) get no Authorization header.
	if o.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", parseAPIError(o.name, resp.StatusCode, respBody)
	}

	return readOpenAIStream(resp.Body, o.name)
}

// resolveChatEndpoint turns a base URL (e.g. ".../v1") into the full
// chat/completions URL, leaving an already-complete URL untouched.
func resolveChatEndpoint(endpoint string) string {
	e := strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(e, "/chat/completions") {
		return e
	}
	return e + "/chat/completions"
}

// readOpenAIStream consumes a Chat Completions SSE stream and returns the
// concatenated text from all `choices[].delta.content` fragments.
func readOpenAIStream(r io.Reader, backendName string) (string, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var ev openAIStreamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			// Malformed event line — skip.
			continue
		}
		if ev.Error != nil {
			return "", &APIError{
				Backend: backendName,
				Type:    ev.Error.Type,
				Message: ev.Error.Message,
			}
		}
		for _, c := range ev.Choices {
			sb.WriteString(c.Delta.Content)
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
