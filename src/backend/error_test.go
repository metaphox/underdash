package backend

import "testing"

func TestParseAPIError_AnthropicEnvelope(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"not_found_error","message":"model: claude-sonnet-4-20250514"},"request_id":"req_011CcLF9ApjLpVNNkXfmLwRc"}`)

	e := parseAPIError("claude", 404, body)

	if e.Backend != "claude" {
		t.Errorf("Backend = %q, want claude", e.Backend)
	}
	if e.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", e.StatusCode)
	}
	if e.Type != "not_found_error" {
		t.Errorf("Type = %q, want not_found_error", e.Type)
	}
	if e.Message != "model: claude-sonnet-4-20250514" {
		t.Errorf("Message = %q", e.Message)
	}
	if e.RequestID != "req_011CcLF9ApjLpVNNkXfmLwRc" {
		t.Errorf("RequestID = %q", e.RequestID)
	}
}

func TestParseAPIError_UnparseableBody(t *testing.T) {
	body := []byte("  <html>502 Bad Gateway</html>  ")

	e := parseAPIError("claude", 502, body)

	if e.Type != "" {
		t.Errorf("Type = %q, want empty for non-JSON body", e.Type)
	}
	if e.Raw != "<html>502 Bad Gateway</html>" {
		t.Errorf("Raw = %q, want trimmed body", e.Raw)
	}
	if got := e.Summary(); got != "API error" {
		t.Errorf("Summary() = %q, want fallback %q", got, "API error")
	}
}

func TestAPIError_Summary(t *testing.T) {
	tests := []struct {
		typ  string
		want string
	}{
		{"not_found_error", "not found"},
		{"rate_limit_error", "rate limit"},
		{"authentication_error", "authentication"},
		{"", "API error"},
	}
	for _, tc := range tests {
		e := &APIError{Type: tc.typ}
		if got := e.Summary(); got != tc.want {
			t.Errorf("Summary(%q) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestAPIError_Hint(t *testing.T) {
	for _, status := range []int{401, 404, 429, 500} {
		e := &APIError{Backend: "claude", StatusCode: status}
		if e.Hint() == "" {
			t.Errorf("Hint() empty for status %d, want guidance", status)
		}
	}
	// An unmapped status yields no hint.
	if got := (&APIError{StatusCode: 418}).Hint(); got != "" {
		t.Errorf("Hint() for 418 = %q, want empty", got)
	}
}
