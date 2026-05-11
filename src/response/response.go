// Package response parses and validates structured LLM responses.
package response

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Kind is the kind of response the model returned.
type Kind string

// Kind constants enumerate the possible response types from the model.
const (
	Command     Kind = "command"
	Explanation Kind = "explanation"
	Script      Kind = "script"
)

// Result is the parsed JSON response from the model.
type Result struct {
	Type        Kind   `json:"type"`
	Command     string `json:"command,omitempty"`
	Explanation string `json:"explanation,omitempty"`
	Script      string `json:"script,omitempty"`
}

// Parse attempts to extract a valid Result from raw model output.
// The response must be a raw JSON object — no prose wrapping, no code fences.
func Parse(raw string) (*Result, error) {
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

func tryUnmarshal(s string) (*Result, error) {
	var resp Result
	if err := json.Unmarshal([]byte(s), &resp); err != nil {
		return nil, err
	}
	return validate(&resp)
}

func validate(resp *Result) (*Result, error) {
	switch resp.Type {
	case Command:
		if resp.Command == "" {
			return nil, fmt.Errorf("type is %q but command field is empty", resp.Type)
		}
	case Explanation:
		if resp.Explanation == "" {
			return nil, fmt.Errorf("type is %q but explanation field is empty", resp.Type)
		}
	case Script:
		if resp.Script == "" {
			return nil, fmt.Errorf("type is %q but script field is empty", resp.Type)
		}
		if !strings.HasPrefix(strings.TrimLeft(resp.Script, " \t\r\n"), "#!") {
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
