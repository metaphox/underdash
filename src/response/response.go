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
// The response must be a raw JSON object — no prose wrapping, no code fences.
func Parse(raw string) (*LLMResponse, error) {
	raw = strings.TrimSpace(raw)

	if !strings.HasPrefix(raw, "{") {
		return nil, fmt.Errorf("invalid JSON response: does not start with '{'")
	}

	// Try direct unmarshal.
	resp, err := tryUnmarshal(raw)
	if err == nil {
		return resp, nil
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
		if !strings.HasPrefix(strings.TrimLeft(resp.ScriptStr, " \t\r\n"), "#!") {
			return nil, fmt.Errorf("type is %q but script does not start with a shebang", resp.Type)
		}
	case "":
		return nil, fmt.Errorf("missing required field: type")
	default:
		return nil, fmt.Errorf("unknown response type: %q", resp.Type)
	}
	return resp, nil
}

// IsRetryable returns true if the raw response starts with '{', indicating
// it was an attempt at JSON (just malformed). Returns false for clearly non-JSON
// responses (prose, code fences, etc.) which should bail immediately.
func IsRetryable(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "{")
}
