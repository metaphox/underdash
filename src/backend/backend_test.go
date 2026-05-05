package backend

import (
	"strings"
	"testing"
)

// --- Finding #3: all spec-defined backend types should be accepted ---

func TestNew_AllSpecTypes(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			"stdout",
			Config{Type: "stdout"},
			false, "",
		},
		{
			"claude with key",
			Config{Type: "claude", APIKey: "sk-test"},
			false, "",
		},
		{
			"claude without key",
			Config{Type: "claude"},
			true, "requires an API key",
		},
		{
			"openai should not be unknown",
			Config{Type: "openai"},
			true, "not yet implemented", // should be a friendly "not implemented" not "unknown"
		},
		{
			"local should not be unknown",
			Config{Type: "local"},
			true, "not yet implemented",
		},
		{
			"http should not be unknown",
			Config{Type: "http"},
			true, "not yet implemented",
		},
		{
			"truly unknown type",
			Config{Type: "foobar"},
			true, "unknown backend type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("New(%+v) should return error", tt.cfg)
				}
				if tt.errMsg != "" && !containsStr(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("New(%+v) unexpected error: %v", tt.cfg, err)
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Claude SSE streaming ---

func TestReadClaudeStream_ConcatenatesTextDeltas(t *testing.T) {
	stream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start"}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"{\"type\":"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"\"command\",\"command\":\"ls\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	got, err := readClaudeStream(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("readClaudeStream: %v", err)
	}
	want := `{"type":"command","command":"ls"}`
	if got != want {
		t.Errorf("readClaudeStream\n got: %q\nwant: %q", got, want)
	}
}

func TestReadClaudeStream_PropagatesErrorEvent(t *testing.T) {
	stream := "data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"servers busy\"}}\n\n"
	_, err := readClaudeStream(strings.NewReader(stream))
	if err == nil {
		t.Fatal("expected error from error event")
	}
	if !strings.Contains(err.Error(), "overloaded_error") || !strings.Contains(err.Error(), "servers busy") {
		t.Errorf("error should surface event details, got: %v", err)
	}
}

func TestReadClaudeStream_EmptyIsError(t *testing.T) {
	stream := "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	_, err := readClaudeStream(strings.NewReader(stream))
	if err == nil {
		t.Fatal("expected error when no text deltas were received")
	}
}
