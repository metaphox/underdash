package response

import (
	"testing"
)

// --- Finding #8: strict JSON parsing ---

func TestParse_ValidJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Kind
	}{
		{
			"command",
			`{"type":"command","command":"ls -la","explanation":"list files"}`,
			Command,
		},
		{
			"explanation",
			`{"type":"explanation","explanation":"awk -F sets the field separator"}`,
			Explanation,
		},
		{
			"script",
			`{"type":"script","script":"#!/bin/bash\necho hello"}`,
			Script,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := Parse(tt.raw)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.raw, err)
			}
			if resp.Type != tt.want {
				t.Errorf("Parse(%q).Type = %q, want %q", tt.raw, resp.Type, tt.want)
			}
		})
	}
}

func TestParse_ProseWrappedJSON_MustFail(t *testing.T) {
	// Per spec: the model must return raw JSON. If it wraps JSON in prose,
	// that's a parse failure (should trigger retry), NOT a silent extraction.
	raw := `Here is the command you need:
{"type":"command","command":"ls -la"}
Hope that helps!`

	_, err := Parse(raw)
	if err == nil {
		t.Error("Parse() should fail on prose-wrapped JSON (spec requires raw JSON only)")
	}
}

func TestParse_CodeFenceWrapped_MustFail(t *testing.T) {
	raw := "```json\n{\"type\":\"command\",\"command\":\"ls -la\"}\n```"

	_, err := Parse(raw)
	if err == nil {
		t.Error("Parse() should fail on code-fence-wrapped JSON")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	raw := `{"type":"command","command":}`
	_, err := Parse(raw)
	if err == nil {
		t.Error("Parse() should fail on invalid JSON")
	}
}

func TestParse_MissingType(t *testing.T) {
	raw := `{"command":"ls -la"}`
	_, err := Parse(raw)
	if err == nil {
		t.Error("Parse() should fail when type field is missing")
	}
}

func TestParse_MissingRequiredField(t *testing.T) {
	raw := `{"type":"command"}`
	_, err := Parse(raw)
	if err == nil {
		t.Error("Parse() should fail when command field is empty for type=command")
	}
}

// --- Script shebang validation ---

func TestParse_ScriptMissingShebang(t *testing.T) {
	raw := `{"type":"script","script":"echo hello\nls"}`
	_, err := Parse(raw)
	if err == nil {
		t.Error("Parse() should fail when script does not start with a shebang")
	}
}

func TestParse_ScriptShebangAfterWhitespace(t *testing.T) {
	// Leading whitespace before #! is tolerated (some models add a stray newline).
	raw := `{"type":"script","script":"\n#!/bin/sh\necho hello"}`
	if _, err := Parse(raw); err != nil {
		t.Errorf("Parse() should accept shebang after leading whitespace, got: %v", err)
	}
}

// --- Finding #8: IsRetryable strictness ---

func TestIsRetryable_StartsWithBrace(t *testing.T) {
	// Only retryable if it starts with {
	if !IsRetryable(`{"type":"comma`) {
		t.Error("should be retryable when starts with {")
	}
}

func TestIsRetryable_StartsWithProse(t *testing.T) {
	// Even if it contains { somewhere, if it doesn't start with { it's not retryable.
	if IsRetryable("I cannot help with that. Here is {something}") {
		t.Error("should NOT be retryable when response starts with prose")
	}
}

func TestIsRetryable_CompletelyNonJSON(t *testing.T) {
	if IsRetryable("I don't understand your request.") {
		t.Error("should NOT be retryable for clearly non-JSON response")
	}
}
