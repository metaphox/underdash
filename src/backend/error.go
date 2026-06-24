package backend

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIError is a structured error from a backend's HTTP/streaming API. Backends
// populate it so the display layer can pretty-print rather than dump raw JSON.
type APIError struct {
	Backend    string // e.g. "claude"
	StatusCode int    // HTTP status, or 0 for stream-level errors
	Type       string // provider error type, e.g. "not_found_error"
	Message    string // provider-supplied message
	RequestID  string // provider request id, when present
	Raw        string // trimmed raw body, fallback when the envelope won't parse
}

// Error returns a concise single-line form, used for logs and as the fallback
// when the structured renderer is not in play.
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s (%s, HTTP %d)", e.Backend, e.Message, e.Summary(), e.StatusCode)
	}
	return fmt.Sprintf("%s API error (HTTP %d): %s", e.Backend, e.StatusCode, e.Raw)
}

// Summary returns a short, humanized label for the error, e.g. "not found".
func (e *APIError) Summary() string {
	if e.Type != "" {
		s := strings.TrimSuffix(e.Type, "_error")
		return strings.ReplaceAll(s, "_", " ")
	}
	return "API error"
}

// Hint returns optional remediation advice keyed off the HTTP status, or "".
func (e *APIError) Hint() string {
	switch e.StatusCode {
	case 401:
		return fmt.Sprintf("check your API key (backends.%s.api_key or the *_API_KEY env var)", e.Backend)
	case 403:
		return "your API key lacks permission for this resource"
	case 404:
		return fmt.Sprintf("the endpoint or model may be wrong or retired — verify backends.%s.model and backends.%s.endpoint", e.Backend, e.Backend)
	case 429:
		return "rate limited — wait and retry, or check your plan limits"
	case 500, 502, 503, 529:
		return "the provider had a server error — retry shortly"
	}
	return ""
}

// IsModelNotFound reports whether the error is a 404 caused by an unknown or
// retired model (as opposed to, say, a wrong endpoint). It keys on the model
// being named in the type or message, which both Anthropic and OpenAI do.
func (e *APIError) IsModelNotFound() bool {
	if e.StatusCode != 404 {
		return false
	}
	return strings.Contains(e.Type, "model") || strings.Contains(strings.ToLower(e.Message), "model")
}

// parseAPIError builds an APIError from a non-2xx response body, unmarshalling
// the common {"error":{"type","message"},"request_id"} envelope used by
// Anthropic and OpenAI. Unparseable bodies are preserved in Raw.
func parseAPIError(backendName string, status int, body []byte) *APIError {
	e := &APIError{
		Backend:    backendName,
		StatusCode: status,
		Raw:        strings.TrimSpace(string(body)),
	}

	var env struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if json.Unmarshal(body, &env) == nil {
		e.Type = env.Error.Type
		e.Message = env.Error.Message
		e.RequestID = env.RequestID
	}
	return e
}
