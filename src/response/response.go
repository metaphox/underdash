package response

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResponseType is the kind of response the model returned.
type ResponseType string

const (
	Command     ResponseType = "command"
	Explanation ResponseType = "explanation"
	Script      ResponseType = "script"
)

// LLMResponse is the parsed JSON response from the model.
type LLMResponse struct {
	Type        ResponseType `json:"type"`
	CommandStr  string       `json:"command,omitempty"`
	Explanation string       `json:"explanation,omitempty"`
	ScriptStr   string       `json:"script,omitempty"`
}

// Parse attempts to extract a valid LLMResponse from raw model output.
func Parse(raw string) (*LLMResponse, error) {
	raw = strings.TrimSpace(raw)

	// Try direct unmarshal first.
	resp, err := tryUnmarshal(raw)
	if err == nil {
		return resp, nil
	}

	// Fallback: find the first { and last } and try that substring.
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		resp, err = tryUnmarshal(raw[start : end+1])
		if err == nil {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("invalid JSON response: %w", err)
}

func tryUnmarshal(s string) (*LLMResponse, error) {
	var resp LLMResponse
	if err := json.Unmarshal([]byte(s), &resp); err != nil {
		return nil, err
	}
	return validate(&resp)
}

func validate(resp *LLMResponse) (*LLMResponse, error) {
	switch resp.Type {
	case Command:
		if resp.CommandStr == "" {
			return nil, fmt.Errorf("type is %q but command field is empty", resp.Type)
		}
	case Explanation:
		if resp.Explanation == "" {
			return nil, fmt.Errorf("type is %q but explanation field is empty", resp.Type)
		}
	case Script:
		if resp.ScriptStr == "" {
			return nil, fmt.Errorf("type is %q but script field is empty", resp.Type)
		}
	case "":
		return nil, fmt.Errorf("missing required field: type")
	default:
		return nil, fmt.Errorf("unknown response type: %q", resp.Type)
	}
	return resp, nil
}

// IsRetryable returns true if the raw response looks like a failed attempt at JSON
// (contains '{') rather than something completely unrelated.
func IsRetryable(raw string) bool {
	return strings.Contains(raw, "{")
}
